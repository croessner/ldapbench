package runner

// Package runner orchestrates the benchmark execution: it spins up workers,
// applies optional global rate limiting, and records metrics for each attempt.

import (
	"context"
	"fmt"
	"math/rand"
	"strings"
	"sync"
	"time"

	"github.com/croessner/ldapbench/internal/config"
	"github.com/croessner/ldapbench/internal/csvdata"
	"github.com/croessner/ldapbench/internal/fail"
	"github.com/croessner/ldapbench/internal/ldapclient"
	"github.com/croessner/ldapbench/internal/metrics"
)

// Runner holds the components required to execute a scenario.
type Runner struct {
	cfg    *config.Config
	client ldapclient.Client
	users  *csvdata.Users
	m      *metrics.Metrics
	flog   *fail.Logger
}

// New constructs a Runner.
func New(cfg *config.Config, client ldapclient.Client, users *csvdata.Users, m *metrics.Metrics, flog *fail.Logger) *Runner {
	return &Runner{cfg: cfg, client: client, users: users, m: m, flog: flog}
}

// Run executes until the configured duration elapses or the context is canceled.
func (r *Runner) Run(ctx context.Context) error {
	ctx, cancel := context.WithTimeout(ctx, r.cfg.Duration)
	defer cancel()

	wg := &sync.WaitGroup{}
	wg.Add(r.cfg.Concurrency)

	// Optional simple global rate limiter using a ticker.
	var tick <-chan time.Time
	var ticker *time.Ticker

	if r.cfg.Rate > 0 {
		period := time.Duration(float64(time.Second) / r.cfg.Rate)
		if period <= 0 {
			period = time.Nanosecond
		}

		ticker = time.NewTicker(period)
		tick = ticker.C

		defer ticker.Stop()
	}

	for i := 0; i < r.cfg.Concurrency; i++ {
		go func() {
			defer wg.Done()
			for {
				select {
				case <-ctx.Done():
					return
				default:
				}

				if tick != nil {
					select {
					case <-ctx.Done():
						return
					case <-tick:
					}
				}

				r.runOnce()
			}
		}()
	}

	wg.Wait()

	// return context error so caller can distinguish normal timeout
	return ctx.Err()
}

// runOnce performs a single attempt depending on the configured mode.
func (r *Runner) runOnce() {
	r.m.Attempts.Add(1)
	user := r.users.All[rand.Intn(len(r.users.All))]

	// Lookup DN using the service account
	dn, err := r.client.LookupDN(user.Username)
	if err != nil {
		r.m.Fail.Add(1)
		if r.flog != nil {
			r.flog.Log(fail.Record{Timestamp: time.Now(), Operation: "lookup", Username: user.Username, DN: "", Filter: "", Error: err.Error()})
		}

		return
	}

	switch r.cfg.Mode {
	case config.ModeAuth:
		if err := r.client.UserBind(dn, user.Password); err != nil {
			r.m.Fail.Add(1)
			if r.flog != nil {
				r.flog.Log(fail.Record{Timestamp: time.Now(), Operation: "bind", Username: user.Username, DN: dn, Filter: "", Error: err.Error()})
			}

			return
		}

		r.m.Success.Add(1)
	case config.ModeSearch:
		filter := r.prepareFilter(user.Username)
		if _, err := r.client.UserSearch(dn, user.Password, filter); err != nil {
			r.m.Fail.Add(1)
			if r.flog != nil {
				r.flog.Log(fail.Record{Timestamp: time.Now(), Operation: "search", Username: user.Username, DN: dn, Filter: filter, Error: err.Error()})
			}

			return
		}

		r.m.Success.Add(1)
	case config.ModeBoth:
		if err := r.client.UserBind(dn, user.Password); err != nil {
			r.m.Fail.Add(1)
			if r.flog != nil {
				r.flog.Log(fail.Record{Timestamp: time.Now(), Operation: "bind", Username: user.Username, DN: dn, Filter: "", Error: err.Error()})
			}

			return
		}

		filter := r.prepareFilter(user.Username)
		if _, err := r.client.UserSearch(dn, user.Password, filter); err != nil {
			r.m.Fail.Add(1)
			if r.flog != nil {
				r.flog.Log(fail.Record{Timestamp: time.Now(), Operation: "search", Username: user.Username, DN: dn, Filter: filter, Error: err.Error()})
			}

			return
		}

		r.m.Success.Add(1)
	default:
		// Should not happen due to validation
		r.m.Fail.Add(1)

		fmt.Println("unknown mode")
	}
}

// prepareFilter injects the username into the filter if %s placeholder exists.
func (r *Runner) prepareFilter(username string) string {
	f := r.cfg.Filter
	if strings.Contains(f, "%s") {
		return fmt.Sprintf(f, username)
	}

	return f
}
