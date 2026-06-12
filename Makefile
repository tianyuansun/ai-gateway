# Version info — injected via ldflags at build time.
VERSION ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo "dev")
COMMIT  ?= $(shell git rev-parse --short HEAD 2>/dev/null || echo "unknown")
DATE    ?= $(shell date -u +"%Y-%m-%dT%H:%M:%SZ")

LDFLAGS := -s -w \
	-X 'main.version=$(VERSION)' \
	-X 'main.commit=$(COMMIT)' \
	-X 'main.date=$(DATE)'

BIN_DIR := bin
BINARY  := $(BIN_DIR)/gateway

.PHONY: build test test-e2e lint run clean

build:
	go build -ldflags="$(LDFLAGS)" -o $(BINARY) ./cmd/gateway

test:
	go test -count=1 ./...

test-e2e:
	go test -count=1 -tags e2e ./...

lint:
	golangci-lint run

run: build
	@test -f .env && set -a && . ./.env && set +a || true
	$(BINARY) -config config/gateway.local.yaml

clean:
	rm -rf $(BIN_DIR)
