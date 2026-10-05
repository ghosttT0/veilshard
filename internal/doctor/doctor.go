package doctor

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"os"
	"os/exec"
	"strings"
	"time"

	"github.com/veilshard/veilshard/internal/config"
	"github.com/veilshard/veilshard/internal/core/xray"
	"github.com/veilshard/veilshard/internal/firewall/ufw"
	"github.com/veilshard/veilshard/internal/platform"
	"github.com/veilshard/veilshard/internal/platform/ubuntu"
	"github.com/veilshard/veilshard/internal/preflight"
	"github.com/veilshard/veilshard/internal/service/systemd"
)

type Runner struct{}

func New() *Runner {
	return &Runner{}
}

func (r *Runner) Run(ctx context.Context) (*Report, error) {
	rep := &Report{
		Items: make([]CheckItem, 0),
	}

	info, _ := platform.Detect()
	cfg, _ := config.Load("")

	// --- 1. System Category ---
	// Supported OS
	if err := ubuntu.ValidateSupport(info); err != nil {
		rep.add(CheckItem{
			ID:             "SYS_OS",
			Category:       "System",
			Name:           "Supported OS",
			Status:         StatusWarn,
			Message:        err.Error(),
			Recommendation: "Official support is tailored for Ubuntu 24.04.",
		})
	} else {
		rep.add(CheckItem{
			ID:       "SYS_OS",
			Category: "System",
			Name:     "Supported OS",
			Status:   StatusPass,
			Message:  fmt.Sprintf("%s %s", info.Distro, info.Version),
		})
	}

	// Architecture
	if info.Arch == "amd64" {
		rep.add(CheckItem{
			ID:       "SYS_ARCH",
			Category: "System",
			Name:     "Architecture",
			Status:   StatusPass,
			Message:  "amd64",
		})
	} else {
		rep.add(CheckItem{
			ID:       "SYS_ARCH",
			Category: "System",
			Name:     "Architecture",
			Status:   StatusFail,
			Message:  fmt.Sprintf("unsupported architecture: %s", info.Arch),
		})
	}

	// Disk
	if info.FreeDiskMB >= 300 {
		rep.add(CheckItem{
			ID:       "SYS_DISK",
			Category: "System",
			Name:     "Disk Space",
			Status:   StatusPass,
			Message:  fmt.Sprintf("%d MB free", info.FreeDiskMB),
		})
	} else {
		rep.add(CheckItem{
			ID:       "SYS_DISK",
			Category: "System",
			Name:     "Disk Space",
			Status:   StatusWarn,
			Message:  fmt.Sprintf("low disk space: %d MB", info.FreeDiskMB),
		})
	}

	// Memory
	if info.TotalMemMB >= 256 {
		rep.add(CheckItem{
			ID:       "SYS_MEM",
			Category: "System",
			Name:     "Memory",
			Status:   StatusPass,
			Message:  fmt.Sprintf("%d MB total", info.TotalMemMB),
		})
	} else {
		rep.add(CheckItem{
			ID:       "SYS_MEM",
			Category: "System",
			Name:     "Memory",
			Status:   StatusWarn,
			Message:  fmt.Sprintf("low memory: %d MB total", info.TotalMemMB),
		})
	}

	// Clock synchronization
	clockSynced := checkClockSync(ctx)
	if clockSynced {
		rep.add(CheckItem{
			ID:       "SYS_CLOCK",
			Category: "System",
			Name:     "Clock synchronization",
			Status:   StatusPass,
			Message:  "NTP synchronized",
		})
	} else {
		rep.add(CheckItem{
			ID:             "SYS_CLOCK",
			Category:       "System",
			Name:           "Clock synchronization",
			Status:         StatusWarn,
			Message:        "NTP synchronization status uncertain",
			Recommendation: "Run 'timedatectl' to ensure system clock is synchronized.",
		})
	}

	// --- 2. Network Category ---
	// Public IPv4
	publicIP := cfg.Server.PublicIP
	if publicIP != "" {
		rep.add(CheckItem{
			ID:       "NET_IPV4",
			Category: "Network",
			Name:     "Public IPv4",
			Status:   StatusPass,
			Message:  publicIP,
		})
	} else {
		rep.add(CheckItem{
			ID:       "NET_IPV4",
			Category: "Network",
			Name:     "Public IPv4",
			Status:   StatusWarn,
			Message:  "Not configured or detected",
		})
	}

	// DNS resolution
	_, err := net.LookupHost("cloudflare.com")
	if err == nil {
		rep.add(CheckItem{
			ID:       "NET_DNS",
			Category: "Network",
			Name:     "DNS resolution",
			Status:   StatusPass,
			Message:  "resolved cloudflare.com",
		})
	} else {
		rep.add(CheckItem{
			ID:       "NET_DNS",
			Category: "Network",
			Name:     "DNS resolution",
			Status:   StatusFail,
			Message:  fmt.Sprintf("DNS lookup failed: %v", err),
		})
	}

	// Proxy port listening
	port := cfg.Server.Port
	if port <= 0 {
		port = 443
	}
	conn, err := net.DialTimeout("tcp", fmt.Sprintf("127.0.0.1:%d", port), 2*time.Second)
	if err == nil {
		_ = conn.Close()
		rep.add(CheckItem{
			ID:       "NET_PORT",
			Category: "Network",
			Name:     "Proxy port listening",
			Status:   StatusPass,
			Message:  fmt.Sprintf("TCP/%d is listening", port),
		})
	} else {
		rep.add(CheckItem{
			ID:       "NET_PORT",
			Category: "Network",
			Name:     "Proxy port listening",
			Status:   StatusFail,
			Message:  fmt.Sprintf("TCP/%d is NOT responding (%v)", port, err),
		})
	}

	// Internet connectivity
	httpClient := &http.Client{Timeout: 3 * time.Second}
	resp, err := httpClient.Get("https://1.1.1.1")
	if err == nil {
		_ = resp.Body.Close()
		rep.add(CheckItem{
			ID:       "NET_INET",
			Category: "Network",
			Name:     "Internet connectivity",
			Status:   StatusPass,
			Message:  "outbound internet OK",
		})
	} else {
		rep.add(CheckItem{
			ID:       "NET_INET",
			Category: "Network",
			Name:     "Internet connectivity",
			Status:   StatusWarn,
			Message:  fmt.Sprintf("outbound probe failed: %v", err),
		})
	}

	// --- 3. Security Category ---
	// Firewall enabled
	ufwMgr := ufw.New()
	fwActive, _, _ := ufwMgr.Status(ctx)
	if fwActive {
		rep.add(CheckItem{
			ID:       "SEC_FW",
			Category: "Security",
			Name:     "Firewall enabled",
			Status:   StatusPass,
			Message:  "UFW is active",
		})
	} else {
		rep.add(CheckItem{
			ID:       "SEC_FW",
			Category: "Security",
			Name:     "Firewall enabled",
			Status:   StatusWarn,
			Message:  "UFW is currently inactive",
		})
	}

	// SSH rule preserved
	sshPorts, _ := preflight.DetectSSHPorts()
	sshRuleOK := true
	for _, sp := range sshPorts {
		allowed, _ := ufwMgr.IsPortAllowed(ctx, sp, "tcp")
		if !allowed && fwActive {
			sshRuleOK = false
		}
	}
	if sshRuleOK {
		rep.add(CheckItem{
			ID:       "SEC_SSH_RULE",
			Category: "Security",
			Name:     "SSH rule preserved",
			Status:   StatusPass,
			Message:  fmt.Sprintf("SSH on %s protected", preflight.FormatSSHPorts(sshPorts)),
		})
	} else {
		rep.add(CheckItem{
			ID:             "SEC_SSH_RULE",
			Category:       "Security",
			Name:           "SSH rule preserved",
			Status:         StatusWarn,
			Message:        "SSH port rule missing in active firewall",
			Recommendation: "Run 'sudo ufw allow <port>/tcp comment SSH' to avoid lockout.",
		})
	}

	// Core isn't running as root
	if cfg.Security.RunAsUser != "" && cfg.Security.RunAsUser != "root" {
		rep.add(CheckItem{
			ID:       "SEC_ROOT",
			Category: "Security",
			Name:     "Core isn't running as root",
			Status:   StatusPass,
			Message:  fmt.Sprintf("configured user: %s", cfg.Security.RunAsUser),
		})
	} else {
		rep.add(CheckItem{
			ID:             "SEC_ROOT",
			Category:       "Security",
			Name:           "Core isn't running as root",
			Status:         StatusWarn,
			Message:        "proxy is configured to run as root",
			Recommendation: "Configure security.run_as_user to 'vpnctl-proxy' in vpnctl.yaml.",
		})
	}

	// Config permissions
	coreCfgPath := "/etc/vpnctl/core/config.json"
	if fi, err := os.Stat(coreCfgPath); err == nil {
		mode := fi.Mode().Perm()
		if mode <= 0640 {
			rep.add(CheckItem{
				ID:       "SEC_PERM",
				Category: "Security",
				Name:     "Config permission",
				Status:   StatusPass,
				Message:  fmt.Sprintf("file permission is %04o", mode),
			})
		} else {
			rep.add(CheckItem{
				ID:             "SEC_PERM",
				Category:       "Security",
				Name:           "Config permission",
				Status:         StatusWarn,
				Message:        fmt.Sprintf("file permission %04o is overly permissive", mode),
				Recommendation: "Run 'chmod 0640 /etc/vpnctl/core/config.json'",
			})
		}
	}

	// Private key permission
	credsPath := "/etc/vpnctl/credentials.json"
	if fi, err := os.Stat(credsPath); err == nil {
		mode := fi.Mode().Perm()
		if mode <= 0600 {
			rep.add(CheckItem{
				ID:       "SEC_KEY_PERM",
				Category: "Security",
				Name:     "Private key permission",
				Status:   StatusPass,
				Message:  fmt.Sprintf("permission %04o", mode),
			})
		} else {
			rep.add(CheckItem{
				ID:             "SEC_KEY_PERM",
				Category:       "Security",
				Name:           "Private key permission",
				Status:         StatusWarn,
				Message:        fmt.Sprintf("permission %04o is overly permissive", mode),
				Recommendation: "Run 'chmod 0600 /etc/vpnctl/credentials.json'",
			})
		}
	}

	// --- 4. Service Category ---
	// systemd active
	svcMgr := systemd.New("vpnctl-proxy")
	svcSt, _ := svcMgr.Status(ctx)
	if svcSt != nil && svcSt.Active {
		rep.add(CheckItem{
			ID:       "SRV_ACTIVE",
			Category: "Service",
			Name:     "systemd active",
			Status:   StatusPass,
			Message:  fmt.Sprintf("running (PID %d)", svcSt.PID),
		})
	} else {
		rep.add(CheckItem{
			ID:       "SRV_ACTIVE",
			Category: "Service",
			Name:     "systemd active",
			Status:   StatusFail,
			Message:  "service is not active",
		})
	}

	// Configuration valid
	xr := xray.New("", "")
	if err := xr.Validate(ctx); err == nil {
		rep.add(CheckItem{
			ID:       "SRV_CFG_VALID",
			Category: "Service",
			Name:     "configuration valid",
			Status:   StatusPass,
			Message:  "xray test passed",
		})
	} else {
		rep.add(CheckItem{
			ID:       "SRV_CFG_VALID",
			Category: "Service",
			Name:     "configuration valid",
			Status:   StatusFail,
			Message:  fmt.Sprintf("syntax error: %v", err),
		})
	}

	// No restart loop
	if svcSt != nil && svcSt.RestartCnt < 3 {
		rep.add(CheckItem{
			ID:       "SRV_LOOP",
			Category: "Service",
			Name:     "no restart loop",
			Status:   StatusPass,
			Message:  fmt.Sprintf("%d restarts observed", svcSt.RestartCnt),
		})
	} else if svcSt != nil {
		rep.add(CheckItem{
			ID:             "SRV_LOOP",
			Category:       "Service",
			Name:           "no restart loop",
			Status:         StatusWarn,
			Message:        fmt.Sprintf("elevated restarts: %d", svcSt.RestartCnt),
			Recommendation: "Check crash logs with 'vpnctl logs'.",
		})
	}

	return rep, nil
}

func (r *Runner) Explain(ruleID string) (*CheckItem, error) {
	ruleID = strings.ToUpper(strings.TrimSpace(ruleID))
	item, ok := Explanations[ruleID]
	if !ok {
		return nil, fmt.Errorf("no detailed explanation found for rule %s", ruleID)
	}
	return &item, nil
}

func (rep *Report) add(item CheckItem) {
	rep.Items = append(rep.Items, item)
	switch item.Status {
	case StatusPass:
		rep.Pass++
	case StatusWarn:
		rep.Warn++
	case StatusFail:
		rep.Fail++
	}
}

func checkClockSync(ctx context.Context) bool {
	out, err := exec.CommandContext(ctx, "timedatectl", "show", "--property=NTPSynchronized").Output()
	if err == nil && strings.Contains(string(out), "NTPSynchronized=yes") {
		return true
	}
	return false
}
