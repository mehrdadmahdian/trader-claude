package api

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"strconv"
	"time"

	"github.com/gofiber/fiber/v2"
	"gorm.io/gorm"

	"github.com/trader-claude/backend/internal/analytics"
	"github.com/trader-claude/backend/internal/models"
	"github.com/trader-claude/backend/internal/worker"
)

type analyticsHandler struct {
	db   *gorm.DB
	pool *worker.WorkerPool
}

func newAnalyticsHandler(db *gorm.DB, pool *worker.WorkerPool) *analyticsHandler {
	return &analyticsHandler{db: db, pool: pool}
}

// --- GET /api/v1/backtest/runs/:id/param-heatmap ---
func (h *analyticsHandler) paramHeatmap(c *fiber.Ctx) error {
	runID, err := strconv.ParseInt(c.Params("id"), 10, 64)
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "invalid run ID"})
	}

	xParam := c.Query("x_param")
	yParam := c.Query("y_param")
	if xParam == "" || yParam == "" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "x_param and y_param are required"})
	}
	gridSize, _ := strconv.Atoi(c.Query("grid_size", "10"))

	params := models.JSON{
		"x_param":   xParam,
		"y_param":   yParam,
		"grid_size": gridSize,
	}
	job, err := h.createJob(c.Context(), runID, models.AnalyticsTypeHeatmap, params)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "failed to create job"})
	}

	// Capture values for closure
	jobID := job.ID
	submitted := h.pool.Submit(worker.Job{
		Name: fmt.Sprintf("analytics-heatmap-%d", jobID),
		Task: func(ctx context.Context) error {
			return h.runJob(ctx, jobID, func(runCtx context.Context) (interface{}, error) {
				return analytics.RunHeatmap(runCtx, h.db, runID, xParam, yParam, gridSize)
			})
		},
	})

	if !submitted {
		h.db.Model(&models.AnalyticsResult{}).Where("id = ?", jobID).Updates(map[string]interface{}{
			"status":        models.AnalyticsStatusFailed,
			"error_message": "worker pool is full",
		})
		return c.Status(fiber.StatusServiceUnavailable).JSON(fiber.Map{"error": "worker pool is full"})
	}

	return c.Status(fiber.StatusAccepted).JSON(fiber.Map{"job_id": job.ID, "status": job.Status})
}

// --- POST /api/v1/backtest/runs/:id/monte-carlo ---
func (h *analyticsHandler) monteCarlo(c *fiber.Ctx) error {
	runID, err := strconv.ParseInt(c.Params("id"), 10, 64)
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "invalid run ID"})
	}

	var req struct {
		NumSimulations int     `json:"num_simulations"`
		RuinThreshold  float64 `json:"ruin_threshold"`
	}
	if err := c.BodyParser(&req); err != nil || req.NumSimulations <= 0 {
		req.NumSimulations = 1000
	}
	if req.RuinThreshold <= 0 {
		req.RuinThreshold = 0.5
	}

	params := models.JSON{
		"num_simulations": req.NumSimulations,
		"ruin_threshold":  req.RuinThreshold,
	}
	job, err := h.createJob(c.Context(), runID, models.AnalyticsTypeMonteCarlo, params)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "failed to create job"})
	}

	// Capture values for closure
	jobID := job.ID
	numSims := req.NumSimulations
	ruinThreshold := req.RuinThreshold
	submitted := h.pool.Submit(worker.Job{
		Name: fmt.Sprintf("analytics-montecarlo-%d", jobID),
		Task: func(ctx context.Context) error {
			return h.runJob(ctx, jobID, func(runCtx context.Context) (interface{}, error) {
				return analytics.RunMonteCarlo(runCtx, h.db, runID, numSims, ruinThreshold)
			})
		},
	})

	if !submitted {
		h.db.Model(&models.AnalyticsResult{}).Where("id = ?", jobID).Updates(map[string]interface{}{
			"status":        models.AnalyticsStatusFailed,
			"error_message": "worker pool is full",
		})
		return c.Status(fiber.StatusServiceUnavailable).JSON(fiber.Map{"error": "worker pool is full"})
	}

	return c.Status(fiber.StatusAccepted).JSON(fiber.Map{"job_id": job.ID, "status": job.Status})
}

