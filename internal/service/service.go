package service

import (
	"context"
	"io"
	"time"
)

// ServiceConfig defines parameters to render the systemd unit.
type ServiceConfig struct {
	ServiceName string
	BinaryPath  string
	ConfigPath  string
	User        string
	Description string
}

// Status represents the current execution status of the service.
type Status struct {
	Active     bool
	SubState   string
	PID        int
	Uptime     time.Duration
	StartedAt  time.Time
	RestartCnt int
}

// Manager defines the service lifecycle interface.
type Manager interface {
	Name() string
	CreateSystemUser(ctx context.Context, username string) error
	Install(ctx context.Context, cfg ServiceConfig) error
	Start(ctx context.Context) error
	Stop(ctx context.Context) error
	Restart(ctx context.Context) error
	Enable(ctx context.Context) error
	Disable(ctx context.Context) error
	Remove(ctx context.Context) error
	Status(ctx context.Context) (*Status, error)
	GetLogs(ctx context.Context, lines int) (string, error)
	FollowLogs(ctx context.Context, lines int) (io.ReadCloser, error)
}
