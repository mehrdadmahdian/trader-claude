package api

import (
	"context"
	"encoding/json"

	"github.com/gofiber/contrib/websocket"
	"github.com/gofiber/fiber/v2"
	"github.com/redis/go-redis/v9"
	"gorm.io/gorm"

	"github.com/trader-claude/backend/internal/auth"
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
	userID := auth.GetUserID(c)

	var total int64
	h.db.Model(&models.Notification{}).Where("user_id = ?", userID).Count(&total)

	var notifs []models.Notification
	if err := h.db.Where("user_id = ?", userID).Order("created_at DESC").Limit(limit).Offset(offset).Find(&notifs).Error; err != nil {
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
		Where("id = ? AND user_id = ?", id, auth.GetUserID(c)).
		Update("read", true).Error; err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}
	return c.SendStatus(fiber.StatusNoContent)
}

// POST /api/v1/notifications/read-all
func (h *notificationHandler) markAllRead(c *fiber.Ctx) error {
	if err := h.db.Model(&models.Notification{}).
		Where("user_id = ? AND read = ?", auth.GetUserID(c), false).
		Update("read", true).Error; err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}
	return c.SendStatus(fiber.StatusNoContent)
}

// GET /api/v1/notifications/unread-count
func (h *notificationHandler) unreadCount(c *fiber.Ctx) error {
	var count int64
	h.db.Model(&models.Notification{}).Where("user_id = ? AND read = ?", auth.GetUserID(c), false).Count(&count)
	return c.JSON(fiber.Map{"count": count})
}

// GET /ws/notifications — subscribes to Redis pub/sub and pushes new notifications over WS.
// On connect: sends the 5 most recent unread notifications immediately (scoped to the authenticated user).
// Ongoing: whenever the alert evaluator publishes a notification ID, fetches and pushes the full object
// only if it belongs to this user.
func (h *notificationHandler) notificationsWS(conn *websocket.Conn) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Get user ID from the WS connection locals (set by upgrade middleware)
	userID, _ := conn.Locals("user_id").(int64)

	sub := h.rdb.Subscribe(ctx, "notifications:new")
	defer sub.Close()

	// Send recent unread notifications on connect (oldest first) — scoped to user
	var recent []models.Notification
	h.db.Where("read = ? AND user_id = ?", false, userID).Order("created_at DESC").Limit(5).Find(&recent)
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
			// Only deliver notifications that belong to THIS user
			if err := h.db.Where("id = ? AND user_id = ?", msg.Payload, userID).First(&notif).Error; err != nil {
				// Not found or belongs to another user — skip silently
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
