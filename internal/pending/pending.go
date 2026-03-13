package pending

import (
	"context"
	"time"

	"zephyr-relay/internal/relay"
)

func (s *Store) Save(envelope relay.Envelope) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.items[envelope.RecipientAddr] = append(s.items[envelope.RecipientAddr], Item{
		Envelope:  envelope,
		CreatedAt: time.Now(),
	})
	return nil
}

func (s *Store) GetAll(address string) ([]Item, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	now := time.Now()
	var valid []Item
	for _, item := range s.items[address] {
		if now.Sub(item.CreatedAt) < s.ttl {
			valid = append(valid, item)
		}
	}

	if len(valid) == 0 {
		delete(s.items, address)
	} else {
		s.items[address] = valid
	}
	return valid, nil
}

func (s *Store) DeleteAll(address string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.items, address)
	return nil
}

func (s *Store) startCleanup(ctx context.Context, ttl time.Duration) {
	go func() {
		ticker := time.NewTicker(time.Hour)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				s.cleanup(ttl)
			}
		}
	}()
}

func (s *Store) cleanup(ttl time.Duration) {
	s.mu.Lock()
	defer s.mu.Unlock()

	now := time.Now()
	for addr, items := range s.items {
		var valid []Item
		for _, item := range items {
			if now.Sub(item.CreatedAt) < ttl {
				valid = append(valid, item)
			}
		}
		if len(valid) == 0 {
			delete(s.items, addr)
		} else {
			s.items[addr] = valid
		}
	}
}
