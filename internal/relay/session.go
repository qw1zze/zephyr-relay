package relay

import (
	"sync"
	"github.com/gofiber/contrib/websocket"
)

type Session struct {
	Address string
	Conn *websocket.Conn
	Send chan []byte
}

type SessionStore struct {
	sessions sync.Map
}

func NewSessionStore() *SessionStore {
	return &SessionStore{}
}

func (s *SessionStore) Add(address string, session *Session) {
	s.sessions.Store(address, session)
}

func (s *SessionStore) Get(address string) (*Session, bool) {
	v, ok := s.sessions.Load(address)
	if !ok {
		return nil, false
	}
	return v.(*Session), true
}

func (s *SessionStore) Delete(address string) {
	s.sessions.Delete(address)
}
