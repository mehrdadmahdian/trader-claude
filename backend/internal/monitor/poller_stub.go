package monitor

import (
	"context"

	"github.com/redis/go-redis/v9"
	"gorm.io/gorm"

	"github.com/trader-claude/backend/internal/adapter"
)

// executePoll is a stub — will be replaced by Task B4.
func executePoll(ctx context.Context, db *gorm.DB, rdb *redis.Client, ds *adapter.DataService, monitorID int64) {
	// placeholder
}
