package pending

import (
	"zephyr-relay/internal/relay"
)

func (s *Store) Save(envelope relay.Envelope) error {
	return nil
}

func (s *Store) GetAll(address string) ([]Item, error) {
	return nil, nil
}

func (s *Store) DeleteAll(address string) error {
	return nil
}
