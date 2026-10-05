package ufw

import (
	"bufio"
	"bytes"
	"context"
	"fmt"
	"os/exec"
	"strconv"
	"strings"

	"github.com/veilshard/veilshard/internal/firewall"
)

type Manager struct{}

func New() firewall.Firewall {
	return &Manager{}
}

func (m *Manager) Name() string {
	return "ufw"
}

// EnsureSSHAllowed verifies that detected SSH ports are explicitly allowed in UFW before applying other rules.
func (m *Manager) EnsureSSHAllowed(ctx context.Context, sshPorts []int) error {
	for _, port := range sshPorts {
		if port <= 0 {
			continue
		}
		allowed, err := m.IsPortAllowed(ctx, port, "tcp")
		if err != nil {
			return fmt.Errorf("check ssh port %d: %w", port, err)
		}
		if !allowed {
			// Add rule for SSH port
			cmd := exec.CommandContext(ctx, "ufw", "allow", fmt.Sprintf("%d/tcp", port), "comment", "SSH")
			out, err := cmd.CombinedOutput()
			if err != nil {
				return fmt.Errorf("allow ssh port %d/tcp: %w (%s)", port, err, string(out))
			}
		}
	}
	return nil
}

// AllowProxyPort opens the proxy port with a specific 'vpnctl:proxy' comment for clean tracking.
func (m *Manager) AllowProxyPort(ctx context.Context, port int, proto string) error {
	if proto == "" {
		proto = "tcp"
	}
	rule := fmt.Sprintf("%d/%s", port, proto)

	allowed, err := m.IsPortAllowed(ctx, port, proto)
	if err != nil {
		return err
	}
	if allowed {
		return nil
	}

	cmd := exec.CommandContext(ctx, "ufw", "allow", rule, "comment", "vpnctl:proxy")
	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("ufw allow %s: %w (%s)", rule, err, string(out))
	}
	return nil
}

// RemoveProxyPort removes only the proxy port rule managed by vpnctl.
func (m *Manager) RemoveProxyPort(ctx context.Context, port int, proto string) error {
	if proto == "" {
		proto = "tcp"
	}
	rule := fmt.Sprintf("%d/%s", port, proto)

	cmd := exec.CommandContext(ctx, "ufw", "delete", "allow", rule)
	out, err := cmd.CombinedOutput()
	if err != nil {
		// If rule already doesn't exist, ignore error
		if strings.Contains(string(out), "Could not delete") || strings.Contains(string(out), "not found") {
			return nil
		}
		return fmt.Errorf("ufw delete allow %s: %w (%s)", rule, err, string(out))
	}
	return nil
}

// Status returns whether UFW is active and its raw status string.
func (m *Manager) Status(ctx context.Context) (bool, string, error) {
	cmd := exec.CommandContext(ctx, "ufw", "status")
	out, err := cmd.CombinedOutput()
	raw := string(out)
	if err != nil {
		return false, raw, fmt.Errorf("ufw status: %w (%s)", err, raw)
	}

	enabled := strings.Contains(strings.ToLower(raw), "status: active")
	return enabled, strings.TrimSpace(raw), nil
}

// IsPortAllowed checks if a specific port and protocol is currently permitted in UFW.
func (m *Manager) IsPortAllowed(ctx context.Context, port int, proto string) (bool, error) {
	cmd := exec.CommandContext(ctx, "ufw", "status")
	out, err := cmd.Output()
	if err != nil {
		return false, err
	}

	portStr := strconv.Itoa(port)
	protoLower := strings.ToLower(proto)

	scanner := bufio.NewScanner(bytes.NewReader(out))
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "Status:") || strings.HasPrefix(line, "To") || strings.HasPrefix(line, "--") {
			continue
		}

		// Example lines:
		// 443/tcp ALLOW Anywhere
		// 62505/tcp ALLOW Anywhere
		// 22 ALLOW Anywhere
		fields := strings.Fields(line)
		if len(fields) >= 2 {
			rulePortProto := fields[0]
			action := strings.ToUpper(fields[1])

			if action == "ALLOW" || action == "LIMIT" {
				if rulePortProto == portStr {
					return true, nil
				}
				if rulePortProto == fmt.Sprintf("%s/%s", portStr, protoLower) {
					return true, nil
				}
			}
		}
	}

	return false, nil
}
