package handler

import (
	"log/slog"
	"zephyr-relay/internal/auth"
	"zephyr-relay/internal/config"
	"zephyr-relay/internal/pending"
	"zephyr-relay/internal/relay"
)

type Handler struct {
	cfg      *config.Config
	auth     *auth.Auth
	relay    *relay.Relay
	sessions *relay.SessionStore
	pending  *pending.Store
	router   *relay.Router
	log      *slog.Logger
}

func New(
	cfg *config.Config,
	auth *auth.Auth,
	relay *relay.Relay,
	sessions *relay.SessionStore,
	pending *pending.Store,
	router *relay.Router,
	log *slog.Logger,
) *Handler {
	return &Handler{
		cfg:      cfg,
		auth:     auth,
		relay:    relay,
		sessions: sessions,
		pending:  pending,
		router:   router,
		log:      log,
	}
}
