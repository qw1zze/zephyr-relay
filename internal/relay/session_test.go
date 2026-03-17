package relay_test

import (
	"fmt"
	"log/slog"
	"net"
	"sync"
	"testing"
	"time"

	fasthttpws "github.com/fasthttp/websocket"
	fiberws "github.com/gofiber/contrib/websocket"
	"github.com/gofiber/fiber/v2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"zephyr-relay/internal/relay"
)

func TestNewSession_Success(t *testing.T) {
	sess := relay.NewSession("0xABC", nil)

	assert.Equal(t, "0xABC", sess.Address)
	assert.NotNil(t, sess.Send)
	assert.NotNil(t, sess.Done)
	assert.WithinDuration(t, time.Now(), sess.CreatedAt, time.Second)
}

func TestSendMessage_Success(t *testing.T) {
	sess := relay.NewSession("0xABC", nil)

	data := []byte(`{"type":"test"}`)
	err := sess.SendMessage(data)
	require.NoError(t, err)

	select {
	case got := <-sess.Send:
		assert.Equal(t, data, got)
	default:
		t.Fatal("expected message in Send channel")
	}
}

func TestSendMessage_SessionClosed(t *testing.T) {
	sess := relay.NewSession("0xABC", nil)
	sess.Close()

	err := sess.SendMessage([]byte("hello"))
	assert.ErrorIs(t, err, relay.ErrSessionClosed)
}

func TestSendMessage_BufferFull(t *testing.T) {
	sess := relay.NewSession("0xABC", nil)

	for i := 0; i < 256; i++ {
		require.NoError(t, sess.SendMessage([]byte("x")))
	}

	err := sess.SendMessage([]byte("overflow"))
	assert.ErrorIs(t, err, relay.ErrSessionFull)
}

func TestClose_Idempotent(t *testing.T) {
	sess := relay.NewSession("0xABC", nil)

	assert.NotPanics(t, func() {
		sess.Close()
		sess.Close()
	})
}

func TestWriteLoop_SendsMessages(t *testing.T) {
	var (
		sessMu sync.Mutex
		sess   *relay.Session
		ready  = make(chan struct{})
	)

	app := fiber.New(fiber.Config{DisableStartupMessage: true})
	app.Use("/ws", func(c *fiber.Ctx) error {
		if fiberws.IsWebSocketUpgrade(c) {
			return c.Next()
		}
		return fiber.ErrUpgradeRequired
	})
	app.Get("/ws", fiberws.New(func(c *fiberws.Conn) {
		s := relay.NewSession("0xTEST", c)
		sessMu.Lock()
		sess = s
		sessMu.Unlock()
		close(ready)

		go s.WriteLoop(slog.Default())
		<-s.Done
	}))

	ln, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	port := ln.Addr().(*net.TCPAddr).Port

	go func() { _ = app.Listener(ln) }()
	t.Cleanup(func() { _ = app.Shutdown() })

	wsURL := fmt.Sprintf("ws://127.0.0.1:%d/ws", port)
	clientConn, _, err := fasthttpws.DefaultDialer.Dial(wsURL, nil)
	require.NoError(t, err)
	defer clientConn.Close()

	<-ready

	msg := []byte(`{"type":"hello"}`)
	sessMu.Lock()
	s := sess
	sessMu.Unlock()

	require.NoError(t, s.SendMessage(msg))

	_ = clientConn.SetReadDeadline(time.Now().Add(2 * time.Second))
	_, received, err := clientConn.ReadMessage()
	require.NoError(t, err)
	assert.Equal(t, msg, received)

	s.Close()
}

func TestSessionStore_Add_Get(t *testing.T) {
	store := relay.NewSessionStore(slog.Default())
	sess := relay.NewSession("0xABC", nil)

	store.Add("0xABC", sess)

	got, ok := store.Get("0xABC")
	assert.True(t, ok)
	assert.Same(t, sess, got)
}

func TestSessionStore_Add_ReplacesExisting(t *testing.T) {
	store := relay.NewSessionStore(slog.Default())

	first := relay.NewSession("0xABC", nil)
	second := relay.NewSession("0xABC", nil)

	store.Add("0xABC", first)
	store.Add("0xABC", second)

	got, ok := store.Get("0xABC")
	assert.True(t, ok)
	assert.Same(t, second, got)

	select {
	case <-first.Done:

	case <-time.After(time.Second):
		t.Fatal("first session Done channel not closed after replacement")
	}
}

func TestSessionStore_Delete(t *testing.T) {
	store := relay.NewSessionStore(slog.Default())
	sess := relay.NewSession("0xABC", nil)

	store.Add("0xABC", sess)
	store.Delete("0xABC")

	_, ok := store.Get("0xABC")
	assert.False(t, ok)
}

func TestSessionStore_Count(t *testing.T) {
	store := relay.NewSessionStore(slog.Default())

	store.Add("0x1", relay.NewSession("0x1", nil))
	store.Add("0x2", relay.NewSession("0x2", nil))
	store.Add("0x3", relay.NewSession("0x3", nil))
	assert.Equal(t, 3, store.Count())

	store.Delete("0x2")
	assert.Equal(t, 2, store.Count())
}
