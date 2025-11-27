package config

// Package config provides CLI parsing and runtime configuration for the
// ldapbench tool.

import (
	"crypto/tls"
	"errors"
	"time"

	"github.com/spf13/pflag"
)

// Mode selects which scenario to execute.
type Mode string

const (
	ModeAuth   Mode = "auth"
	ModeSearch Mode = "search"
	ModeBoth   Mode = "both"
)

// Config holds all runtime settings parsed from CLI flags.
type Config struct {
	LDAPURL            string
	StartTLS           bool
	InsecureSkipVerify bool

	LookupBindDN   string
	LookupBindPass string
	BaseDN         string
	UIDAttr        string

	CSVPath string
	Mode    Mode
	Filter  string

	Concurrency   int
	Connections   int
	Duration      time.Duration
	Rate          float64 // target requests per second; 0 = unlimited
	StatsInterval time.Duration
	Timeout       time.Duration // per-request timeout

	// Optional failure logging
	FailLogPath  string // path to write failed attempts (CSV). Empty disables.
	FailLogBatch int    // how many records to buffer before writing

	// CheckOnly, when true, runs a quick configuration/connectivity check and exits.
	CheckOnly bool
}

// Parse reads CLI flags into a Config instance and validates essential fields.
func Parse() (*Config, error) {
	var cfg Config
	pflag.StringVar(&cfg.LDAPURL, "ldap-url", "ldap://localhost:389", "LDAP URL, e.g. ldap://host:389, ldaps://host:636, or ldapi:// (Unix domain socket)")
	pflag.BoolVar(&cfg.StartTLS, "starttls", false, "Use STARTTLS on ldap:// connections")
	pflag.BoolVar(&cfg.InsecureSkipVerify, "insecure-skip-verify", false, "Skip TLS certificate verification (unsafe, test only)")
	pflag.StringVar(&cfg.LookupBindDN, "lookup-bind-dn", "", "Lookup service account bind DN")
	pflag.StringVar(&cfg.LookupBindPass, "lookup-bind-pass", "", "Lookup service account password")
	pflag.StringVar(&cfg.BaseDN, "base-dn", "", "Base DN for user searches")
	pflag.StringVar(&cfg.UIDAttr, "uid-attribute", "uid", "Attribute used to map username to DN (e.g., uid, sAMAccountName)")
	pflag.StringVar(&cfg.CSVPath, "csv", "users.csv", "CSV file path with username,password header")
	var mode string
	pflag.StringVar(&mode, "mode", string(ModeAuth), "Benchmark mode: auth|search|both")
	pflag.StringVar(&cfg.Filter, "filter", "(objectClass=person)", "LDAP filter for search mode; use %s as username placeholder when desired")
	pflag.IntVar(&cfg.Concurrency, "concurrency", 32, "Number of concurrent workers")
	pflag.IntVar(&cfg.Connections, "connections", 1, "Connections per worker (>=1)")
	pflag.DurationVar(&cfg.Duration, "duration", time.Minute, "Total benchmark duration")
	pflag.Float64Var(&cfg.Rate, "rate", 0, "Target requests per second (0 = unlimited)")
	pflag.DurationVar(&cfg.StatsInterval, "stats-interval", time.Minute, "Statistics print interval")
	pflag.DurationVar(&cfg.Timeout, "timeout", 5*time.Second, "Per-request timeout")
	pflag.StringVar(&cfg.FailLogPath, "fail-log", "", "Optional path to write failed attempts as CSV (disabled when empty)")
	pflag.IntVar(&cfg.FailLogBatch, "fail-batch", 256, "Batch size for failure log writes")
	pflag.BoolVar(&cfg.CheckOnly, "check", false, "Only check configuration/connectivity and exit")
	pflag.Parse()

	switch Mode(mode) {
	case ModeAuth, ModeSearch, ModeBoth:
		cfg.Mode = Mode(mode)
	default:
		return nil, errors.New("invalid mode: must be auth, search, or both")
	}

	if cfg.BaseDN == "" {
		return nil, errors.New("base-dn is required")
	}

	if cfg.LookupBindDN == "" || cfg.LookupBindPass == "" {
		return nil, errors.New("lookup-bind-dn and lookup-bind-pass are required")
	}

	if cfg.Concurrency <= 0 || cfg.Connections <= 0 {
		return nil, errors.New("concurrency and connections must be >= 1")
	}

	return &cfg, nil
}

// TLSConfig returns a TLS config honoring the InsecureSkipVerify flag.
func (c *Config) TLSConfig() *tls.Config {
	return &tls.Config{InsecureSkipVerify: c.InsecureSkipVerify}
}
