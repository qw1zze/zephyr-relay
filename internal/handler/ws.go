package handler

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/gofiber/contrib/websocket"
	"zephyr-relay/internal/relay"
)

func (h *Handler) HandleWS(c *websocket.Conn) {
	remoteAddr := c.RemoteAddr().String()
	h.log.Info("websocket connected", "remote_addr", remoteAddr)
	defer func() {
		h.log.Info("websocket disconnected", "remote_addr", remoteAddr)
	}()

	nonce, err := h.auth.NewChallenge()
	if err != nil {
		h.log.Error("failed to generate challenge", "remote_addr", remoteAddr, "err", err)
		_ = c.Close()
		return
	}

	challengeMsg, _ := json.Marshal(relay.Message{
		Type:    relay.TypeChallenge,
		Payload: marshalPayload(relay.ChallengePayload{Nonce: nonce}),
	})
	if err := c.WriteMessage(websocket.TextMessage, challengeMsg); err != nil {
		h.log.Error("failed to send challenge", "remote_addr", remoteAddr, "err", err)
		return
	}

	_ = c.SetReadDeadline(time.Now().Add(time.Duration(h.cfg.ChallengesTTLSec) * time.Second))

	_, raw, err := c.ReadMessage()
	if err != nil {
		h.log.Warn("failed to read auth message", "remote_addr", remoteAddr, "err", err)
		return
	}

	var m relay.Message
	if err := json.Unmarshal(raw, &m); err != nil || m.Type != relay.TypeAuth {
		writeError(c, "expected auth message")
		return
	}

	var payload relay.AuthPayload
	if err := json.Unmarshal(m.Payload, &payload); err != nil {
		writeError(c, "invalid auth payload")
		return
	}

	ok, err := h.auth.Verify(nonce, payload.Address, payload.Signature)
	if err != nil || !ok {
		h.log.Warn("authentication failed", "remote_addr", remoteAddr, "address", payload.Address, "err", err)
		writeError(c, "authentication failed")
		_ = c.Close()
		return
	}

	_ = c.SetReadDeadline(time.Time{})

	authOK, _ := json.Marshal(relay.Message{
		Type:    relay.TypeAuthOK,
		Payload: json.RawMessage("{}"),
	})
	if err := c.WriteMessage(websocket.TextMessage, authOK); err != nil {
		h.log.Error("failed to send auth_ok", "remote_addr", remoteAddr, "err", err)
		return
	}

	address := payload.Address
	h.log.Info("client authenticated", "remote_addr", remoteAddr, "address", address)

	if h.blocklist.IsBlocked(address) {
		h.log.Warn("blocked address rejected", "remote_addr", remoteAddr, "address", address)
		writeError(c, "access denied")
		_ = c.Close()
		return
	}

	session := relay.NewSession(address, c)
	h.sessions.Add(address, session)
	defer h.sessions.Delete(address)

	go session.WriteLoop(h.log)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	pongTimeout := time.Duration(h.cfg.PongTimeout) * time.Second

	c.SetPingHandler(func(data string) error {
		_ = c.SetReadDeadline(time.Now().Add(pongTimeout))
		return c.WriteControl(websocket.PongMessage, []byte(data), time.Now().Add(time.Second))
	})

	c.SetPongHandler(func(_ string) error {
		h.log.Debug("pong received", "address", address)
		_ = c.SetReadDeadline(time.Now().Add(pongTimeout))
		return nil
	})

	pingInterval := time.Duration(h.cfg.PingInterval) * time.Second
	go func() {
		ticker := time.NewTicker(pingInterval)
		defer ticker.Stop()
		for {
			select {
			case <-ticker.C:
				ping, _ := json.Marshal(relay.Message{Type: relay.TypePing})
				if err := session.SendMessage(ping); err != nil {
					return
				}
			case <-session.Done:
				return
			}
		}
	}()

	items, _ := h.pending.GetAll(address)
	if len(items) > 0 {
		h.log.Info("flushing pending messages", "address", address, "count", len(items))
	}
	for _, item := range items {
		if err := wsSendJSON(session, relay.TypeDeliver, item.Envelope); err != nil {
			h.log.Warn("failed to deliver pending envelope",
				"address", address, "message_id", item.Envelope.MessageID, "err", err)
		} else {
			h.log.Info("pending message delivered", "address", address, "message_id", item.Envelope.MessageID)
			h.router.RegisterSender(item.Envelope.MessageID, item.Envelope.SenderAddr)
		}
	}
	_ = h.pending.DeleteAll(address)

	for {
		_, data, err := c.ReadMessage()
		if err != nil {
			break
		}
		var msg relay.Message
		if err := json.Unmarshal(data, &msg); err != nil {
			h.log.Error("invalid message", "address", address, "error", err)
			continue
		}
		h.log.Debug("message received", "address", address, "type", msg.Type)
		if err := h.router.Route(ctx, session, msg); err != nil {
			h.log.Error("route error", "address", address, "type", msg.Type, "error", err)
		}
	}
}

func wsSendJSON(session *relay.Session, msgType relay.MessageType, payload any) error {
	data, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("marshal payload: %w", err)
	}
	msg, err := json.Marshal(relay.Message{Type: msgType, Payload: data})
	if err != nil {
		return fmt.Errorf("marshal message: %w", err)
	}
	return session.SendMessage(msg)
}

func marshalPayload(v any) json.RawMessage {
	b, _ := json.Marshal(v)
	return b
}

func writeError(c *websocket.Conn, msg string) {
	type errPayload struct {
		Error string `json:"error"`
	}
	b, _ := json.Marshal(relay.Message{
		Type:    "error",
		Payload: marshalPayload(errPayload{Error: msg}),
	})
	_ = c.WriteMessage(websocket.TextMessage, b)
}
