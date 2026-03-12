package auth

import (
	"log/slog"
	"sync"
	"time"
	"zephyr-relay/internal/config"
)

type Auth struct {
	mu         sync.Mutex
	challenges map[string]Challenge
	ttl        time.Duration
	logger     *slog.Logger
}

func New(cfg *config.Config, log *slog.Logger) *Auth {
	return &Auth{
		challenges: make(map[string]Challenge),
		ttl:        time.Duration(cfg.ChallengesTTLSec) * time.Second,
		logger:     log,
	}
}
