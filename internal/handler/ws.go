package handler

import (
	"encoding/json"
	"github.com/gofiber/contrib/websocket"
	"zephyr-relay/internal/relay"
)

func (h *Handler) HandleWS(c *websocket.Conn) {
	remoteAddr := c.RemoteAddr().String()
	h.log.Info("websocket connected", "remote_addr", remoteAddr)
	defer func() {
		h.log.Info("websocket disconnected", "remote_addr", remoteAddr)
	}()
	for {
		_, msg, err := c.ReadMessage()
		if err != nil {
			h.log.Warn("websocket read error", "remote_addr", remoteAddr, "err", err)
			break
		}
		var m relay.Message
		if jsonErr := json.Unmarshal(msg, &m); jsonErr == nil {
			h.log.Info("websocket message received", "remote_addr", remoteAddr, "type", m.Type)
		} else {
			h.log.Warn("websocket invalid json", "remote_addr", remoteAddr, "err", jsonErr)
		}
		_ = msg
	}
}
