# INSTALLATION GUIDE

This document describes how to build, install, and verify ldapbench on your system. All steps work offline thanks to vendored dependencies.


## System requirements

- Supported OS: Linux, macOS, and other Unix-like platforms supported by Go
- Go toolchain: recent Go version capable of building modules; the module declares `go 1.25`
- Network: not required for builds/tests (dependencies are vendored)
- Optional: make(1) for convenient build/install targets


## Source layout

- CLI entry point: cmd/ldapbench
- Go module: github.com/croessner/ldapbench
- Vendored deps: vendor/
- Makefile with common targets: build, install, uninstall, clean, test


## Quick install (default prefix /usr/local)

1) Build and install using the Makefile:

    make
    sudo make install

This produces `ldapbench` and installs it to `/usr/local/bin/ldapbench` by default.

To change the prefix (e.g., /opt) or staging root (DESTDIR) use:

    make PREFIX=/opt
    make PREFIX=/opt DESTDIR=/tmp/pkgroot install


## Build from source without make

You can invoke the Go toolchain directly. The repository vendors dependencies so offline builds work.

Build the CLI binary in the repository root:

    go build -mod=vendor -o ldapbench ./cmd/ldapbench

Then place it on your PATH, for example:

    install -m 0755 ldapbench /usr/local/bin/ldapbench


## Uninstall

If installed via `make install` with default PREFIX:

    sudo make uninstall

If you installed manually, remove the binary you installed, e.g. `/usr/local/bin/ldapbench`.


## Verifying your install

Run the built binary to print help and confirm it starts:

    ldapbench --help

To validate configuration and connectivity before running benchmarks, use `--check` with a small CSV file:

    echo "username,password" > users.csv
    echo "alice,secret" >> users.csv
    ldapbench \
      --ldap-url ldaps://ldap.example.com:636 \
      --base-dn "dc=example,dc=com" \
      --lookup-bind-dn "cn=svc,ou=system,dc=example,dc=com" \
      --lookup-bind-pass "…" \
      --csv users.csv \
      --mode auth \
      --check


## Troubleshooting

- Command not found after install
  - Ensure your PREFIX/bin is on PATH. Default is /usr/local/bin.
- TLS/certificate errors
  - Provide a valid CA chain on your system or use `--insecure-skip-verify` in controlled test setups only.
- Cannot connect to LDAP server
  - Verify `--ldap-url` (ldap:// vs ldaps://), firewall access, and server availability.
- No entries found during search mode
  - Check `--base-dn`, `--uid-attribute`, and `--filter`. If the filter contains `%s`, it will be substituted with the username; otherwise it is used verbatim.
- High failure rates during benchmarks
  - Start with `--check` to validate, lower concurrency, or increase `--timeout`. Consider writing failures to a fast filesystem via `--fail-log` and adjust `--fail-batch`.


## Reproducible builds and CI notes

- Prefer `-mod=vendor` to force using vendored dependencies:

    go build -mod=vendor ./cmd/ldapbench
    go test  -mod=vendor ./...

- Avoid introducing new external dependencies unless you also vendor them or allow module downloads in your CI environment.
