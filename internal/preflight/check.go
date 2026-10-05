package preflight

import (
	"context"
	"fmt"
	"io"
	"net"
	"net/http"
	"strings"
	"time"

	"github.com/veilshard/veilshard/internal/platform"
	"github.com/veilshard/veilshard/internal/platform/ubuntu"
	"github.com/veilshard/veilshard/internal/state"
)

// Environment holds the inspected system context.
type Environment struct {
	Platform     *platform.Info `json:"platform"`
	PublicIPv4   string         `json:"public_ipv4"`
	SSHPorts     []int          `json:"ssh_ports"`
	PortConflict bool           `json:"port_conflict"`
	IsInstalled  bool           `json:"is_installed"`
}

// CheckResult represents a single check entry.
type CheckResult struct {
	Name    string
	Passed  bool
	Message string
	Warning bool
}

// Run executes the complete preflight verification.
func Run(ctx context.Context, targetPort int) (*Environment, []CheckResult, error) {
	var results []CheckResult

	// 1. Detect Platform
	info, err := platform.Detect()
	if err != nil {
		return nil, nil, fmt.Errorf("detect platform: %w", err)
	}

	env := &Environment{
		Platform: info,
	}

	// OS check
	ubuntuErr := ubuntu.ValidateSupport(info)
	if ubuntuErr != nil {
		results = append(results, CheckResult{
			Name:    "OS Support",
			Passed:  false,
			Message: ubuntuErr.Error(),
			Warning: true, // Warn if non-standard, but allow force
		})
	} else {
		results = append(results, CheckResult{
			Name:    "Operating System",
			Passed:  true,
			Message: fmt.Sprintf("%s %s (%s)", info.Distro, info.Version, info.Arch),
		})
	}

	// Root privileges
	if !info.IsRoot {
		results = append(results, CheckResult{
			Name:    "Privileges",
			Passed:  false,
			Message: "root privileges required (run with sudo)",
		})
	} else {
		results = append(results, CheckResult{
			Name:    "Privileges",
			Passed:  true,
			Message: "root privileges confirmed",
		})
	}

	// Systemd
	if !info.HasSystemd {
		results = append(results, CheckResult{
			Name:    "Init System",
			Passed:  false,
			Message: "systemd not detected",
		})
	} else {
		results = append(results, CheckResult{
			Name:    "Init System",
			Passed:  true,
			Message: "systemd detected",
		})
	}

	// UFW
	if !info.HasUFW {
		results = append(results, CheckResult{
			Name:    "Firewall",
			Passed:  false,
			Message: "UFW not found (please install ufw)",
		})
	} else {
		results = append(results, CheckResult{
			Name:    "Firewall",
			Passed:  true,
			Message: "UFW detected",
		})
	}

	// Disk space (minimum 300MB)
	if info.FreeDiskMB > 0 && info.FreeDiskMB < 300 {
		results = append(results, CheckResult{
			Name:    "Disk Space",
			Passed:  false,
			Message: fmt.Sprintf("low disk space: %d MB free (min 300 MB required)", info.FreeDiskMB),
		})
	} else {
		results = append(results, CheckResult{
			Name:    "Disk Space",
			Passed:  true,
			Message: fmt.Sprintf("%d MB available", info.FreeDiskMB),
		})
	}

	// 2. SSH Detection
	sshPorts, _ := DetectSSHPorts()
	env.SSHPorts = sshPorts
	results = append(results, CheckResult{
		Name:    "SSH Service",
		Passed:  len(sshPorts) > 0,
		Message: fmt.Sprintf("Detected on %s", FormatSSHPorts(sshPorts)),
	})

	// 3. Network & Public IPv4
	publicIP := detectPublicIPv4(ctx)
	env.PublicIPv4 = publicIP
	if publicIP != "" {
		results = append(results, CheckResult{
			Name:    "Public IPv4",
			Passed:  true,
			Message: publicIP,
		})
	} else {
		results = append(results, CheckResult{
			Name:    "Public IPv4",
			Passed:  false,
			Message: "unable to determine public IPv4 address",
			Warning: true,
		})
	}

	// 4. Target Port Conflict
	if targetPort <= 0 {
		targetPort = 443
	}
	portConflict := checkPortConflict(targetPort)
	env.PortConflict = portConflict
	if portConflict {
		results = append(results, CheckResult{
			Name:    fmt.Sprintf("Port TCP/%d", targetPort),
			Passed:  false,
			Message: fmt.Sprintf("TCP/%d is already in use by another process", targetPort),
		})
	} else {
		results = append(results, CheckResult{
			Name:    fmt.Sprintf("Port TCP/%d", targetPort),
			Passed:  true,
			Message: "available",
		})
	}

	// 5. Existing Installation
	st, _ := state.Load("")
	if st != nil && st.Installed {
		env.IsInstalled = true
		results = append(results, CheckResult{
			Name:    "Installation State",
			Passed:  true,
			Message: fmt.Sprintf("existing installation detected (core: %s)", st.CoreVersion),
			Warning: true,
		})
	}

	return env, results, nil
}

// detectPublicIPv4 queries multiple lightweight IP endpoints with timeout.
func detectPublicIPv4(ctx context.Context) string {
	endpoints := []string{
		"https://api.ipify.org",
		"https://icanhazip.com",
		"https://ifconfig.me/ip",
		"https://ip.sb",
	}

	client := &http.Client{
		Timeout: 4 * time.Second,
	}

	for _, ep := range endpoints {
		req, err := http.NewRequestWithContext(ctx, "GET", ep, nil)
		if err != nil {
			continue
		}
		resp, err := client.Do(req)
		if err != nil {
			continue
		}
		data, err := io.ReadAll(resp.Body)
		_ = resp.Body.Close()
		if err != nil {
			continue
		}
		ipStr := strings.TrimSpace(string(data))
		ip := net.ParseIP(ipStr)
		if ip != nil && ip.To4() != nil {
			return ip.String()
		}
	}

	return ""
}

// DetectPublicIPv6 queries lightweight IPv6 endpoints or inspects local interfaces.
func DetectPublicIPv6(ctx context.Context) string {
	endpoints := []string{
		"https://api6.ipify.org",
		"https://ipv6.icanhazip.com",
		"https://v6.ident.me",
	}

	client := &http.Client{
		Timeout: 3 * time.Second,
	}

	for _, ep := range endpoints {
		req, err := http.NewRequestWithContext(ctx, "GET", ep, nil)
		if err != nil {
			continue
		}
		resp, err := client.Do(req)
		if err != nil {
			continue
		}
		data, err := io.ReadAll(resp.Body)
		_ = resp.Body.Close()
		if err != nil {
			continue
		}
		ipStr := strings.TrimSpace(string(data))
		ip := net.ParseIP(ipStr)
		if ip != nil && ip.To4() == nil && ip.To16() != nil && !ip.IsLoopback() && !ip.IsPrivate() {
			return ip.String()
		}
	}

	return ""
}


// checkPortConflict checks if listening on :port succeeds.
func checkPortConflict(port int) bool {
	l, err := net.Listen("tcp", fmt.Sprintf(":%d", port))
	if err != nil {
		return true // port is occupied
	}
	_ = l.Close()
	return false
}
