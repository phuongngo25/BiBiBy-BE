package orchestrator

import (
	"context"
	"fmt"
	"log"
	"math/rand"
	"time"
)

// JobHandler defines the function signature for processing a job payload.
type JobHandler func(ctx context.Context, payload []byte) error

// Worker pulls jobs and executes them with panic safety and retry logic.
type Worker struct {
	store       JobStore
	handlers    map[string]JobHandler
	maxRetries  int
}

// NewWorker initializes a bulletproof worker.
func NewWorker(store JobStore) *Worker {
	return &Worker{
		store:      store,
		handlers:   make(map[string]JobHandler),
		maxRetries: 3,
	}
}

// RegisterHandler binds a handler function to a specific job type.
func (w *Worker) RegisterHandler(jobType string, handler JobHandler) {
	w.handlers[jobType] = handler
}

// Process executes the job with strict Panic Safety and Jitter Retry mechanisms.
func (w *Worker) Process(ctx context.Context, job *Job) {
	startTime := time.Now()
	
	// Structured Logging: Job Start
	log.Printf("{\"event\":\"job_started\",\"job_id\":\"%s\",\"status\":\"%s\",\"duration_ms\":0}", job.ID, StatusRunning)

	// 1. PANIC SAFETY: Ensure worker thread survives any handler catastrophe
	defer func() {
		if r := recover(); r != nil {
			duration := time.Since(startTime).Milliseconds()
			job.Status = StatusFailed
			job.Done = true
			job.Error = fmt.Sprintf("panic: %v", r)
			job.UpdatedAt = time.Now()
			_ = w.store.Save(job)
			
			// Structured Logging: Panic Caught
			log.Printf("{\"event\":\"job_panicked\",\"job_id\":\"%s\",\"status\":\"%s\",\"duration_ms\":%d}", job.ID, StatusFailed, duration)
		}
	}()

	handler, exists := w.handlers[job.Type]
	if !exists {
		job.Status = StatusFailed
		job.Done = true
		job.Error = "no handler registered for job type"
		job.UpdatedAt = time.Now()
		_ = w.store.Save(job)
		return
	}

	// 2. RETRY WITH JITTER: Prevent Thundering Herds
	var err error
	for attempt := 1; attempt <= w.maxRetries; attempt++ {
		err = handler(ctx, job.Payload)
		if err == nil {
			break // Success
		}

		if attempt < w.maxRetries {
			// Backoff: (2s * attempt) + random_jitter(0-500ms)
			baseDelay := time.Duration(attempt) * 2 * time.Second
			jitter := time.Duration(rand.Intn(500)) * time.Millisecond
			time.Sleep(baseDelay + jitter)
		}
	}

	duration := time.Since(startTime).Milliseconds()
	job.UpdatedAt = time.Now()
	job.Done = true

	if err != nil {
		job.Status = StatusFailed
		job.Error = err.Error()
		log.Printf("{\"event\":\"job_failed\",\"job_id\":\"%s\",\"status\":\"%s\",\"duration_ms\":%d}", job.ID, StatusFailed, duration)
	} else {
		job.Status = StatusCompleted
		log.Printf("{\"event\":\"job_completed\",\"job_id\":\"%s\",\"status\":\"%s\",\"duration_ms\":%d}", job.ID, StatusCompleted, duration)
	}

	_ = w.store.Save(job)
}