// --- GET /api/v1/backtest/runs/:id/walk-forward ---
func (h *analyticsHandler) walkForward(c *fiber.Ctx) error {
	runID, err := strconv.ParseInt(c.Params("id"), 10, 64)
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "invalid run ID"})
	}

	numWindows, _ := strconv.Atoi(c.Query("windows", "5"))

	params := models.JSON{"num_windows": numWindows}
	job, err := h.createJob(c.Context(), runID, models.AnalyticsTypeWalkForward, params)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "failed to create job"})
	}

	// Capture values for closure
	jobID := job.ID
	submitted := h.pool.Submit(worker.Job{
		Name: fmt.Sprintf("analytics-walkforward-%d", jobID),
		Task: func(ctx context.Context) error {
			return h.runJob(ctx, jobID, func(runCtx context.Context) (interface{}, error) {
				return analytics.RunWalkForward(runCtx, h.db, runID, numWindows)
			})
		},
	})

	if !submitted {
		h.db.Model(&models.AnalyticsResult{}).Where("id = ?", jobID).Updates(map[string]interface{}{
			"status":        models.AnalyticsStatusFailed,
			"error_message": "worker pool is full",
		})
		return c.Status(fiber.StatusServiceUnavailable).JSON(fiber.Map{"error": "worker pool is full"})
	}

	return c.Status(fiber.StatusAccepted).JSON(fiber.Map{"job_id": job.ID, "status": job.Status})
}

// --- POST /api/v1/backtest/compare ---
func (h *analyticsHandler) compareRuns(c *fiber.Ctx) error {
	var req struct {
		RunIDs []int64 `json:"run_ids"`
	}
	if err := c.BodyParser(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "invalid request body"})
	}
	if len(req.RunIDs) == 0 {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "run_ids required"})
	}

	result, err := analytics.CompareRuns(c.Context(), h.db, req.RunIDs)
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": err.Error()})
	}

	return c.Status(fiber.StatusOK).JSON(result)
}

// --- GET /api/v1/analytics/jobs/:jobId ---
func (h *analyticsHandler) getJob(c *fiber.Ctx) error {
	jobID, err := strconv.ParseInt(c.Params("jobId"), 10, 64)
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "invalid job ID"})
	}

	var job models.AnalyticsResult
	if err := h.db.First(&job, jobID).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return c.Status(fiber.StatusNotFound).JSON(fiber.Map{"error": "job not found"})
		}
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "failed to load job"})
	}

	return c.Status(fiber.StatusOK).JSON(fiber.Map{
		"id":           job.ID,
		"status":       job.Status,
		"type":         job.Type,
		"result":       job.Result,
		"error":        job.ErrorMessage,
		"created_at":   job.CreatedAt,
		"completed_at": job.CompletedAt,
	})
}

// --- helpers ---

func (h *analyticsHandler) createJob(ctx context.Context, backtestRunID int64, jobType string, params models.JSON) (*models.AnalyticsResult, error) {
	job := &models.AnalyticsResult{
		BacktestRunID: backtestRunID,
		Type:          jobType,
		Status:        models.AnalyticsStatusPending,
		Params:        params,
	}
	if err := h.db.WithContext(ctx).Create(job).Error; err != nil {
		return nil, err
	}
	return job, nil
}

func (h *analyticsHandler) runJob(ctx context.Context, jobID int64, fn func(context.Context) (interface{}, error)) error {
	// Mark running
	if err := h.db.WithContext(ctx).Model(&models.AnalyticsResult{}).Where("id = ?", jobID).Update("status", models.AnalyticsStatusRunning).Error; err != nil {
		log.Printf("[analytics %d] failed to update status to running: %v", jobID, err)
	}

	result, err := fn(ctx)

	now := time.Now()
	if err != nil {
		if updateErr := h.db.WithContext(ctx).Model(&models.AnalyticsResult{}).Where("id = ?", jobID).Updates(map[string]interface{}{
			"status":        models.AnalyticsStatusFailed,
			"error_message": err.Error(),
			"completed_at":  now,
		}).Error; updateErr != nil {
			log.Printf("[analytics %d] failed to update DB on failure: %v", jobID, updateErr)
		}
		log.Printf("[analytics %d] failed: %v", jobID, err)
		return err
	}

	// Serialize result to JSON
	b, _ := json.Marshal(result)
	var resultJSON models.JSON
	json.Unmarshal(b, &resultJSON)

	if updateErr := h.db.WithContext(ctx).Model(&models.AnalyticsResult{}).Where("id = ?", jobID).Updates(map[string]interface{}{
		"status":       models.AnalyticsStatusCompleted,
		"result":       resultJSON,
		"completed_at": now,
	}).Error; updateErr != nil {
		log.Printf("[analytics %d] failed to save results: %v", jobID, updateErr)
		return updateErr
	}

	log.Printf("[analytics %d] completed", jobID)
	return nil
}
