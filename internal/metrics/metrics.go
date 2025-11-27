package metrics

// Package metrics provides simple atomic counters for the benchmark runtime.

import (
	"sync/atomic"
	"time"
)

// Metrics keeps track of attempts and outcomes. All fields are safe for
// concurrent use via atomic operations.
type Metrics struct {
	Attempts atomic.Int64
	Success  atomic.Int64
	Fail     atomic.Int64
	Start    time.Time
}

// New creates a new Metrics struct initialized with the current start time.
func New() *Metrics {
	return &Metrics{Start: time.Now()}
}

// Snapshot returns current counts and elapsed time. RPS is not computed here
// because reporters may choose different time windows; callers can compute
// deltas between snapshots if needed.
func (m *Metrics) Snapshot() (attempts, success, fail int64, elapsed time.Duration) {
	att := m.Attempts.Load()
	suc := m.Success.Load()
	fal := m.Fail.Load()
	el := time.Since(m.Start)

	return att, suc, fal, el
}
