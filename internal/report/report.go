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

			// Deltas for letzte Periode
			deltaAtt := att - lastAtt
			deltaSuc := suc - lastSuc
			deltaFal := fal - lastFal

			dur := t.Sub(lastAt).Seconds()
			rps := 0.0           // Erfolgs-RPS (wie bisher)
			arps := 0.0          // Attempts-RPS (alle Versuche/sek)
			successRate := 0.0   // Gesamterfolg in % seit Start
			intervalSRate := 0.0 // Erfolg % in der letzten Periode

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

			fmt.Printf("[stats] elapsed=%v attempts=%d success=%d fail=%d rps=%.2f arps=%.2f srate=%.2f%% israte=%.2f%% ds=%d df=%d\n",
				time.Since(r.m.Start).Truncate(time.Second), att, suc, fal, rps, arps, successRate, intervalSRate, deltaSuc, deltaFal)

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
}
