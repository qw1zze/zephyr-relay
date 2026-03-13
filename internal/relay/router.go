package relay

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"strings"
	"sync"
)

type Router struct {
	sessions   *SessionStore
	pending    PendingStore
	msgSenders sync.Map 
	logger     *slog.Logger
}

func NewRouter(sessions *SessionStore, pending PendingStore, logger *slog.Logger) *Router {
	return &Router{
		sessions: sessions,
		pending:  pending,
		logger:   logger,
	}
}

func (r *Router) Route(ctx context.Context, sender *Session, msg Message) error {
	switch msg.Type {
	case TypeSend:
		return r.handleSend(ctx, sender, msg.Payload)
	case TypeAck:
		return r.handleAck(ctx, sender, msg.Payload)
	case TypePong:
		return nil
	default:
		return fmt.Errorf("router: unknown message type: %s", msg.Type)
	}
}

func (r *Router) handleSend(_ context.Context, sender *Session, payload json.RawMessage) error {
	var env Envelope
	if err := json.Unmarshal(payload, &env); err != nil {
		return fmt.Errorf("router: send: unmarshal envelope: %w", err)
	}

	if err := validateEnvelope(sender, env); err != nil {
		return fmt.Errorf("router: send: %w", err)
	}

	r.msgSenders.Store(env.MessageID, env.SenderAddr)

	if recipientSession, online := r.sessions.Get(env.RecipientAddr); online {
		if err := sendJSON(recipientSession, TypeDeliver, env); err != nil {
			r.logger.Warn("router: send: direct delivery failed, saving as pending",
				"message_id", env.MessageID, "err", err)
		} else {
			return sendJSON(sender, TypeServerAck, ServerAckPayload{
				MessageID: env.MessageID,
				Status:    "delivered",
			})
		}
	}

	if err := r.pending.Save(env); err != nil {
		return fmt.Errorf("router: send: save pending: %w", err)
	}

	return sendJSON(sender, TypeServerAck, ServerAckPayload{
		MessageID: env.MessageID,
		Status:    "pending",
	})
}

func (r *Router) handleAck(_ context.Context, _ *Session, payload json.RawMessage) error {
	var ack AckPayload
	if err := json.Unmarshal(payload, &ack); err != nil {
		return fmt.Errorf("router: ack: unmarshal: %w", err)
	}

	if ack.MessageID == "" {
		return fmt.Errorf("router: ack: message_id is empty")
	}

	val, ok := r.msgSenders.Load(ack.MessageID)
	if !ok {
		return fmt.Errorf("router: ack: unknown message_id: %s", ack.MessageID)
	}

	senderAddr := val.(string)
	senderSession, online := r.sessions.Get(senderAddr)
	if !online {
		r.logger.Info("router: ack: original sender is offline",
			"message_id", ack.MessageID, "sender_addr", senderAddr)
		return nil
	}

	if err := sendJSON(senderSession, TypeServerAck, ServerAckPayload{
		MessageID: ack.MessageID,
		Status:    "delivered",
	}); err != nil {
		r.logger.Warn("router: ack: failed to notify sender",
			"message_id", ack.MessageID, "err", err)
	}

	return nil
}

func validateEnvelope(sender *Session, env Envelope) error {
	if env.MessageID == "" {
		return fmt.Errorf("message_id is empty")
	}
	if env.ChatID == "" {
		return fmt.Errorf("chat_id is empty")
	}
	if env.RecipientAddr == "" || !isValidEthAddress(env.RecipientAddr) {
		return fmt.Errorf("recipient_addr is invalid")
	}
	if env.CID == "" {
		return fmt.Errorf("cid is empty")
	}
	if env.Timestamp == 0 {
		return fmt.Errorf("timestamp is zero")
	}
	if !strings.EqualFold(env.SenderAddr, sender.Address) {
		return fmt.Errorf("sender_addr mismatch")
	}
	if env.Signature == "" {
		return fmt.Errorf("signature is empty")
	}
	return nil
}

func sendJSON(session *Session, msgType MessageType, payload any) error {
	data, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("marshal payload: %w", err)
	}
	msg, err := json.Marshal(Message{
		Type:    msgType,
		Payload: data,
	})
	if err != nil {
		return fmt.Errorf("marshal message: %w", err)
	}
	return session.SendMessage(msg)
}

func isValidEthAddress(addr string) bool {
	if len(addr) != 42 {
		return false
	}
	if addr[:2] != "0x" && addr[:2] != "0X" {
		return false
	}
	for _, c := range addr[2:] {
		if !((c >= '0' && c <= '9') || (c >= 'a' && c <= 'f') || (c >= 'A' && c <= 'F')) {
			return false
		}
	}
	return true
}
