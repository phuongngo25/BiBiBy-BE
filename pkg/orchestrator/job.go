package orchestrator

import (
	"errors"
	"time"
)

// JobStatus represents the state of an orchestrated task.
type JobStatus string

const (
	StatusPending   JobStatus = "pending"
	StatusRunning   JobStatus = "running"
	StatusCompleted JobStatus = "completed"
	StatusFailed    JobStatus = "failed"
)

var (
	ErrJobAlreadyExists = errors.New("job already exists")
	ErrJobNotFound      = errors.New("job not found")
)

// Job represents a single background task managed by the orchestrator.
type Job struct {
	ID        string    `json:"id"`
	Type      string    `json:"job_type"`
	Status    JobStatus `json:"status"`
	Done      bool      `json:"done"`
	Payload   []byte    `json:"-"`
	Error     string    `json:"error,omitempty"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

// JobStore defines the contract for persisting Job states.
type JobStore interface {
	Save(job *Job) error
	Get(id string) (*Job, error)
	GetStaleRunningJobs(cutoff time.Time) ([]*Job, error)
	DeleteOldJobs(cutoff time.Time) error
}
