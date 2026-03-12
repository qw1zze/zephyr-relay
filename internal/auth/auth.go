package auth

import (
	"context"
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
	ctx        context.Context
	once       sync.Once
}

func New(ctx context.Context, cfg *config.Config, log *slog.Logger) *Auth {
	return &Auth{
		challenges: make(map[string]Challenge),
		ttl:        time.Duration(cfg.ChallengesTTLSec) * time.Second,
		logger:     log,
		ctx:        ctx,
	}
}

func NewWithTTL(ctx context.Context, ttl time.Duration, log *slog.Logger) *Auth {
	return &Auth{
		challenges: make(map[string]Challenge),
		ttl:        ttl,
		logger:     log,
		ctx:        ctx,
	}
}
