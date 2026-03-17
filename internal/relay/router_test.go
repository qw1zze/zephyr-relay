package relay_test

import (
	"context"
	"encoding/json"
	"log/slog"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"zephyr-relay/internal/relay"
)

type mockPendingStore struct {
	mu    sync.Mutex
	items map[string][]relay.PendingItem
}

func newMockPendingStore() *mockPendingStore {
	return &mockPendingStore{items: make(map[string][]relay.PendingItem)}
}

func (m *mockPendingStore) Save(env relay.Envelope) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.items[env.RecipientAddr] = append(m.items[env.RecipientAddr], relay.PendingItem{
		Envelope:  env,
		CreatedAt: time.Now(),
	})
	return nil
}

func (m *mockPendingStore) GetAll(address string) ([]relay.PendingItem, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.items[address], nil
}

func (m *mockPendingStore) DeleteAll(address string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	delete(m.items, address)
	return nil
}

func (m *mockPendingStore) saved(addr string) []relay.PendingItem {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.items[addr]
}

func newTestSession(addr string) *relay.Session {
	return relay.NewSession(addr, nil)
}

func readMsg(t *testing.T, sess *relay.Session) relay.Message {
	t.Helper()
	select {
	case data := <-sess.Send:
		var msg relay.Message
		require.NoError(t, json.Unmarshal(data, &msg))
		return msg
	case <-time.After(time.Second):
		t.Fatal("timeout waiting for message on Send channel")
		return relay.Message{}
	}
}

func readServerAck(t *testing.T, sess *relay.Session) relay.ServerAckPayload {
	t.Helper()
	msg := readMsg(t, sess)
	assert.Equal(t, relay.TypeServerAck, msg.Type)
	var ack relay.ServerAckPayload
	require.NoError(t, json.Unmarshal(msg.Payload, &ack))
	return ack
}

func marshalEnvelope(t *testing.T, env relay.Envelope) json.RawMessage {
	t.Helper()
	data, err := json.Marshal(env)
	require.NoError(t, err)
	return data
}

func marshalAck(t *testing.T, msgID string) json.RawMessage {
	t.Helper()
	data, err := json.Marshal(relay.AckPayload{MessageID: msgID})
	require.NoError(t, err)
	return data
}

const (
	aliceAddr = "0x1111111111111111111111111111111111111111"
	bobAddr   = "0x2222222222222222222222222222222222222222"
)

func validEnvelope(senderAddr, recipientAddr, msgID string) relay.Envelope {
	return relay.Envelope{
		MessageID:     msgID,
		ChatID:        "chat1",
		SenderAddr:    senderAddr,
		RecipientAddr: recipientAddr,
		CID:           "QmTestCID1234567890",
		Timestamp:     1234567890,
	}
}

func newRouter(sessions *relay.SessionStore, ps relay.PendingStore) *relay.Router {
	return relay.NewRouter(sessions, ps, slog.Default())
}

func TestHandleSend_RecipientOnline(t *testing.T) {
	store := relay.NewSessionStore(slog.Default())
	ps := newMockPendingStore()
	router := newRouter(store, ps)

	sender := newTestSession(aliceAddr)
	recipient := newTestSession(bobAddr)
	store.Add(aliceAddr, sender)
	store.Add(bobAddr, recipient)

	env := validEnvelope(aliceAddr, bobAddr, "msg1")
	msg := relay.Message{Type: relay.TypeSend, Payload: marshalEnvelope(t, env)}

	err := router.Route(context.Background(), sender, msg)
	require.NoError(t, err)

	delivered := readMsg(t, recipient)
	assert.Equal(t, relay.TypeDeliver, delivered.Type)
	var gotEnv relay.Envelope
	require.NoError(t, json.Unmarshal(delivered.Payload, &gotEnv))
	assert.Equal(t, env, gotEnv)

	ack := readServerAck(t, sender)
	assert.Equal(t, "msg1", ack.MessageID)
	assert.Equal(t, "delivered", ack.Status)

	assert.Empty(t, ps.saved(bobAddr))
}

