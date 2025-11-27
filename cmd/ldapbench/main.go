package main

// Entry point for ldapbench CLI. Parses configuration, initializes LDAP client,
// loads CSV users, starts reporter and runs the benchmark runner until the
// configured duration elapses or a termination signal is received.

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/croessner/ldapbench/internal/check"
	"github.com/croessner/ldapbench/internal/config"
	"github.com/croessner/ldapbench/internal/csvdata"
	"github.com/croessner/ldapbench/internal/fail"
	"github.com/croessner/ldapbench/internal/ldapclient"
	"github.com/croessner/ldapbench/internal/metrics"
	"github.com/croessner/ldapbench/internal/report"
	"github.com/croessner/ldapbench/internal/runner"
)

func main() {
	cfg, err := config.Parse()
	if err != nil {
		fmt.Fprintf(os.Stderr, "config error: %v\n", err)
		os.Exit(2)
	}

	// Check-only mode: run quick validations/tests and exit.
	if cfg.CheckOnly {
		if err := check.Run(cfg); err != nil {
			fmt.Fprintf(os.Stderr, "ERROR: %v\n", err)
			os.Exit(2)
		}
		fmt.Println("check: OK")
		os.Exit(0)
	}

	users, err := csvdata.Load(cfg.CSVPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "csv error: %v\n", err)
		os.Exit(2)
	}

	if len(users.All) == 0 {
		fmt.Fprintf(os.Stderr, "csv error: no users found in %s\n", cfg.CSVPath)
		os.Exit(2)
	}

	client, err := ldapclient.New(cfg)
	if err != nil {
		fmt.Fprintf(os.Stderr, "ldap client error: %v\n", err)
		os.Exit(2)
	}

	defer client.Close()

	// Validate lookup bind works upfront so benchmark isn't skewed by initial failures.
	if err := client.BindLookup(); err != nil {
		fmt.Fprintf(os.Stderr, "lookup bind failed: %v\n", err)
		os.Exit(2)
	}

	m := metrics.New()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Handle OS termination signals for graceful shutdown.
	sigCh := make(chan os.Signal, 2)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
	go func() {
		<-sigCh
		cancel()
	}()

	reporter := report.New(m, cfg.StatsInterval)
	go reporter.Run(ctx)

	// Optional failure logger
	var flog *fail.Logger
	if cfg.FailLogPath != "" {
		flog = fail.New(cfg.FailLogPath, cfg.FailLogBatch)
		defer flog.Close()
	}

	r := runner.New(cfg, client, users, m, flog)
	start := time.Now()
	err = r.Run(ctx)
	elapsed := time.Since(start)

	reporter.Stop()

	report.PrintSummary(os.Stdout, m, elapsed)
	if err != nil {
		fmt.Fprintf(os.Stderr, "run error: %v\n", err)
		os.Exit(1)
	}
}
