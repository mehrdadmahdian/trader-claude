package api

import (
	"context"
	"encoding/json"
	"log"

	"github.com/gofiber/contrib/websocket"
	"github.com/gofiber/fiber/v2"
	"github.com/redis/go-redis/v9"
	"gorm.io/gorm"

	"github.com/trader-claude/backend/internal/models"
)

type notificationHandler struct {
	db  *gorm.DB
	rdb *redis.Client
}

func newNotificationHandler(db *gorm.DB, rdb *redis.Client) *notificationHandler {
	return &notificationHandler{db: db, rdb: rdb}
}

// GET /api/v1/notifications?limit=20&offset=0
func (h *notificationHandler) listNotifications(c *fiber.Ctx) error {
	limit := c.QueryInt("limit", 20)
	if limit > 100 {
		limit = 100
	}
	offset := c.QueryInt("offset", 0)

	var total int64
	h.db.Model(&models.Notification{}).Count(&total)

	var notifs []models.Notification
	if err := h.db.Order("created_at DESC").Limit(limit).Offset(offset).Find(&notifs).Error; err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}

	return c.JSON(fiber.Map{
		"data":      notifs,
		"total":     total,
		"page":      offset/limit + 1,
		"page_size": limit,
	})
}

// PATCH /api/v1/notifications/:id/read
func (h *notificationHandler) markRead(c *fiber.Ctx) error {
	id, err := c.ParamsInt("id")
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "invalid id"})
	}
	if err := h.db.Model(&models.Notification{}).
		Where("id = ?", id).
		Update("read", true).Error; err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}
	return c.SendStatus(fiber.StatusNoContent)
}

// POST /api/v1/notifications/read-all
func (h *notificationHandler) markAllRead(c *fiber.Ctx) error {
	if err := h.db.Model(&models.Notification{}).
		Where("read = ?", false).
		Update("read", true).Error; err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}
	return c.SendStatus(fiber.StatusNoContent)
}

// GET /api/v1/notifications/unread-count
func (h *notificationHandler) unreadCount(c *fiber.Ctx) error {
	var count int64
	h.db.Model(&models.Notification{}).Where("read = ?", false).Count(&count)
	return c.JSON(fiber.Map{"count": count})
}

// GET /ws/notifications — subscribes to Redis pub/sub and pushes new notifications over WS.
// On connect: sends the 5 most recent unread notifications immediately.
// Ongoing: whenever the alert evaluator publishes a notification ID, fetches and pushes the full object.
func (h *notificationHandler) notificationsWS(conn *websocket.Conn) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	sub := h.rdb.Subscribe(ctx, "notifications:new")
	defer sub.Close()

	// Send recent unread notifications on connect (oldest first)
	var recent []models.Notification
	h.db.Where("read = ?", false).Order("created_at DESC").Limit(5).Find(&recent)
	for i := len(recent) - 1; i >= 0; i-- {
		if b, err := json.Marshal(recent[i]); err == nil {
			conn.WriteMessage(websocket.TextMessage, b)
		}
	}

	ch := sub.Channel()
	for {
		select {
		case msg, ok := <-ch:
			if !ok {
				return
			}
			var notif models.Notification
			if err := h.db.First(&notif, msg.Payload).Error; err != nil {
				log.Printf("ws/notifications: notification %s not found: %v", msg.Payload, err)
				continue
			}
			b, err := json.Marshal(notif)
			if err != nil {
				continue
			}
			if err := conn.WriteMessage(websocket.TextMessage, b); err != nil {
				return
			}
		case <-ctx.Done():
			return
		}
	}
}
