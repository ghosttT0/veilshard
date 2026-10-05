BINARY_NAME=veilshard
ALIAS_NAME=vpnctl
VERSION=v0.2.0
BUILD_DIR=bin

.PHONY: all build build-linux docker-build clean test

all: build-linux

# Build for current OS
build:
	go build -ldflags="-s -w -X 'github.com/veilshard/veilshard/internal/cli.Version=$(VERSION)'" -o $(BUILD_DIR)/$(BINARY_NAME) ./cmd/veilshard
	go build -ldflags="-s -w -X 'github.com/veilshard/veilshard/internal/cli.Version=$(VERSION)'" -o $(BUILD_DIR)/$(ALIAS_NAME) ./cmd/vpnctl

# Cross-compile static binary for target Ubuntu (linux/amd64)
build-linux:
	CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build \
		-trimpath \
		-ldflags="-s -w -X 'github.com/veilshard/veilshard/internal/cli.Version=$(VERSION)'" \
		-o $(BUILD_DIR)/$(BINARY_NAME)-linux-amd64 \
		./cmd/veilshard
	cp $(BUILD_DIR)/$(BINARY_NAME)-linux-amd64 $(BUILD_DIR)/$(ALIAS_NAME)-linux-amd64

# One-liner docker build (extracts static binary without installing Go locally)
docker-build:
	docker run --rm -v "$$(pwd):/src" -w /src golang:1.22-alpine \
		sh -c "CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -trimpath -ldflags='-s -w' -o $(BINARY_NAME) ./cmd/veilshard && cp $(BINARY_NAME) $(ALIAS_NAME)"

# Build nexus proxy core
nexus:
	go build -ldflags="-s -w" -o $(BUILD_DIR)/nexus ./cmd/nexus

nexus-linux:
	CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -trimpath -ldflags="-s -w" -o $(BUILD_DIR)/nexus-linux-amd64 ./cmd/nexus

test:
	go test -v ./internal/...

clean:
	rm -rf $(BUILD_DIR) $(BINARY_NAME) nexus
