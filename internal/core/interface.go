package core

import (
	"context"

	"github.com/veilshard/veilshard/internal/config"
	"github.com/veilshard/veilshard/internal/credentials"
	"github.com/veilshard/veilshard/internal/users"
)

// CoreStatus represents runtime metrics of the proxy core.
type CoreStatus struct {
	Version     string
	Running     bool
	PID         int
	Connections int
}

// Core defines the abstraction interface for proxy engines (Xray, sing-box, etc.).
type Core interface {
	Name() string
	Install(ctx context.Context, version string) error
	GenerateConfig(ctx context.Context, cfg *config.Config, userList []*users.User, creds *credentials.Credentials) error
	Validate(ctx context.Context) error
	BinaryPath() string
	ConfigPath() string
	Version(ctx context.Context) (string, error)
	Status(ctx context.Context) (*CoreStatus, error)
}
