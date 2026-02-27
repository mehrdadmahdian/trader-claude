package alert

import (
	"context"
	"fmt"
	"log"
	"math"
	"time"

	"github.com/redis/go-redis/v9"
	"gorm.io/gorm"

	"github.com/trader-claude/backend/internal/models"
	"github.com/trader-claude/backend/internal/price"
)

const (
	evalInterval    = 60 * time.Second
	redisPubChannel = "notifications:new"
)

// Evaluator checks active alerts every 60 seconds and fires notifications.
type Evaluator struct {
	db       *gorm.DB
	priceSvc *price.Service
	rdb      *redis.Client
}

// NewEvaluator creates an Evaluator.
func NewEvaluator(db *gorm.DB, priceSvc *price.Service, rdb *redis.Client) *Evaluator {
	return &Evaluator{db: db, priceSvc: priceSvc, rdb: rdb}
}

// Start launches the evaluation loop in a background goroutine.
// Cancel ctx to stop.
func (e *Evaluator) Start(ctx context.Context) {
	go func() {
		ticker := time.NewTicker(evalInterval)
		defer ticker.Stop()
		for {
			select {
			case <-ticker.C:
				e.evaluate(ctx)
			case <-ctx.Done():
				return
			}
		}
	}()
}

func (e *Evaluator) evaluate(ctx context.Context) {
	var alerts []models.Alert
	if err := e.db.WithContext(ctx).
		Where("status = ?", models.AlertStatusActive).
		Find(&alerts).Error; err != nil {
		log.Printf("alert evaluator: failed to load alerts: %v", err)
		return
	}
	for i := range alerts {
		e.evalAlert(ctx, alerts[i])
	}
}

func (e *Evaluator) evalAlert(ctx context.Context, alert models.Alert) {
	// Cooldown check for recurring alerts
	if alert.RecurringEnabled && alert.LastFiredAt != nil {
		cooldown := time.Duration(alert.CooldownMinutes) * time.Minute
		if time.Since(*alert.LastFiredAt) < cooldown {
			return
		}
	}

	currentPrice, err := e.priceSvc.GetPrice(ctx, alert.AdapterID, alert.Symbol)
	if err != nil {
		log.Printf("alert evaluator: price fetch failed for %s/%s: %v", alert.AdapterID, alert.Symbol, err)
		return
	}

	triggered, msg := checkCondition(alert, currentPrice)
	if !triggered {
		return
	}

	// Create notification
	notif := models.Notification{
		Type:  models.NotificationTypeAlert,
		Title: alert.Name + " triggered",
		Body:  msg,
		Metadata: models.JSON{
			"alert_id": alert.ID,
			"symbol":   alert.Symbol,
			"price":    currentPrice,
		},
	}
	if err := e.db.WithContext(ctx).Create(&notif).Error; err != nil {
		log.Printf("alert evaluator: failed to create notification: %v", err)
		return
	}

	// Update alert state
	now := time.Now()
	updates := map[string]interface{}{"last_fired_at": now}
	if !alert.RecurringEnabled {
		updates["status"] = models.AlertStatusTriggered
		updates["triggered_at"] = now
	}
	if err := e.db.WithContext(ctx).Model(&alert).Updates(updates).Error; err != nil {
		log.Printf("alert evaluator: failed to update alert %d: %v", alert.ID, err)
	}

	// Publish notification ID to Redis for WebSocket push
	e.rdb.Publish(ctx, redisPubChannel, fmt.Sprintf("%d", notif.ID))
}

// checkCondition evaluates whether the current price satisfies the alert condition.
// Returns (triggered bool, human-readable message string).
func checkCondition(alert models.Alert, currentPrice float64) (bool, string) {
	switch alert.Condition {
	case models.AlertConditionPriceAbove:
		if currentPrice > alert.Threshold {
			return true, fmt.Sprintf(
				"%s is above $%.4f (current: $%.4f)",
				alert.Symbol, alert.Threshold, currentPrice,
			)
		}
	case models.AlertConditionPriceBelow:
		if currentPrice < alert.Threshold {
			return true, fmt.Sprintf(
				"%s is below $%.4f (current: $%.4f)",
				alert.Symbol, alert.Threshold, currentPrice,
			)
		}
	case models.AlertConditionPriceChange:
		if alert.BasePrice == 0 {
			return false, ""
		}
		changePct := math.Abs((currentPrice-alert.BasePrice)/alert.BasePrice) * 100
		if changePct >= alert.Threshold {
			direction := "up"
			if currentPrice < alert.BasePrice {
				direction = "down"
			}
			return true, fmt.Sprintf(
				"%s moved %s %.2f%% from base $%.4f (current: $%.4f)",
				alert.Symbol, direction, changePct, alert.BasePrice, currentPrice,
			)
		}
	}
	return false, ""
}
