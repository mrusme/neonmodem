.PHONY: all build install-deps install-deps-go test vet fmt fmt-check lint check
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

fmt:
	gofmt -l -w .

fmt-check:
	@test -z "$$(gofmt -l .)" || (gofmt -l . && exit 1)

lint:
	golangci-lint run ./...

check: fmt-check vet test
