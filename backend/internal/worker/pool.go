package worker

import (
	"context"
	"log"
	"sync"
)

// Job is a unit of work to be executed by the pool
type Job struct {
	Name string
	Task func(ctx context.Context) error
}

// WorkerPool is a simple goroutine pool
type WorkerPool struct {
	jobs    chan Job
	workers int
	wg      sync.WaitGroup
	ctx     context.Context
	cancel  context.CancelFunc
}

// NewPool creates a new WorkerPool with the given concurrency
func NewPool(workers int) *WorkerPool {
	ctx, cancel := context.WithCancel(context.Background())
	return &WorkerPool{
		jobs:    make(chan Job, workers*10),
		workers: workers,
		ctx:     ctx,
		cancel:  cancel,
	}
}

// Start launches the worker goroutines
func (p *WorkerPool) Start() {
	for i := 0; i < p.workers; i++ {
		p.wg.Add(1)
		go p.runWorker(i)
	}
	log.Printf("[worker] pool started with %d workers", p.workers)
}

// Submit enqueues a job; returns false if the pool is full or stopped
func (p *WorkerPool) Submit(job Job) bool {
	select {
	case p.jobs <- job:
		return true
	case <-p.ctx.Done():
		return false
	default:
		return false
	}
}

// Stop gracefully shuts down the pool, waiting for in-flight jobs to finish
func (p *WorkerPool) Stop() {
	p.cancel()
	close(p.jobs)
	p.wg.Wait()
	log.Println("[worker] pool stopped")
}

func (p *WorkerPool) runWorker(id int) {
	defer p.wg.Done()
	for job := range p.jobs {
		func() {
			defer func() {
				if r := recover(); r != nil {
					log.Printf("[worker %d] panic in job %q: %v", id, job.Name, r)
				}
			}()
			if err := job.Task(p.ctx); err != nil {
				log.Printf("[worker %d] job %q failed: %v", id, job.Name, err)
			}
		}()
	}
}
