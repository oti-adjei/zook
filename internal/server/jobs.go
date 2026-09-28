// Package server exposes zook's deploy engine over HTTP: an async job
// registry with per-stack serialization, authenticated deploy/rollback
// endpoints, stack status, and a GitHub webhook receiver.
package server

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"sync"
	"time"
)

// JobStatus is the lifecycle state of a job.
type JobStatus string

const (
	Queued  JobStatus = "queued"
	Running JobStatus = "running"
	Done    JobStatus = "done"
	Failed  JobStatus = "failed"
)

// Job is one enqueued deploy or rollback. Records are in-memory only; the
// durable record of a deploy is the stack's state.json and deploy log.
type Job struct {
	ID         string    `json:"id"`
	Kind       string    `json:"kind"` // "deploy" | "rollback"
	Stack      string    `json:"stack"`
	Version    string    `json:"version,omitempty"`
	Status     JobStatus `json:"status"`
	Err        string    `json:"error,omitempty"`
	CreatedAt  time.Time `json:"created_at"`
	FinishedAt time.Time `json:"finished_at,omitempty"`
}

// ErrStackBusy is returned when a deploy is already in flight for a stack.
// The engine assumes single-writer; the server enforces it.
var ErrStackBusy = errors.New("deploy already in progress for stack")

// Jobs tracks in-flight and completed jobs, serializing work per stack.
type Jobs struct {
	mu      sync.Mutex
	jobs    map[string]*Job
	running map[string]bool // stack name → in flight
}

// NewJobs returns an empty registry.
func NewJobs() *Jobs {
	return &Jobs{jobs: map[string]*Job{}, running: map[string]bool{}}
}

func newID() string {
	var b [8]byte
	if _, err := rand.Read(b[:]); err != nil {
		// crypto/rand failing is unrecoverable; fall back to a time-based id
		return time.Now().UTC().Format("20060102T150405.000000000")
	}
	return hex.EncodeToString(b[:])
}

// Start registers a job for stack and runs fn in a goroutine. It refuses
// with ErrStackBusy while another job for the same stack is in flight.
// The returned Job is a snapshot of the queued job; the live record is
// mutated only under the registry's mutex (see Get).
func (j *Jobs) Start(kind, stack, version string, fn func(context.Context) error) (Job, error) {
	j.mu.Lock()
	if j.running[stack] {
		j.mu.Unlock()
		return Job{}, ErrStackBusy
	}
	job := &Job{ID: newID(), Kind: kind, Stack: stack, Version: version, Status: Queued, CreatedAt: time.Now().UTC()}
	j.jobs[job.ID] = job
	j.running[stack] = true
	snapshot := *job
	j.mu.Unlock()

	go func() {
		j.mu.Lock()
		job.Status = Running
		j.mu.Unlock()

		err := fn(context.Background())

		j.mu.Lock()
		if err != nil {
			job.Status = Failed
			job.Err = err.Error()
		} else {
			job.Status = Done
		}
		job.FinishedAt = time.Now().UTC()
		delete(j.running, stack)
		j.mu.Unlock()
	}()

	return snapshot, nil
}

// Get returns a snapshot of the job with the given id.
func (j *Jobs) Get(id string) (Job, bool) {
	j.mu.Lock()
	defer j.mu.Unlock()
	job, ok := j.jobs[id]
	if !ok {
		return Job{}, false
	}
	return *job, true
}

// Wait blocks until no jobs are in flight. Used by graceful shutdown and
// tests; polls at a short interval.
func (j *Jobs) Wait() {
	for {
		j.mu.Lock()
		busy := len(j.running) > 0
		j.mu.Unlock()
		if !busy {
			return
		}
		time.Sleep(5 * time.Millisecond)
	}
}
