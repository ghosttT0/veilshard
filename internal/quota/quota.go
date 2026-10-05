package quota

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"time"
)

type QuotaStatus struct {
	QuotaBytes    uint64    `json:"quota_bytes"`
	UsedBytes     uint64    `json:"used_bytes"`
	UsagePercent  float64   `json:"usage_percent"`
	State         string    `json:"state"` // "NORMAL", "DRAINING", "BLOCKED"
	UpdatedAt     time.Time `json:"updated_at"`
	QuotaHuman    string    `json:"quota_human"`
	UsedHuman     string    `json:"used_human"`
}

// EnsureAccountingRules sets up kernel byte counters for port 443 without DB dependencies.
func EnsureAccountingRules(port int) error {
	if port <= 0 {
		port = 443
	}

	cmds := []string{
		"iptables -N vpnctl-acct 2>/dev/null || true",
		fmt.Sprintf("iptables -C INPUT -p tcp --dport %d -j vpnctl-acct 2>/dev/null || iptables -I INPUT 1 -p tcp --dport %d -j vpnctl-acct", port, port),
		fmt.Sprintf("iptables -C OUTPUT -p tcp --sport %d -j vpnctl-acct 2>/dev/null || iptables -I OUTPUT 1 -p tcp --sport %d -j vpnctl-acct", port, port),
		"iptables -C vpnctl-acct -j RETURN 2>/dev/null || iptables -A vpnctl-acct -j RETURN",
	}

	for _, cmdStr := range cmds {
		_ = exec.Command("sh", "-c", cmdStr).Run()
	}
	return nil
}

// QueryUsedBytes queries kernel counters from iptables or /proc/net/dev without disk I/O.
func QueryUsedBytes(ctx context.Context) (uint64, error) {
	cmd := exec.CommandContext(ctx, "iptables", "-L", "vpnctl-acct", "-v", "-n", "-x")
	out, err := cmd.CombinedOutput()
	if err == nil {
		lines := strings.Split(string(out), "\n")
		var totalBytes uint64
		for _, l := range lines {
			fields := strings.Fields(l)
			if len(fields) >= 2 {
				var b uint64
				if _, errScan := fmt.Sscanf(fields[1], "%d", &b); errScan == nil && b > 0 {
					totalBytes += b
				}
			}
		}
		if totalBytes > 0 {
			return totalBytes, nil
		}
	}

	// Fallback: Check primary network interface counters from /proc/net/dev
	data, err := os.ReadFile("/proc/net/dev")
	if err == nil {
		lines := strings.Split(string(data), "\n")
		for _, line := range lines {
			trimmed := strings.TrimSpace(line)
			if strings.HasPrefix(trimmed, "eth0:") || strings.HasPrefix(trimmed, "ens") || strings.HasPrefix(trimmed, "enp") {
				parts := strings.Fields(trimmed)
				if len(parts) >= 10 {
					rx, _ := strconv.ParseUint(parts[1], 10, 64)
					tx, _ := strconv.ParseUint(parts[9], 10, 64)
					return rx + tx, nil
				}
			}
		}
	}

	return 0, nil
}

// ParseQuotaString converts human strings like "800GB", "1TB", "500MB" to bytes.
func ParseQuotaString(s string) uint64 {
	s = strings.ToUpper(strings.TrimSpace(s))
	if s == "" {
		return 0
	}

	multiplier := uint64(1)
	cleanStr := s

	if strings.HasSuffix(s, "TB") {
		multiplier = 1024 * 1024 * 1024 * 1024
		cleanStr = strings.TrimSuffix(s, "TB")
	} else if strings.HasSuffix(s, "GB") {
		multiplier = 1024 * 1024 * 1024
		cleanStr = strings.TrimSuffix(s, "GB")
	} else if strings.HasSuffix(s, "MB") {
		multiplier = 1024 * 1024
		cleanStr = strings.TrimSuffix(s, "MB")
	}

	val, err := strconv.ParseFloat(strings.TrimSpace(cleanStr), 64)
	if err != nil {
		return 0
	}
	return uint64(val * float64(multiplier))
}

// FormatBytes produces clean human-readable representation.
func FormatBytes(b uint64) string {
	const unit = 1024
	if b < unit {
		return fmt.Sprintf("%d B", b)
	}
	div, exp := int64(unit), 0
	for n := b / unit; n >= unit; n /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.2f %cB", float64(b)/float64(div), "KMGTPE"[exp])
}

// CheckAndEnforce evaluates current traffic against declared monthly quota.
func CheckAndEnforce(ctx context.Context, quotaStr string, port int, webhookURL string) (*QuotaStatus, error) {
	if port <= 0 {
		port = 443
	}

	limitBytes := ParseQuotaString(quotaStr)
	if limitBytes == 0 {
		// Default to 1TB if not configured
		limitBytes = 1000 * 1024 * 1024 * 1024
	}

	usedBytes, _ := QueryUsedBytes(ctx)
	percent := (float64(usedBytes) / float64(limitBytes)) * 100.0

	status := &QuotaStatus{
		QuotaBytes:   limitBytes,
		UsedBytes:    usedBytes,
		UsagePercent: percent,
		State:        "NORMAL",
		UpdatedAt:    time.Now().UTC(),
		QuotaHuman:   FormatBytes(limitBytes),
		UsedHuman:    FormatBytes(usedBytes),
	}

	// 1. Critical Level (>= 98%): Edge Circuit Breaker
	if percent >= 98.0 {
		status.State = "BLOCKED"
		// Sever incoming connections to prevent expensive out-of-quota billing
		_ = exec.Command("iptables", "-I", "INPUT", "1", "-p", "tcp", "--dport", strconv.Itoa(port), "-j", "DROP").Run()
		if webhookURL != "" {
			_ = sendNotification(ctx, webhookURL, fmt.Sprintf("🚨 CRITICAL: Node traffic reached %.1f%% (%s / %s). Port %d automatically blocked by burst protector!", percent, status.UsedHuman, status.QuotaHuman, port))
		}
		return status, nil
	}

	// 2. Warning Level (>= 90%): Node Draining & Alert
	if percent >= 90.0 {
		status.State = "DRAINING"
		if webhookURL != "" {
			_ = sendNotification(ctx, webhookURL, fmt.Sprintf("⚠️ WARNING: Node traffic reached %.1f%% (%s / %s). Node marked as DRAINING for client failover.", percent, status.UsedHuman, status.QuotaHuman))
		}
		return status, nil
	}

	return status, nil
}

// ResetCounters resets the kernel byte counters for a new billing cycle.
func ResetCounters() error {
	return exec.Command("iptables", "-Z", "vpnctl-acct").Run()
}

func sendNotification(ctx context.Context, targetURL, message string) error {
	if strings.Contains(targetURL, "api.telegram.org") {
		// Form POST or GET for Telegram
		endpoint := fmt.Sprintf("%s&text=%s", targetURL, message)
		req, _ := http.NewRequestWithContext(ctx, "GET", endpoint, nil)
		resp, err := http.DefaultClient.Do(req)
		if err == nil {
			_ = resp.Body.Close()
		}
		return err
	}

	// Generic JSON webhook
	payload := map[string]interface{}{
		"text":       message,
		"timestamp":  time.Now().Unix(),
		"source":     "vpnctl-burst-protector",
	}
	body, _ := json.Marshal(payload)
	req, err := http.NewRequestWithContext(ctx, "POST", targetURL, bytes.NewReader(body))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := http.DefaultClient.Do(req)
	if err == nil {
		_ = resp.Body.Close()
	}
	return err
}
