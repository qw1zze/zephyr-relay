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

func (r *Router) RegisterSender(messageID, senderAddr string) {
	r.msgSenders.Store(messageID, senderAddr)
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

	recipients := resolveRecipients(env)

	r.logger.Info("router: message received",
		"message_id", env.MessageID,
		"chat_id", env.ChatID,
		"from", env.SenderAddr,
		"to", recipients,
		"cid", env.CID,
	)

	r.msgSenders.Store(env.MessageID, env.SenderAddr)

	var delivered, queued int
	for _, recipientAddr := range recipients {
		if recipientSession, online := r.sessions.Get(recipientAddr); online {
			if err := sendJSON(recipientSession, TypeDeliver, env); err != nil {
				r.logger.Warn("router: send: direct delivery failed, saving as pending",
					"message_id", env.MessageID, "recipient", recipientAddr, "err", err)
			} else {
				r.logger.Info("router: message delivered directly",
					"message_id", env.MessageID, "to", recipientAddr)
				delivered++
				continue
			}
		}

		pendingEnv := env
		pendingEnv.RecipientAddr = recipientAddr
		if err := r.pending.Save(pendingEnv); err != nil {
			r.logger.Warn("router: send: save pending failed",
				"message_id", env.MessageID, "recipient", recipientAddr, "err", err)
		} else {
			r.logger.Info("router: message queued as pending",
				"message_id", env.MessageID, "to", recipientAddr)
			queued++
		}
	}

	status := "delivered"
	if delivered == 0 {
		status = "pending"
	} else if queued > 0 {
		status = "partial"
	}

	return sendJSON(sender, TypeServerAck, ServerAckPayload{
		MessageID: env.MessageID,
		Status:    status,
	})
}

func (r *Router) handleAck(_ context.Context, recipient *Session, payload json.RawMessage) error {
	var ack AckPayload
	if err := json.Unmarshal(payload, &ack); err != nil {
		return fmt.Errorf("router: ack: unmarshal: %w", err)
	}

	if ack.MessageID == "" {
		return fmt.Errorf("router: ack: message_id is empty")
	}

	r.logger.Info("router: ack received",
		"message_id", ack.MessageID,
		"from", recipient.Address,
	)

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

func resolveRecipients(env Envelope) []string {
	if len(env.RecipientAddrs) > 0 {
		return env.RecipientAddrs
	}
	if env.RecipientAddr != "" {
		return []string{env.RecipientAddr}
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
	recipients := resolveRecipients(env)
	if len(recipients) == 0 {
		return fmt.Errorf("recipient_addr is invalid")
	}
	for _, addr := range recipients {
		if !isValidEthAddress(addr) {
			return fmt.Errorf("recipient_addr is invalid: %s", addr)
		}
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