func TestHandleSend_RecipientOffline(t *testing.T) {
	store := relay.NewSessionStore(slog.Default())
	ps := newMockPendingStore()
	router := newRouter(store, ps)

	sender := newTestSession(aliceAddr)
	store.Add(aliceAddr, sender)

	env := validEnvelope(aliceAddr, bobAddr, "msg2")
	msg := relay.Message{Type: relay.TypeSend, Payload: marshalEnvelope(t, env)}

	err := router.Route(context.Background(), sender, msg)
	require.NoError(t, err)

	saved := ps.saved(bobAddr)
	require.Len(t, saved, 1)
	assert.Equal(t, env, saved[0].Envelope)

	ack := readServerAck(t, sender)
	assert.Equal(t, "msg2", ack.MessageID)
	assert.Equal(t, "pending", ack.Status)
}

func TestHandleSend_InvalidSenderAddr(t *testing.T) {
	store := relay.NewSessionStore(slog.Default())
	router := newRouter(store, newMockPendingStore())

	sender := newTestSession(aliceAddr)

	env := validEnvelope(bobAddr, bobAddr, "msg3")
	msg := relay.Message{Type: relay.TypeSend, Payload: marshalEnvelope(t, env)}

	err := router.Route(context.Background(), sender, msg)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "sender_addr mismatch")
}

func TestHandleSend_EmptyMessageID(t *testing.T) {
	store := relay.NewSessionStore(slog.Default())
	router := newRouter(store, newMockPendingStore())

	sender := newTestSession(aliceAddr)

	env := validEnvelope(aliceAddr, bobAddr, "")
	msg := relay.Message{Type: relay.TypeSend, Payload: marshalEnvelope(t, env)}

	err := router.Route(context.Background(), sender, msg)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "message_id is empty")
}

func TestHandleSend_InvalidRecipient(t *testing.T) {
	store := relay.NewSessionStore(slog.Default())
	router := newRouter(store, newMockPendingStore())

	sender := newTestSession(aliceAddr)

	env := validEnvelope(aliceAddr, "not-an-eth-address", "msg4")
	msg := relay.Message{Type: relay.TypeSend, Payload: marshalEnvelope(t, env)}

	err := router.Route(context.Background(), sender, msg)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "recipient_addr is invalid")
}

func TestHandleAck_SenderOnline(t *testing.T) {
	store := relay.NewSessionStore(slog.Default())
	ps := newMockPendingStore()
	router := newRouter(store, ps)

	sender := newTestSession(aliceAddr)
	recipient := newTestSession(bobAddr)
	store.Add(aliceAddr, sender)
	store.Add(bobAddr, recipient)

	env := validEnvelope(aliceAddr, bobAddr, "msgA")
	sendMsg := relay.Message{Type: relay.TypeSend, Payload: marshalEnvelope(t, env)}
	require.NoError(t, router.Route(context.Background(), sender, sendMsg))

	readMsg(t, recipient)
	readMsg(t, sender)  

	ackMsg := relay.Message{Type: relay.TypeAck, Payload: marshalAck(t, "msgA")}
	err := router.Route(context.Background(), recipient, ackMsg)
	require.NoError(t, err)

	ack := readServerAck(t, sender)
	assert.Equal(t, "msgA", ack.MessageID)
	assert.Equal(t, "delivered", ack.Status)
}

func TestHandleAck_SenderOffline(t *testing.T) {
	store := relay.NewSessionStore(slog.Default())
	ps := newMockPendingStore()
	router := newRouter(store, ps)

	sender := newTestSession(aliceAddr)
	recipient := newTestSession(bobAddr)
	store.Add(aliceAddr, sender)
	store.Add(bobAddr, recipient)

	env := validEnvelope(aliceAddr, bobAddr, "msgB")
	sendMsg := relay.Message{Type: relay.TypeSend, Payload: marshalEnvelope(t, env)}
	require.NoError(t, router.Route(context.Background(), sender, sendMsg))
	readMsg(t, recipient)
	readMsg(t, sender)

	store.Delete(aliceAddr)

	ackMsg := relay.Message{Type: relay.TypeAck, Payload: marshalAck(t, "msgB")}
	err := router.Route(context.Background(), recipient, ackMsg)
	require.NoError(t, err)
}

func TestHandleAck_UnknownMessageID(t *testing.T) {
	store := relay.NewSessionStore(slog.Default())
	router := newRouter(store, newMockPendingStore())

	recipient := newTestSession(bobAddr)

	ackMsg := relay.Message{Type: relay.TypeAck, Payload: marshalAck(t, "unknown-id")}
	err := router.Route(context.Background(), recipient, ackMsg)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "unknown message_id")
}
