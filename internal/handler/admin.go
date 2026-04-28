package handler

import (
	"strings"

	"github.com/gofiber/fiber/v2"
)

type blockRequest struct {
	Address string `json:"address"`
}

func AdminLocalOnly() fiber.Handler {
	return func(c *fiber.Ctx) error {
		ip := c.IP()
		if ip != "127.0.0.1" && ip != "::1" {
			return fiber.ErrForbidden
		}
		return c.Next()
	}
}

func (h *Handler) ListBlocked(c *fiber.Ctx) error {
	return c.JSON(fiber.Map{"blocked": h.blocklist.List()})
}

func (h *Handler) BlockAddress(c *fiber.Ctx) error {
	var req blockRequest
	if err := c.BodyParser(&req); err != nil {
		return fiber.NewError(fiber.StatusBadRequest, "invalid request body")
	}
	req.Address = strings.ToLower(strings.TrimSpace(req.Address))
	if req.Address == "" {
		return fiber.NewError(fiber.StatusBadRequest, "address is required")
	}

	added := h.blocklist.Block(req.Address)
	h.sessions.Delete(req.Address) // kick if currently connected
	h.log.Info("address blocked", "address", req.Address, "was_new", added)

	return c.JSON(fiber.Map{"address": req.Address, "blocked": true})
}

func (h *Handler) UnblockAddress(c *fiber.Ctx) error {
	address := strings.ToLower(strings.TrimSpace(c.Params("address")))
	if address == "" {
		return fiber.NewError(fiber.StatusBadRequest, "address is required")
	}

	removed := h.blocklist.Unblock(address)
	h.log.Info("address unblocked", "address", address, "was_blocked", removed)

	return c.JSON(fiber.Map{"address": address, "unblocked": removed})
}
