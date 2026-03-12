package pending

import (
	"sync"
	"zephyr-relay/internal/relay"
)

type Item = relay.PendingItem

type Store struct {
	mu    sync.Mutex
	items map[string][]Item
}

func NewStore() *Store {
	return &Store{
		items: make(map[string][]Item),
	}
}
