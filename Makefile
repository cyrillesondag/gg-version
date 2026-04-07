VERSION := $(shell git describe --tags --always --dirty 2>/dev/null || echo dev)

.PHONY: build test

build:
	go build -ldflags "-X main.Version=$(VERSION)" -o gg-version ./cmd/gg-version

test:
	/usr/local/go/bin/go test ./...
