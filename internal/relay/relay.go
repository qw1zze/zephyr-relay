package relay

import (
	"encoding/json"
	"log/slog"
	"time"
	"zephyr-relay/internal/config"
)

type MessageType string

const (
	TypeChallenge MessageType = "challenge"
	TypeAuth MessageType = "auth"
	TypeAuthOK MessageType = "auth_ok"
	TypeSend MessageType = "send"
	TypeDeliver MessageType = "deliver"
	TypeAck MessageType = "ack"
	TypeServerAck MessageType = "server_ack"
	TypePing MessageType = "ping"
	TypePong MessageType = "pong"
)

type Message struct {
	Type    MessageType     `json:"type"`
	Payload json.RawMessage `json:"payload"`
}

type Envelope struct {
	MessageID     string `json:"message_id"`
	ChatID        string `json:"chat_id"`
	SenderAddr    string `json:"sender_addr"`
	RecipientAddr string `json:"recipient_addr"`
	CID           string `json:"cid"`
	Timestamp     int64  `json:"timestamp"`
	Signature     string `json:"signature"`
}

type ChallengePayload struct {
	Nonce string `json:"nonce"`
}

type AuthPayload struct {
	Address   string `json:"address"`
	Signature string `json:"signature"`
}

type AckPayload struct {
	MessageID string `json:"message_id"`
}

type ServerAckPayload struct {
	MessageID string `json:"message_id"`
	Status string `json:"status"`
}

type PendingItem struct {
	Envelope  Envelope
	CreatedAt time.Time
}

type PendingStore interface {
	Save(envelope Envelope) error
	GetAll(address string) ([]PendingItem, error)
	DeleteAll(address string) error
}

type Relay struct {
	cfg      *config.Config
	sessions *SessionStore
	pending  PendingStore
	log      *slog.Logger
}

func New(cfg *config.Config, sessions *SessionStore, pending PendingStore, log *slog.Logger) *Relay {
	return &Relay{
		cfg:      cfg,
		sessions: sessions,
		pending:  pending,
		log:      log,
	}
}
