# Check to see if we can use ash, in Alpine images, or default to BASH.
# On Windows/MSYS2, derive bash.exe from the default sh.exe path.
# On Unix, uses `which` to find bash for environments like NixOS where
# bash lives in the Nix store rather than /bin/bash.
ifeq ($(OS),Windows_NT)
    SHELL := $(subst sh.exe,bash.exe,$(SHELL))
else
    SHELL := $(if $(wildcard /bin/ash),/bin/ash,$(shell which bash 2>/dev/null || echo /bin/sh))
endif

# ==============================================================================
# Install

install-gotooling:
	go install honnef.co/go/tools/cmd/staticcheck@latest
	go install golang.org/x/vuln/cmd/govulncheck@latest

# ==============================================================================
# Go Modules support

tidy:
	go mod tidy

deps-upgrade: bui-upgrade
	go get -u -v ./...
	go mod tidy

# ==============================================================================
# Tests

lint:
	go vet ./...
	staticcheck -checks=all ./...

vuln-check:
	govulncheck ./...

diff:
	go fix -diff ./...

test-only:
	@echo ========== RUN TESTS ==========
	go test -v -count=1 ./...

test: test-only lint vuln-check diff