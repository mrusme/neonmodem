.PHONY: all build install-deps install-deps-go test vet vet-cross fmt fmt-check lint vuln probes check
VERSION := $(shell git describe --tags 2> /dev/null || git rev-parse --short HEAD)

all: install-deps build

build:
	go build -ldflags "-X github.com/mrusme/neonmodem/internal/config.VERSION=$(VERSION)"

install-deps: install-deps-go

install-deps-go:
	go get

test:
	go test ./... -count=1

vet:
	go vet ./...

vet-cross:
	GOOS=windows go vet ./...
	GOOS=darwin go vet ./...

fmt:
	gofmt -l -w .

fmt-check:
	@test -z "$$(gofmt -l .)" || (gofmt -l . && exit 1)

lint:
	golangci-lint run ./...

vuln:
	govulncheck ./...

probes:
	@if [ -d _internal/probe ]; then go vet ./_internal/probe/...; fi

check: fmt-check vet vet-cross test probes
	@if command -v golangci-lint > /dev/null 2>&1; then $(MAKE) lint; else echo "golangci-lint is not on the path, lint skipped"; fi
