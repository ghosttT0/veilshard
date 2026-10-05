# Multi-stage Dockerfile for building statically-linked veilshard Linux binary
FROM golang:1.22-alpine AS builder

WORKDIR /src

# Copy go module definitions
COPY go.mod ./

# Copy source code
COPY cmd/ ./cmd/
COPY internal/ ./internal/

# Build static binary for Linux amd64
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build \
    -trimpath \
    -ldflags="-s -w -X 'github.com/veilshard/veilshard/internal/cli.Version=v0.2.0'" \
    -o /bin/veilshard \
    ./cmd/veilshard

# Minimal scratch or alpine artifact image
FROM alpine:3.20

RUN apk add --no-cache ca-certificates

COPY --from=builder /bin/veilshard /usr/local/bin/veilshard
RUN ln -s /usr/local/bin/veilshard /usr/local/bin/vpnctl

ENTRYPOINT ["/usr/local/bin/veilshard"]
CMD ["--help"]
