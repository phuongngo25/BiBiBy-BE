package orchestrator

import (
	"context"
	"log"
	"time"
)

// Orchestrator manages the lifecycle of background jobs.
type Orchestrator struct {
	store  JobStore
	worker *Worker
}

// NewOrchestrator initializes the job orchestrator and starts the background TTL sweeper.
func NewOrchestrator(store JobStore, worker *Worker) *Orchestrator {
	o := &Orchestrator{
		store:  store,
		worker: worker,
	}

	// Start background sweeper
	go o.startStaleJobSweeper()

	return o
}

// Submit enqueues a new job safely with idempotency checks.
func (o *Orchestrator) Submit(ctx context.Context, job *Job) error {
	// 1. IDEMPOTENT SUBMISSION: Check if job already exists
	existing, err := o.store.Get(job.ID)
	if err != nil && err != ErrJobNotFound {
		return err
	}
	if existing != nil {
		return ErrJobAlreadyExists
	}

	// 2. DEFAULT STATE: Enforce secure initialization bounds
	job.Status = StatusPending
	job.Done = false
	job.CreatedAt = time.Now()
	job.UpdatedAt = time.Now()

	if err := o.store.Save(job); err != nil {
		return err
	}

	// Structured Logging: Submission
	log.Printf("{\"event\":\"job_submitted\",\"job_id\":\"%s\",\"status\":\"%s\",\"duration_ms\":0}", job.ID, StatusPending)

	// Async dispatch (in a real system, this would push to a message queue or worker pool)
	// For this scope, we dispatch the worker in a goroutine directly.
	go o.worker.Process(context.Background(), job)

	return nil
}

// startStaleJobSweeper hunts down ghost jobs and bloated memory.
func (o *Orchestrator) startStaleJobSweeper() {
	ticker := time.NewTicker(5 * time.Minute)
	defer ticker.Stop()

	for range ticker.C {
		now := time.Now()
		
		// 1. Ghost Jobs (Stuck Running > 1 Hour)
		ghostCutoff := now.Add(-1 * time.Hour)
		staleJobs, err := o.store.GetStaleRunningJobs(ghostCutoff)
		if err == nil && len(staleJobs) > 0 {
			for _, job := range staleJobs {
				job.Status = StatusFailed
				job.Done = true
				job.Error = "system timeout: job stuck in running state for > 1 hour"
				job.UpdatedAt = now
				_ = o.store.Save(job)
				
				log.Printf("{\"event\":\"job_ghost_reaped\",\"job_id\":\"%s\",\"status\":\"%s\",\"duration_ms\":0}", job.ID, StatusFailed)
			}
		}

		// 2. Ultimate Eviction (TTL > 24 hours for Terminal states)
		evictCutoff := now.Add(-24 * time.Hour)
		_ = o.store.DeleteOldJobs(evictCutoff)
	}
}
