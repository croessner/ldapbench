# Simple Makefile for ldapbench
# Builds with vendored dependencies and supports install/uninstall.

## Variables (override via environment, e.g., `make PREFIX=/opt`)
GO       ?= go
GOFLAGS  ?= -mod=vendor
CMD      ?= ./cmd/ldapbench
BINARY   ?= ldapbench

PREFIX   ?= /usr/local
DESTDIR  ?=
BINDIR    = $(DESTDIR)$(PREFIX)/bin

.PHONY: all build install uninstall clean test

all: build

build:
	$(GO) build $(GOFLAGS) -o $(BINARY) $(CMD)

install: build
	install -d "$(BINDIR)"
	install -m 0755 "$(BINARY)" "$(BINDIR)/$(BINARY)"

uninstall:
	rm -f "$(BINDIR)/$(BINARY)"

clean:
	rm -f "$(BINARY)"

test:
	$(GO) test $(GOFLAGS) ./...
