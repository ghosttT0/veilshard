package firewall

import (
	"context"
)

// Firewall defines management methods for firewall rules.
type Firewall interface {
	Name() string
	EnsureSSHAllowed(ctx context.Context, sshPorts []int) error
	AllowProxyPort(ctx context.Context, port int, proto string) error
	RemoveProxyPort(ctx context.Context, port int, proto string) error
	Status(ctx context.Context) (enabled bool, rawStatus string, err error)
	IsPortAllowed(ctx context.Context, port int, proto string) (bool, error)
}
