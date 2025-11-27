# ldapbench

A fast, reproducible command-line benchmark and validation tool for LDAP directories.

ldapbench drives authentication (bind) and/or search workloads against an LDAP server and reports throughput and error rates. It is designed for quick configuration checks ("does my config work?") and for short synthetic benchmarks to compare server settings or network conditions.

- Offline builds via vendored dependencies
- Simple CSV input for test users
- Configurable concurrency, connection pool size, duration, and optional global rate limiting
- Modes: auth, search, or both
- STARTTLS, LDAPS, and LDAPI (Unix domain socket) support; optional TLS verification skip for test rigs
- Periodic and final summary reporting; optional failure CSV logging

Project module path: github.com/croessner/ldapbench


## Contents
- Quick start
- Installation
- CSV input format
- Configuration and flags
- SASL/EXTERNAL authentication (optional)
- Workload model
- Output and metrics
- Failure logging
- TLS and security
- Tips for reliable benchmarks
- Development and testing
- License


## Quick start

1) Prepare a minimal CSV file users.csv:

    username,password
    alice,secret

2) Run a configuration check against your LDAP. Replace values for your environment:

    ./ldapbench \
      --ldap-url ldaps://ldap.example.com:636 \
      --base-dn "dc=example,dc=com" \
      --lookup-bind-dn "cn=svc,ou=system,dc=example,dc=com" \
      --lookup-bind-pass "…" \
      --csv users.csv \
      --mode auth \
      --check

3) Run a small benchmark:

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

Use --check first to catch configuration issues before running longer tests.


## Installation

See INSTALL.md for detailed system requirements and install methods.

Short version (with vendored modules):

- Build: `go build -mod=vendor ./cmd/ldapbench` or `make`
- Install to /usr/local/bin: `make install`


## CSV input format

The tool reads users from a CSV file.

Required headers:
- username
- password

Optional column:
- expected_ok — when present, only rows with the textual value "true" are included; non-true rows are skipped. This is useful if your CSV contains negative test cases.

Notes:
- Trailing CR/LF is trimmed from password values to avoid line-ending artifacts.
- Additional, unknown columns are ignored.


## Configuration and flags

Core flags (see internal/config for full list):
- --ldap-url string
  LDAP URL, e.g. ldap://host:389, ldaps://host:636, or ldapi:// (Unix domain socket; path URL-encoded, e.g., ldapi://%2Fvar%2Frun%2Fslapd%2Fldapi)
- --starttls
  Enable STARTTLS when using ldap:// URLs
- --insecure-skip-verify
  Skip TLS certificate verification (use only in controlled test setups)
- --tls-cert / --tls-key
  Optional TLS client certificate and private key (PEM files) for mutual TLS. Required when using SASL/EXTERNAL over TLS.
- --lookup-bind-dn string
  Service account DN used to resolve user DNs for bind/search (optional when --sasl-external is set)
- --lookup-bind-pass string
  Password for the lookup DN (optional when --sasl-external is set)
- --base-dn string
  Base DN for user searches (required)
- --uid-attribute string
  Attribute used to map username to entry (default: uid)
- --csv path
  Path to the CSV input file
- --mode string
  Workload mode: auth | search | both (default: auth)
- --filter string
  LDAP filter used in search mode. If it contains "%s", the username is substituted. Example: (&(objectClass=person)(uid=%s))
- --sasl-external
  Use SASL/EXTERNAL for DN lookup and the search step (mode=search and the search phase of mode=both). Requires ldapi:// or TLS client certificates. User bind for authentication tests remains simple bind with the user's DN/password.
- Workload controls:
  - --concurrency int: number of workers
  - --connections int: number of LDAP connections in the pool
  - --duration duration: total run time, e.g. 30s, 2m
  - --rate int: global requests-per-second limit (0 = unlimited)
  - --timeout duration: per-operation timeout
- Reporting:
  - --stats-interval duration: how often to print interim stats
- Failure logging:
  - --fail-log path: write failed operations to CSV
  - --fail-batch int: batch size for buffered writes
- Validation only:
  - --check: run a short end-to-end verification and exit

Run `./ldapbench --help` for the authoritative list and defaults.


## SASL/EXTERNAL authentication (optional)

ldapbench can authenticate the search step with SASL/EXTERNAL when `--sasl-external` is set. This is useful when the server maps the client identity from:
- LDAPI (Unix domain socket), or
- TLS client certificates (mutual TLS) on LDAPS or LDAP+STARTTLS.

Notes and behavior:
- With `--sasl-external`, the lookup/service connection authenticates via SASL/EXTERNAL. DN resolution (LookupDN) runs under the socket/certificate identity.
- The user search step also uses SASL/EXTERNAL when enabled.
- `--mode=auth` and the user bind phase in `--mode=both` continue to use simple bind with the user's DN/password from the CSV.

Examples:

1) LDAPI with EXTERNAL (lookup credentials omitted):

```
./ldapbench \
  --ldap-url ldapi://%2Fvar%2Frun%2Fslapd%2Fldapi \
  --base-dn "dc=example,dc=com" \
  --csv users.csv \
  --mode search \
  --sasl-external \
  --check
```

2) LDAPS with mutual TLS and EXTERNAL for lookup + search:

```
./ldapbench \
  --ldap-url ldaps://ldap.example.com:636 \
  --tls-cert client.crt --tls-key client.key \
  --base-dn "dc=example,dc=com" \
  --csv users.csv \
  --mode both \
  --sasl-external
```

When using `ldap://` with `--starttls`, the same `--tls-cert/--tls-key` flags apply if your server supports EXTERNAL via TLS client auth.


## Workload model

- A global context is created with timeout equal to --duration. Workers loop until the context is done.
- An optional global rate limiter (ticker) is enabled when --rate > 0; workers select on its ticks before issuing operations.
- On each iteration, a worker:
  1. Increments Attempts
  2. Resolves the user DN via the lookup client
  3. Executes bind and/or search depending on --mode
  4. Updates atomic success/failure counters and optionally records failures

Search filter handling:
- If --filter contains "%s", the placeholder is replaced with the current username
- Otherwise the filter is used verbatim


## Output and metrics

Metrics are maintained via atomic counters and include Attempts, Successes, Failures, and elapsed time. The reporter prints periodic stats at --stats-interval and a final summary with rates derived from the snapshot.

For automated environments, capture stdout/stderr and parse only the final summary to avoid noise from periodic reports.

### Meaning of the periodic [stats] line

Example output:

    [stats] elapsed=2m0s attempts=274516 success=274484 fail=0 rps=2316.08 arps=2316.08 srate=99.99% israte=100.00% ds=138965 df=0

Fields explained:

- elapsed: Total runtime since the benchmark started (here: 2 minutes).
- attempts: Cumulative number of all attempts (bind/search operations) since start.
- success: Cumulative number of successful attempts since start.
- fail: Cumulative number of failed attempts since start.
- rps: Successful requests per second within the last reporting interval only (deltaSuccess / seconds in the last period).
- arps: Attempts per second (all attempts, successful + failed) within the last reporting interval (deltaAttempts / seconds in the last period).
- srate: Overall success rate in percent since start (success / attempts).
- israte: Interval success rate in percent for the last period only (deltaSuccess / deltaAttempts).
- ds: Delta success — number of successful operations in the last period.
- df: Delta fail — number of failed operations in the last period.

Notes:

- Periodic values (rps, arps, israte, ds, df) always refer to the most recent reporting interval (--stats-interval). They show short-term fluctuations.
- Cumulative counters (attempts, success, fail, srate) apply to the entire runtime so far.
- At the end of the run, an additional summary is printed. There, “avg rps (success)” is the average over the whole runtime (success / elapsed), in contrast to rps in the [stats] line, which reflects only the last interval.


## Failure logging

When --fail-log is provided, failed operations are appended as CSV records. To minimize I/O overhead during benchmarks, writes are batched; configure with --fail-batch. Use a path on a fast filesystem.


## TLS and security

- STARTTLS can be enabled with --starttls on ldap:// connections.
- For LDAPS (ldaps://), standard TLS is used.
- --insecure-skip-verify disables certificate verification and should be used only in controlled testing.
 - For LDAPI (ldapi://), connections use a local Unix domain socket; TLS/STARTTLS are not applicable.
 - Mutual TLS: provide `--tls-cert` and `--tls-key` (PEM) to present a client certificate. This is required for SASL/EXTERNAL over TLS.
 - SASL/EXTERNAL: when `--sasl-external` is set, the lookup connection and the search step authenticate via EXTERNAL (requires LDAPI or mutual TLS). For these steps, the server derives identity from the socket or client certificate. User DN/password are still used for simple bind in auth mode and in the bind phase of mode=both.

See internal/config TLSConfig for details.


## Tips for reliable benchmarks

- Use --check first to validate connectivity and credentials.
- Run short duration (e.g., 15–60 seconds) for tuning, then increase.
- Pin client and server to low-latency networks; avoid noisy neighbors on shared infrastructure.
- Keep CSV small and repeat users to stress server caches if that reflects your scenario; or expand CSV to model cold-cache behavior.
- Write failure logs to fast storage and avoid extremely small batch sizes.
- Prefer a single ldaps:// or ldap://+--starttls configuration; avoid mixing during runs.


## Development and testing

- Language/tooling: Go (module github.com/croessner/ldapbench); dependencies are vendored under vendor/ so builds work offline.
- Build: `go build ./cmd/ldapbench` (prefer `-mod=vendor`). Makefile targets: build, install, uninstall, clean, test.
- Testing: `go test ./...` (race: `go test -race ./...`, coverage: `go test -cover ./...`).
- LDAP connectivity is abstracted behind internal/ldapclient.Client. Tests inject fakes by overriding a package-level constructor variable (newClient) in internal/check and by supplying fake implementations to the runner.
- Concurrency: runner uses context.WithTimeout for --duration; set low durations in tests to keep them fast.
- Metrics: use m.Snapshot() in assertions.

Example fake for runner tests:

    type fakeClient struct{ dn string; bindErr, searchErr error }
    func (f *fakeClient) LookupDN(u string) (string, error) { return f.dn, nil }
    func (f *fakeClient) UserBind(dn, pw string) error { return f.bindErr }
    func (f *fakeClient) UserSearch(dn, pw, filter string) (int, error) { return 1, f.searchErr }
    func (f *fakeClient) BindLookup() error { return nil }
    func (f *fakeClient) Close() {}


## License

This project is licensed under the terms of the LICENSE file in this repository.
