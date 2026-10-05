package systemd

import (
	"bufio"
	"bytes"
	"context"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/veilshard/veilshard/internal/service"
)

type Manager struct {
	unitName string
}

func New(unitName string) service.Manager {
	if unitName == "" {
		unitName = "vpnctl-proxy"
	}
	return &Manager{unitName: unitName}
}

func (m *Manager) Name() string {
	return "systemd"
}

func (m *Manager) unitFile() string {
	return fmt.Sprintf("/etc/systemd/system/%s.service", m.unitName)
}

func (m *Manager) CreateSystemUser(ctx context.Context, username string) error {
	if username == "" || username == "root" {
		return nil
	}

	// Check if user already exists
	checkCmd := exec.CommandContext(ctx, "id", "-u", username)
	if err := checkCmd.Run(); err == nil {
		return nil // User already exists
	}

	// Ensure group exists
	_ = exec.CommandContext(ctx, "groupadd", "-r", username).Run()

	// Create system user with primary group
	cmd := exec.CommandContext(ctx, "useradd", "-r", "-g", username, "-s", "/usr/sbin/nologin", "-M", "-d", "/nonexistent", username)
	out, err := cmd.CombinedOutput()
	if err != nil {
		// Fallback without -g
		cmdFallback := exec.CommandContext(ctx, "useradd", "-r", "-s", "/usr/sbin/nologin", "-M", "-d", "/nonexistent", username)
		if outFb, errFb := cmdFallback.CombinedOutput(); errFb != nil {
			return fmt.Errorf("useradd %s: %w (%s)", username, errFb, string(outFb))
		}
	}

	return nil
}

func (m *Manager) Install(ctx context.Context, cfg service.ServiceConfig) error {
	unitPath := m.unitFile()

	user := cfg.User
	if user == "" {
		user = "vpnctl-proxy"
	}

	// Ensure system directories exist with proper permissions
	_ = os.MkdirAll("/var/log/vpnctl", 0755)
	_ = os.MkdirAll("/var/lib/vpnctl", 0755)
	if user != "root" {
		_ = exec.Command("chown", "-R", user, "/var/log/vpnctl", "/var/lib/vpnctl").Run()
	}

	userSection := ""
	if user != "" && user != "root" {
		userSection = fmt.Sprintf("User=%s\n", user)
	}

	unitContent := fmt.Sprintf(`[Unit]
Description=%s
Documentation=https://github.com/veilshard/veilshard
After=network.target network-online.target nss-lookup.target
Wants=network-online.target

[Service]
Type=simple
%sCapabilityBoundingSet=CAP_NET_ADMIN CAP_NET_BIND_SERVICE
AmbientCapabilities=CAP_NET_ADMIN CAP_NET_BIND_SERVICE
NoNewPrivileges=true
ExecStart=%s run -config %s
Restart=on-failure
RestartSec=5s
LimitNOFILE=65535

[Install]
WantedBy=multi-user.target
`, cfg.Description, userSection, cfg.BinaryPath, cfg.ConfigPath)

	dir := filepath.Dir(unitPath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("mkdir unit dir: %w", err)
	}

	if err := os.WriteFile(unitPath, []byte(unitContent), 0644); err != nil {
		return fmt.Errorf("write systemd unit: %w", err)
	}

	// Reload daemon
	if err := exec.CommandContext(ctx, "systemctl", "daemon-reload").Run(); err != nil {
		return fmt.Errorf("systemctl daemon-reload: %w", err)
	}

	// Enable unit
	if err := exec.CommandContext(ctx, "systemctl", "enable", m.unitName).Run(); err != nil {
		return fmt.Errorf("systemctl enable %s: %w", m.unitName, err)
	}

	return nil
}

func (m *Manager) Start(ctx context.Context) error {
	out, err := exec.CommandContext(ctx, "systemctl", "start", m.unitName).CombinedOutput()
	if err != nil {
		return fmt.Errorf("systemctl start %s: %w (%s)", m.unitName, err, string(out))
	}
	return nil
}

func (m *Manager) Stop(ctx context.Context) error {
	out, err := exec.CommandContext(ctx, "systemctl", "stop", m.unitName).CombinedOutput()
	if err != nil {
		return fmt.Errorf("systemctl stop %s: %w (%s)", m.unitName, err, string(out))
	}
	return nil
}

func (m *Manager) Restart(ctx context.Context) error {
	out, err := exec.CommandContext(ctx, "systemctl", "restart", m.unitName).CombinedOutput()
	if err != nil {
		return fmt.Errorf("systemctl restart %s: %w (%s)", m.unitName, err, string(out))
	}
	return nil
}

func (m *Manager) Enable(ctx context.Context) error {
	return exec.CommandContext(ctx, "systemctl", "enable", m.unitName).Run()
}

func (m *Manager) Disable(ctx context.Context) error {
	return exec.CommandContext(ctx, "systemctl", "disable", m.unitName).Run()
}

func (m *Manager) Remove(ctx context.Context) error {
	_ = m.Stop(ctx)
	_ = m.Disable(ctx)
	_ = os.Remove(m.unitFile())
	return exec.CommandContext(ctx, "systemctl", "daemon-reload").Run()
}

func (m *Manager) Status(ctx context.Context) (*service.Status, error) {
	cmd := exec.CommandContext(ctx, "systemctl", "show", m.unitName,
		"--property=ActiveState,SubState,MainPID,ExecMainStartTimestamp,NRestarts")
	out, err := cmd.Output()
	if err != nil {
		return &service.Status{Active: false, SubState: "inactive"}, nil
	}

	st := &service.Status{}
	scanner := bufio.NewScanner(bytes.NewReader(out))
	for scanner.Scan() {
		line := scanner.Text()
		parts := strings.SplitN(line, "=", 2)
		if len(parts) != 2 {
			continue
		}
		k, v := parts[0], parts[1]
		switch k {
		case "ActiveState":
			st.Active = (v == "active")
		case "SubState":
			st.SubState = v
		case "MainPID":
			if pid, err := strconv.Atoi(v); err == nil {
				st.PID = pid
			}
		case "ExecMainStartTimestamp":
			// Example format: Mon 2026-10-05 19:30:00 UTC
			if t, err := time.Parse("Mon 2006-01-02 15:04:05 MST", v); err == nil {
				st.StartedAt = t
				st.Uptime = time.Since(t)
			}
		case "NRestarts":
			if r, err := strconv.Atoi(v); err == nil {
				st.RestartCnt = r
			}
		}
	}

	return st, nil
}

func (m *Manager) GetLogs(ctx context.Context, lines int) (string, error) {
	if lines <= 0 {
		lines = 50
	}
	cmd := exec.CommandContext(ctx, "journalctl", "-u", m.unitName, "-n", strconv.Itoa(lines), "--no-pager")
	out, err := cmd.CombinedOutput()
	if err != nil {
		return string(out), fmt.Errorf("journalctl: %w", err)
	}
	return string(out), nil
}

func (m *Manager) FollowLogs(ctx context.Context, lines int) (io.ReadCloser, error) {
	if lines <= 0 {
		lines = 50
	}
	cmd := exec.CommandContext(ctx, "journalctl", "-u", m.unitName, "-f", "-n", strconv.Itoa(lines), "--no-pager")
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return nil, err
	}
	if err := cmd.Start(); err != nil {
		return nil, err
	}
	return stdout, nil
}
