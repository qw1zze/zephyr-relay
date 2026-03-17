package pending

import (
	"context"
	"sync"
	"time"

	"zephyr-relay/internal/relay"
)

type Item = relay.PendingItem

type Store struct {
	mu    sync.Mutex
	items map[string][]Item
	ttl   time.Duration
}

func NewStore(ctx context.Context, ttl time.Duration) *Store {
	s := &Store{
		items: make(map[string][]Item),
		ttl:   ttl,
	}
	s.startCleanup(ctx, ttl)
	return s
}
