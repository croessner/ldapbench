package metrics

// Package metrics provides simple atomic counters and latency statistics for the
// benchmark runtime.

import (
	"sort"
	"sync"
	"sync/atomic"
	"time"
)

// Metrics keeps track of attempts and outcomes and request latencies.
// Count fields are safe for concurrent use via atomic operations. Latency
// collection is protected by an internal mutex.
type Metrics struct {
	Attempts atomic.Int64
	Success  atomic.Int64
	Fail     atomic.Int64
	Start    time.Time

	// Lat holds per-request latency measurements.
	Lat *LatencyRecorder
}

// New creates a new Metrics struct initialized with the current start time.
func New() *Metrics {
	return &Metrics{Start: time.Now(), Lat: NewLatencyRecorder(20000)}
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

// LatencyStats is an immutable snapshot of latency metrics.
type LatencyStats struct {
	Count int64
	Avg   time.Duration
	P50   time.Duration
	P95   time.Duration
	P99   time.Duration
}

// LatencyRecorder collects latencies both for a complete run (cumulative) and
// for the current reporting window (window). It keeps a bounded sample of the
// cumulative latencies to compute percentiles without unbounded memory growth.
type LatencyRecorder struct {
	mu sync.Mutex

	// cumulative
	totalCount int64
	totalSum   time.Duration
	// reservoir sample for percentiles (bounded by cap)
	sample []time.Duration
	cap    int

	// window collects raw latencies since last window snapshot
	window []time.Duration
}

// NewLatencyRecorder creates a recorder with a bounded reservoir capacity for
// cumulative percentile calculations. A capacity of 0 disables sampling (no
// percentiles for cumulative stats), but averages still work.
func NewLatencyRecorder(capacity int) *LatencyRecorder {
	if capacity < 0 {
		capacity = 0
	}

	return &LatencyRecorder{cap: capacity, sample: make([]time.Duration, 0, min(capacity, 1024))}
}

// Record adds a single latency measurement.
func (l *LatencyRecorder) Record(d time.Duration) {
	l.mu.Lock()
	l.totalCount++
	l.totalSum += d

	// reservoir sampling: fill up to cap, then replace randomly
	if l.cap > 0 {
		if len(l.sample) < l.cap {
			l.sample = append(l.sample, d)
		} else {
			// simple reservoir: random index in [0, totalCount)
			// replace if index within sample range
			idx := int(time.Now().UnixNano() % l.totalCount)
			if idx < l.cap {
				l.sample[idx] = d
			}
		}
	}

	// always store in current window
	l.window = append(l.window, d)
	l.mu.Unlock()
}

// TotalSnapshot returns cumulative latency statistics for the entire run so far.
func (l *LatencyRecorder) TotalSnapshot() LatencyStats {
	l.mu.Lock()
	defer l.mu.Unlock()

	var avg time.Duration
	if l.totalCount > 0 {
		avg = time.Duration(int64(l.totalSum) / l.totalCount)
	}

	// compute percentiles from sample (best-effort)
	var p50, p95, p99 time.Duration
	if n := len(l.sample); n > 0 {
		tmp := make([]time.Duration, n)
		copy(tmp, l.sample)
		sort.Slice(tmp, func(i, j int) bool { return tmp[i] < tmp[j] })
		p50 = percentile(tmp, 0.50)
		p95 = percentile(tmp, 0.95)
		p99 = percentile(tmp, 0.99)
	}

	return LatencyStats{Count: l.totalCount, Avg: avg, P50: p50, P95: p95, P99: p99}
}

// WindowSnapshotAndReset returns latency stats for the current reporting window
// and resets the window. Percentiles are exact for the window since it uses the
// full data collected for that interval.
func (l *LatencyRecorder) WindowSnapshotAndReset() LatencyStats {
	l.mu.Lock()
	defer l.mu.Unlock()

	n := len(l.window)
	if n == 0 {
		return LatencyStats{}
	}

	tmp := make([]time.Duration, n)
	copy(tmp, l.window)
	l.window = l.window[:0]

	// compute aggregates
	var sum time.Duration
	for _, d := range tmp {
		sum += d
	}

	avg := time.Duration(int64(sum) / int64(n))
	sort.Slice(tmp, func(i, j int) bool { return tmp[i] < tmp[j] })
	p50 := percentile(tmp, 0.50)
	p95 := percentile(tmp, 0.95)
	p99 := percentile(tmp, 0.99)

	return LatencyStats{Count: int64(n), Avg: avg, P50: p50, P95: p95, P99: p99}
}

func percentile(sorted []time.Duration, p float64) time.Duration {
	if len(sorted) == 0 {
		return 0
	}

	if p <= 0 {
		return sorted[0]
	}

	if p >= 1 {
		return sorted[len(sorted)-1]
	}

	// nearest-rank method
	rank := int(float64(len(sorted)) * p)
	if rank <= 0 {
		return sorted[0]
	}

	if rank >= len(sorted) {
		return sorted[len(sorted)-1]
	}

	return sorted[rank]
}
