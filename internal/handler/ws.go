package handler

import (
	"encoding/json"
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

	h.log.Info("client authenticated", "remote_addr", remoteAddr, "address", payload.Address)

	for {
		_, msg, err := c.ReadMessage()
		if err != nil {
			break
		}
		_ = msg
	}
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
