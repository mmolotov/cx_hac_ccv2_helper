BINARY        := bin/hac-helper
PKG           := github.com/mmolotov/cx_hac_ccv2_helper
VERSION       ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo dev)
LDFLAGS       := -s -w -X $(PKG)/cmd.version=$(VERSION)
GOLANGCI_VER  := v2.1.6
GOLANGCI_LINT := $(shell command -v golangci-lint 2>/dev/null || echo go run github.com/golangci/golangci-lint/v2/cmd/golangci-lint@$(GOLANGCI_VER))

.PHONY: all build test race cover lint fmt vet check completion clean

all: check build

build:
	go build -ldflags "$(LDFLAGS)" -o $(BINARY) .

test:
	go test ./...

race:
	go test -race ./...

cover:
	go test -coverprofile=coverage.out ./...
	go tool cover -func=coverage.out | tail -1

vet:
	go vet ./...

fmt:
	gofmt -w .

lint:
	$(GOLANGCI_LINT) run ./...

check: vet lint test

completion: build
	$(BINARY) completion zsh > completion.sh

clean:
	rm -rf bin coverage.out
