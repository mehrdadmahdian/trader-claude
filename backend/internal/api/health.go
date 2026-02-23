package api

import (
	"context"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/redis/go-redis/v9"
	"gorm.io/gorm"
)

type healthHandler struct {
	db      *gorm.DB
	redis   *redis.Client
	version string
}

type healthResponse struct {
	Status  string `json:"status"`
	DB      string `json:"db"`
	Redis   string `json:"redis"`
	Version string `json:"version"`
}

func newHealthHandler(db *gorm.DB, rdb *redis.Client, version string) *healthHandler {
	return &healthHandler{db: db, redis: rdb, version: version}
}

func (h *healthHandler) check(c *fiber.Ctx) error {
	resp := healthResponse{
		Status:  "ok",
		DB:      "ok",
		Redis:   "ok",
		Version: h.version,
	}

	// Check DB
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	sqlDB, err := h.db.DB()
	if err != nil || sqlDB.PingContext(ctx) != nil {
		resp.Status = "degraded"
		resp.DB = "error"
	}

	// Check Redis
	if err := h.redis.Ping(ctx).Err(); err != nil {
		resp.Status = "degraded"
		resp.Redis = "error"
	}

	statusCode := fiber.StatusOK
	if resp.Status != "ok" {
		statusCode = fiber.StatusServiceUnavailable
	}

	return c.Status(statusCode).JSON(resp)
}
