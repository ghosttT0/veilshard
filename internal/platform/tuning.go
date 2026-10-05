package platform

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"os/exec"
	"strings"
)

const SysctlConfPath = "/etc/sysctl.d/99-vpnctl-tuning.conf"

// KernelTuningConfig contains sysctl parameters for high-throughput proxy operation.
var KernelTuningConfig = `
# vpnctl high-performance & security kernel tuning
net.core.default_qdisc = fq
net.ipv4.tcp_congestion_control = bbr
fs.file-max = 1000000
net.core.rmem_max = 67108864
net.core.wmem_max = 67108864
net.ipv4.tcp_rmem = 4096 87380 67108864
net.ipv4.tcp_wmem = 4096 65536 67108864
net.ipv4.tcp_mtu_probing = 1
net.ipv4.tcp_fastopen = 3
net.ipv4.tcp_max_syn_backlog = 8192
net.core.somaxconn = 8192
`

// ApplyKernelTuning enables TCP BBR congestion control and tunes high-throughput socket buffers.
func ApplyKernelTuning(ctx context.Context) (bool, string, error) {
	// 1. Check current congestion control
	data, err := os.ReadFile("/proc/sys/net/ipv4/tcp_congestion_control")
	currentCC := ""
	if err == nil {
		currentCC = strings.TrimSpace(string(data))
	}

	// 2. Try loading tcp_bbr kernel module
	_ = exec.CommandContext(ctx, "modprobe", "tcp_bbr").Run()

	// 3. Write tuning config
	if err := os.WriteFile(SysctlConfPath, []byte(strings.TrimSpace(KernelTuningConfig)+"\n"), 0644); err != nil {
		return false, currentCC, fmt.Errorf("write %s: %w", SysctlConfPath, err)
	}

	// 4. Reload sysctl
	cmd := exec.CommandContext(ctx, "sysctl", "-p", SysctlConfPath)
	out, err := cmd.CombinedOutput()
	if err != nil {
		// Fallback to sysctl --system
		_ = exec.CommandContext(ctx, "sysctl", "--system").Run()
	}

	// 5. Verify if BBR was activated
	dataAfter, err := os.ReadFile("/proc/sys/net/ipv4/tcp_congestion_control")
	if err == nil {
		activeCC := strings.TrimSpace(string(dataAfter))
		if strings.Contains(activeCC, "bbr") {
			return true, "TCP BBR enabled successfully", nil
		}
		return false, fmt.Sprintf("active cc: %s (bbr not supported by kernel)", activeCC), nil
	}

	return true, string(bytes.TrimSpace(out)), nil
}

// ConfigureTCPScanDrop adds iptables rules to drop abnormal TCP port scans (NULL scan, XMAS scan).
func ConfigureTCPScanDrop(ctx context.Context) error {
	rules := [][]string{
		// Drop invalid TCP flags
		{"-t", "mangle", "-A", "PREROUTING", "-m", "conntrack", "--ctstate", "INVALID", "-j", "DROP"},
		// Drop TCP SYN with payload
		{"-t", "mangle", "-A", "PREROUTING", "-p", "tcp", "--syn", "-m", "connbytes", "--connbytes", "1:", "--connbytes-mode", "bytes", "--connbytes-direction", "both", "-j", "DROP"},
	}

	for _, args := range rules {
		// Test if rule exists first to keep idempotent
		checkArgs := append([]string{"-C"}, args[2:]...)
		if err := exec.CommandContext(ctx, "iptables", append([]string{args[0], args[1]}, checkArgs...)...).Run(); err == nil {
			continue // rule already exists
		}
		_ = exec.CommandContext(ctx, "iptables", args...).Run()
	}

	return nil
}
