package monitor

import (
	"context"
	"fmt"
	"log"
	"sync"
	"time"

	"github.com/redis/go-redis/v9"
	"gorm.io/gorm"

	"github.com/trader-claude/backend/internal/adapter"
	"github.com/trader-claude/backend/internal/models"
	"github.com/trader-claude/backend/internal/worker"
)

// Manager schedules live strategy polls for all active monitors.
// Each active monitor gets a time.AfterFunc timer; when it fires the poll
// job is submitted to the shared worker pool, then the timer is rescheduled.
type Manager struct {
	db   *gorm.DB
	rdb  *redis.Client
	ds   *adapter.DataService
	pool *worker.WorkerPool

	ctx    context.Context
	cancel context.CancelFunc

	mu     sync.Mutex
	timers map[int64]*time.Timer // monitorID → pending next-poll timer
	active sync.Map              // monitorID → struct{} (set while poll job is running)
}

// NewManager creates a Manager with its own long-lived internal context.
func NewManager(db *gorm.DB, rdb *redis.Client, ds *adapter.DataService, pool *worker.WorkerPool) *Manager {
	ctx, cancel := context.WithCancel(context.Background())
	return &Manager{
		db:     db,
		rdb:    rdb,
		ds:     ds,
		pool:   pool,
		ctx:    ctx,
		cancel: cancel,
		timers: make(map[int64]*time.Timer),
	}
}

// Start loads all active monitors from the DB and schedules a poll for each.
// Call this once during server startup. ctx is used only for the DB query;
// all scheduling uses the Manager's own long-lived context.
func (m *Manager) Start(ctx context.Context) {
	var monitors []models.Monitor
	if err := m.db.WithContext(ctx).
		Where("status = ?", models.MonitorStatusActive).
		Find(&monitors).Error; err != nil {
		log.Printf("[monitor] failed to load monitors on start: %v", err)
		return
	}
	for _, mon := range monitors {
		m.scheduleNext(mon.ID, calcPollInterval(mon.Timeframe))
	}
	log.Printf("[monitor] started %d active monitors", len(monitors))
}

// Add schedules polling for a newly created monitor.
// Call this after inserting the monitor record into the DB.
func (m *Manager) Add(monitorID int64, timeframe string) {
	m.scheduleNext(monitorID, calcPollInterval(timeframe))
}

// Remove cancels the timer for a monitor (called on delete).
func (m *Manager) Remove(monitorID int64) {
	m.cancelTimer(monitorID)
	m.active.Delete(monitorID)
}

// Pause cancels the timer without changing DB status.
// The API handler is responsible for setting status = "paused" in the DB.
func (m *Manager) Pause(monitorID int64) {
	m.cancelTimer(monitorID)
}

// Resume re-schedules a paused monitor.
// The API handler is responsible for setting status = "active" in the DB before calling this.
func (m *Manager) Resume(monitorID int64, timeframe string) {
	m.scheduleNext(monitorID, calcPollInterval(timeframe))
}

// Stop cancels all timers and the manager's internal context.
// Call this during server shutdown before stopping the worker pool.
func (m *Manager) Stop() {
	m.cancel()
	m.mu.Lock()
	defer m.mu.Unlock()
	for id, t := range m.timers {
		t.Stop()
		delete(m.timers, id)
	}
}

// cancelTimer stops and removes the timer for a monitor.
func (m *Manager) cancelTimer(monitorID int64) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if t, ok := m.timers[monitorID]; ok {
		t.Stop()
		delete(m.timers, monitorID)
	}
}

// scheduleNext sets up a time.AfterFunc for the next poll of monitorID.
// When the timer fires:
//  1. If a poll is already running (active map), reschedule and return.
//  2. Otherwise mark active, submit the job to the pool, then reschedule after the job.
func (m *Manager) scheduleNext(monitorID int64, interval time.Duration) {
	m.mu.Lock()
	// Cancel any existing timer before creating a new one.
	if t, ok := m.timers[monitorID]; ok {
		t.Stop()
	}
	m.timers[monitorID] = time.AfterFunc(interval, func() {
		// Skip if a previous poll is still running.
		if _, loaded := m.active.LoadOrStore(monitorID, struct{}{}); loaded {
			m.scheduleNext(monitorID, interval)
			return
		}
		submitted := m.pool.Submit(worker.Job{
			Name: fmt.Sprintf("monitor-poll-%d", monitorID),
			Task: func(jobCtx context.Context) error {
				defer m.active.Delete(monitorID)
				executePoll(jobCtx, m.db, m.rdb, m.ds, monitorID)
				// Reschedule after the poll completes.
				m.scheduleNext(monitorID, interval)
				return nil
			},
		})
		if !submitted {
			// Pool is full — reschedule BEFORE clearing the active flag to avoid
			// a race window where a concurrent goroutine could pass LoadOrStore.
			m.scheduleNext(monitorID, interval)
			m.active.Delete(monitorID)
		}
	})
	m.mu.Unlock()
}
