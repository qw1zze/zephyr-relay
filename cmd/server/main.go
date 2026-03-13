package main

import (
	"context"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/gofiber/contrib/websocket"
	"github.com/gofiber/fiber/v2"
	"zephyr-relay/internal/auth"
	"zephyr-relay/internal/config"
	"zephyr-relay/internal/handler"
	"zephyr-relay/internal/middleware"
	"zephyr-relay/internal/pending"
	"zephyr-relay/internal/relay"
	"zephyr-relay/pkg/logger"
)

func main() {
	cfg := config.Load()
	log := logger.New()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	authSvc := auth.New(ctx, cfg, log)
	sessionStore := relay.NewSessionStore(log)

	pendingTTL := time.Duration(cfg.PendingTTLDays) * 24 * time.Hour
	pendingStore := pending.NewStore(ctx, pendingTTL)

	relaySvc := relay.New(cfg, sessionStore, pendingStore, log)
	router := relay.NewRouter(sessionStore, pendingStore, log)

	h := handler.New(cfg, authSvc, relaySvc, sessionStore, pendingStore, router, log)

	app := fiber.New(fiber.Config{
		DisableStartupMessage: true,
	})
	app.Use(middleware.Logger(log))
	app.Use(middleware.Recover(log))
	app.Use("/ws", func(c *fiber.Ctx) error {
		if websocket.IsWebSocketUpgrade(c) {
			return c.Next()
		}
		return fiber.ErrUpgradeRequired
	})
	app.Get("/ws", websocket.New(h.HandleWS))
	app.Get("/healthz", h.Health)
	app.Get("/readyz", h.Ready)

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)

	go func() {
		addr := ":" + cfg.ServerPort
		log.Info("server starting", "addr", addr)
		if err := app.Listen(addr); err != nil {
			log.Error("server listen error", "err", err)
			os.Exit(1)
		}
	}()

	<-quit
	log.Info("shutdown signal received")

	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer shutdownCancel()
	if err := app.ShutdownWithContext(shutdownCtx); err != nil {
		log.Error("graceful shutdown error", "err", err)
	}
	log.Info("server stopped")
}
