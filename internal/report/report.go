package report

// Package report prints periodic statistics and a final summary for the run.

import (
	"context"
	"fmt"
	"io"
	"sync/atomic"
	"time"

	"github.com/croessner/ldapbench/internal/metrics"
)

// Reporter periodically prints stats to stdout.
type Reporter struct {
	m       *metrics.Metrics
	intv    time.Duration
	stopped atomic.Bool
}

// New creates a new Reporter instance.
func New(m *metrics.Metrics, intv time.Duration) *Reporter { return &Reporter{m: m, intv: intv} }

// Run starts the periodic reporting loop until the context is canceled.
func (r *Reporter) Run(ctx context.Context) {
	ticker := time.NewTicker(r.intv)
	defer ticker.Stop()

	var lastAtt int64
	var lastSuc int64
	var lastFal int64
	var lastAt = time.Now()

	for {
		select {
		case <-ctx.Done():
			return
		case t := <-ticker.C:
			att := r.m.Attempts.Load()
			suc := r.m.Success.Load()
			fal := r.m.Fail.Load()

			// Deltas for the last period
			deltaAtt := att - lastAtt
			deltaSuc := suc - lastSuc
			deltaFal := fal - lastFal

			dur := t.Sub(lastAt).Seconds()
			rps := 0.0           // Success RPS (successful requests per second in last period)
			arps := 0.0          // Attempts RPS (all attempts per second in last period)
			successRate := 0.0   // Overall success rate in % since start
			intervalSRate := 0.0 // Success rate % in the last period

			if dur > 0 {
				rps = float64(deltaSuc) / dur
				arps = float64(deltaAtt) / dur
			}

			if att > 0 {
				successRate = (float64(suc) / float64(att)) * 100
			}

			if deltaAtt > 0 {
				intervalSRate = (float64(deltaSuc) / float64(deltaAtt)) * 100
			}

			// window latency stats since last tick
			wlat := r.m.Lat.WindowSnapshotAndReset()
			fmt.Printf("[stats] elapsed=%v attempts=%d success=%d fail=%d rps=%.2f arps=%.2f srate=%.2f%% israte=%.2f%% ds=%d df=%d avg=%.2f p50=%.2f p95=%.2f p99=%.2f wcnt=%d\n",
				time.Since(r.m.Start).Truncate(time.Second), att, suc, fal, rps, arps, successRate, intervalSRate, deltaSuc, deltaFal,
				float64(wlat.Avg.Microseconds())/1000.0, float64(wlat.P50.Microseconds())/1000.0, float64(wlat.P95.Microseconds())/1000.0, float64(wlat.P99.Microseconds())/1000.0, wlat.Count)

			lastAtt = att
			lastSuc = suc
			lastFal = fal
			lastAt = t
		}
	}
}

// Stop marks the reporter stopped (placeholder for future use).
func (r *Reporter) Stop() { r.stopped.Store(true) }

// PrintSummary writes the final summary to the given writer.
func PrintSummary(w io.Writer, m *metrics.Metrics, elapsed time.Duration) {
	att := m.Attempts.Load()
	suc := m.Success.Load()
	fal := m.Fail.Load()

	var rps float64
	if elapsed > 0 {
		rps = float64(suc) / elapsed.Seconds()
	}

	fmt.Fprintf(w, "\n==== Summary ====\n")
	fmt.Fprintf(w, "elapsed: %v\n", elapsed.Truncate(time.Millisecond))
	fmt.Fprintf(w, "attempts: %d\n", att)
	fmt.Fprintf(w, "success: %d\n", suc)
	fmt.Fprintf(w, "fail: %d\n", fal)
	fmt.Fprintf(w, "avg rps (success): %.2f\n", rps)

	// Total latency stats for the whole run
	tlat := m.Lat.TotalSnapshot()
	if tlat.Count > 0 {
		fmt.Fprintf(w, "latency (overall): count=%d avg_ms=%.2f p50_ms=%.2f p95_ms=%.2f p99_ms=%.2f\n",
			tlat.Count,
			float64(tlat.Avg.Microseconds())/1000.0,
			float64(tlat.P50.Microseconds())/1000.0,
			float64(tlat.P95.Microseconds())/1000.0,
			float64(tlat.P99.Microseconds())/1000.0,
		)
	}
}
