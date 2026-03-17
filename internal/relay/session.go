package relay

import (
	"errors"
	"log/slog"
	"sync"
	"time"

	"github.com/gofiber/contrib/websocket"
)

var (
	ErrSessionFull = errors.New("session send buffer is full")

	ErrSessionClosed = errors.New("session is closed")
)

type Session struct {
	Address   string
	Conn      *websocket.Conn
	Send      chan []byte  
	Done      chan struct{}
	CreatedAt time.Time

	closeOnce sync.Once
}

func NewSession(address string, conn *websocket.Conn) *Session {
	return &Session{
		Address:   address,
		Conn:      conn,
		Send:      make(chan []byte, 256),
		Done:      make(chan struct{}),
		CreatedAt: time.Now(),
	}
}

func (s *Session) WriteLoop(logger *slog.Logger) {
	defer s.Close()
	for data := range s.Send {
		if err := s.Conn.WriteMessage(websocket.TextMessage, data); err != nil {
			logger.Error("session write error", "address", s.Address, "err", err)
			return
		}
	}
}

func (s *Session) Close() {
	s.closeOnce.Do(func() {
		close(s.Done)
		close(s.Send)
	})
}

func (s *Session) SendMessage(data []byte) (err error) {
	defer func() {
		if r := recover(); r != nil {
			err = ErrSessionClosed
		}
	}()

	select {
	case <-s.Done:
		return ErrSessionClosed
	default:
	}

	select {
	case s.Send <- data:
		return nil
	case <-s.Done:
		return ErrSessionClosed
	default:
		return ErrSessionFull
	}
}

type SessionStore struct {
	sessions sync.Map // map[string]*Session
	logger   *slog.Logger
}

func NewSessionStore(logger *slog.Logger) *SessionStore {
	return &SessionStore{logger: logger}
}

func (s *SessionStore) Add(address string, session *Session) {
	if old, loaded := s.sessions.Swap(address, session); loaded {
		s.logger.Info("replaced existing session", "address", address)
		old.(*Session).Close()
	}
}

func (s *SessionStore) Get(address string) (*Session, bool) {
	v, ok := s.sessions.Load(address)
	if !ok {
		return nil, false
	}
	return v.(*Session), true
}

func (s *SessionStore) Delete(address string) {
	if v, ok := s.sessions.LoadAndDelete(address); ok {
		v.(*Session).Close()
	}
}

func (s *SessionStore) Count() int {
	count := 0
	s.sessions.Range(func(_, _ any) bool {
		count++
		return true
	})
	return count
}
