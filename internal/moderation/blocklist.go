package moderation

import "sync"

type BlockList struct {
	mu      sync.RWMutex
	blocked map[string]struct{}
}

func NewBlockList() *BlockList {
	return &BlockList{blocked: make(map[string]struct{})}
}

func (b *BlockList) Block(address string) bool {
	b.mu.Lock()
	defer b.mu.Unlock()
	if _, exists := b.blocked[address]; exists {
		return false
	}
	b.blocked[address] = struct{}{}
	return true
}

func (b *BlockList) Unblock(address string) bool {
	b.mu.Lock()
	defer b.mu.Unlock()
	if _, exists := b.blocked[address]; !exists {
		return false
	}
	delete(b.blocked, address)
	return true
}

func (b *BlockList) IsBlocked(address string) bool {
	b.mu.RLock()
	defer b.mu.RUnlock()
	_, ok := b.blocked[address]
	return ok
}

func (b *BlockList) List() []string {
	b.mu.RLock()
	defer b.mu.RUnlock()
	out := make([]string, 0, len(b.blocked))
	for addr := range b.blocked {
		out = append(out, addr)
	}
	return out
}
