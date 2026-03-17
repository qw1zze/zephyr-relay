package pending_test

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"zephyr-relay/internal/pending"
	"zephyr-relay/internal/relay"
)

func newEnvelope(msgID, recipientAddr string) relay.Envelope {
	return relay.Envelope{
		MessageID:     msgID,
		ChatID:        "chat1",
		SenderAddr:    "0x1111111111111111111111111111111111111111",
		RecipientAddr: recipientAddr,
		CID:           "QmTest",
		Timestamp:     1234567890,
	}
}

func TestPendingStore_SaveAndGetAll(t *testing.T) {
	s := pending.NewStore(context.Background(), time.Hour)

	env := newEnvelope("msg1", "0xABC")
	require.NoError(t, s.Save(env))

	items, err := s.GetAll("0xABC")
	require.NoError(t, err)
	require.Len(t, items, 1)
	assert.Equal(t, env, items[0].Envelope)
}

func TestPendingStore_TTLExpired(t *testing.T) {
	s := pending.NewStore(context.Background(), time.Millisecond)

	env := newEnvelope("msg1", "0xABC")
	require.NoError(t, s.Save(env))

	time.Sleep(2 * time.Millisecond)

	items, err := s.GetAll("0xABC")
	require.NoError(t, err)
	assert.Empty(t, items)
}

func TestPendingStore_DeleteAll(t *testing.T) {
	s := pending.NewStore(context.Background(), time.Hour)

	for i := 0; i < 3; i++ {
		env := newEnvelope(fmt.Sprintf("msg%d", i), "0xABC")
		require.NoError(t, s.Save(env))
	}

	require.NoError(t, s.DeleteAll("0xABC"))

	items, err := s.GetAll("0xABC")
	require.NoError(t, err)
	assert.Empty(t, items)
}
