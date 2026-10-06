VERSION ?= 0.3.0-dev

.PHONY: build test release

build:
	go build -trimpath -ldflags "-X github.com/leadfact/format-fast/internal/cli.Version=$(VERSION)" -o bin/formatfast ./cmd/formatfast

test:
	go test -race ./...

# Usage: make release VERSION=0.3.0 (produces archives; never publishes).
release:
	go run ./tools/release -version "$(VERSION)"
