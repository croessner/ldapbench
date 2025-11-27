Project development guidelines for ldapbench

This document captures project-specific knowledge to help advanced contributors build, configure, test, and extend ldapbench efficiently.

1. Build and configuration

- Language/tooling
  - Go module: github.com/croessner/ldapbench
  - go.mod declares go 1.25 and vendors dependencies under vendor/. Use a recent Go toolchain; builds work offline due to vendoring.
- Build
  - CLI target: cmd/ldapbench
  - Build command: go build ./cmd/ldapbench
  - Run with defaults (non-functional without LDAP): ./ldapbench --check --help
- Core flags (from internal/config)
  - --ldap-url: ldap://host:389 or ldaps://host:636
  - --starttls: enable STARTTLS on ldap:// connections
  - --insecure-skip-verify: skip TLS verification (only for test rigs)
  - --lookup-bind-dn / --lookup-bind-pass: service account for lookups
  - --base-dn: base DN for user searches (required)
  - --uid-attribute: attribute for username→entry mapping (default: uid)
  - --csv: CSV path with header username,password[,expected_ok]
  - --mode: auth|search|both (default: auth)
  - --filter: LDAP filter used in search mode; "%s" is replaced with the username when present (e.g., (&(objectClass=person)(uid=%s)))
  - Workload: --concurrency, --connections, --duration, --rate (RPS), --timeout
  - Reporting: --stats-interval
  - Failure logging: --fail-log path (CSV), --fail-batch batch size
  - Validation-only: --check (runs a short end‑to‑end verification and exits)
- CSV input format (internal/csvdata)
  - Required headers: username,password
  - Optional column: expected_ok (true marks rows included when the column exists; non-true rows are skipped). Additional columns are ignored.
  - Password values have trailing CR/LF trimmed to avoid line-ending artifacts.
- Connectivity validation flow (main + internal/check)
  - --check loads CSV, connects as lookup DN, resolves DN for first user, then executes bind and/or search based on --mode.
  - Use this upfront to catch config mistakes before running benchmarks that could skew metrics.

2. Testing

- Running tests
  - Full suite: go test ./...
  - With race detector: go test -race ./...
  - With coverage: go test -cover ./...
  - Subpackages are standard Go packages under internal/ and cmd/; test packages follow Go conventions and require no special harness.
- Mocks and seams
  - LDAP connectivity is abstracted behind internal/ldapclient.Client (interface). Production implementation lives in internal/ldapclient.
  - internal/check uses a package-level variable newClient that points to ldapclient.New; tests can override newClient to inject fakes without changing public APIs.
  - Runner interacts with ldapclient.Client; tests can supply a fake that implements LookupDN, UserBind, UserSearch, Close to control outcomes deterministically.
- Concurrency/time in tests
  - Runner uses context.WithTimeout(ctx, cfg.Duration). Keep cfg.Duration low (e.g., 10–50 ms) in tests to bound runtime.
  - Rate limiting uses a time.Ticker when --rate > 0; set rate=0 in tests unless you are explicitly verifying pacing.
- Metrics and reporting
  - Metrics use sync/atomic counters. Prefer reading via m.Snapshot() for assertions.
- Temporary example test (verified during guideline creation)
  - We validated the testing flow by adding a transient test under internal/metrics that exercised Metrics.Snapshot. It was executed with go test ./... and then removed to keep the repository unchanged.
- Adding new tests
  - Favor table-driven tests and explicit fakes over global state. Example sketch for a fake client used by Runner tests:
    type fakeClient struct{ dn string; bindErr, searchErr error }
    func (f *fakeClient) LookupDN(u string) (string, error) { return f.dn, nil }
    func (f *fakeClient) UserBind(dn, pw string) error { return f.bindErr }
    func (f *fakeClient) UserSearch(dn, pw, filter string) (int, error) { return 1, f.searchErr }
    func (f *fakeClient) BindLookup() error { return nil }
    func (f *fakeClient) Close() {}
  - Then pass it to runner.New alongside a small csvdata.Users and a metrics.Metrics.

3. Additional development notes

- Concurrency model (internal/runner)
  - Global context with timeout equals cfg.Duration; workers loop until context is done. Optional global rate limiter uses a single ticker; workers select on its ticks.
  - On each iteration: Attempts++, resolve DN via lookup client, then execute bind/search per mode. Failures/successes increment atomic counters; optional failure records are batched to CSV via internal/fail.
  - prepareFilter substitutes %s with the username if present; otherwise uses the provided filter verbatim.
- Metrics (internal/metrics)
  - Atomic counters across workers; Start time captured on New(). Snapshot() returns counts and elapsed for external reporters (internal/report) to compute rates.
- Failure logging (internal/fail)
  - When --fail-log is set, failed operations are appended as CSV records. Use a path on a fast filesystem to avoid I/O bottlenecks; batching is controlled by --fail-batch.
- TLS and security (internal/config -> TLSConfig)
  - TLSConfig honors InsecureSkipVerify; avoid using it outside controlled test setups.
- Coding style
  - Follow standard Go formatting (gofmt), static checks (go vet), and prefer returning wrapped errors with context (fmt.Errorf("...: %w", err)).
  - Keep package boundaries clean: config parsing in internal/config; network I/O in internal/ldapclient; orchestration in internal/runner; no cross-traffic of concerns.
- Observability
  - The reporter prints periodic stats at --stats-interval and a final summary. For automated environments, capture stdout/stderr and parse the final summary only to avoid noise from periodic reports.

4. Practical quickstart

1) Prepare a minimal CSV users.csv:
   username,password
   alice,secret

2) Validate configuration against your LDAP:
   ./ldapbench \
     --ldap-url ldaps://ldap.example.com:636 \
     --base-dn "dc=example,dc=com" \
     --lookup-bind-dn "cn=svc,ou=system,dc=example,dc=com" \
     --lookup-bind-pass "…" \
     --csv users.csv \
     --mode auth \
     --check

3) Run a short benchmark:
   ./ldapbench \
     --ldap-url ldaps://ldap.example.com:636 \
     --base-dn "dc=example,dc=com" \
     --lookup-bind-dn "cn=svc,ou=system,dc=example,dc=com" \
     --lookup-bind-pass "…" \
     --csv users.csv \
     --mode both \
     --concurrency 32 \
     --duration 30s \
     --stats-interval 10s

5. Reproducibility and CI notes

- Vendored dependencies allow network-less builds. Prefer go build -mod=vendor and go test -mod=vendor in isolated environments.
- When adding new external dependencies, either vendor them or update CI to allow module downloads.
- Keep tests hermetic: do not rely on live LDAP. Use fakes/mocks at the interface boundaries.
