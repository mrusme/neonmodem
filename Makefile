.PHONY: build install-deps install-deps-go
VERSION := $(shell git describe --tags 2> /dev/null || git rev-parse --short HEAD)

all: install-deps build

build:
	go build -ldflags "-X github.com/mrusme/neonmodem/config.VERSION=$(VERSION)"

install-deps: install-deps-go

install-deps-go:
	go get

