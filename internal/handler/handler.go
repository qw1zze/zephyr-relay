package handler

import (
	"log/slog"
	"zephyr-relay/internal/auth"
	"zephyr-relay/internal/config"
	"zephyr-relay/internal/pending"
	"zephyr-relay/internal/relay"
)

type Handler struct {
	cfg     *config.Config
	auth    *auth.Auth
	relay   *relay.Relay
	pending *pending.Store
	log     *slog.Logger
}

func New(
	cfg *config.Config,
	auth *auth.Auth,
	relay *relay.Relay,
	pending *pending.Store,
	log *slog.Logger,
) *Handler {
	return &Handler{
		cfg:     cfg,
		auth:    auth,
		relay:   relay,
		pending: pending,
		log:     log,
	}
}
