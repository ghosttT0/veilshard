package cli

import (
	"bufio"
	"bytes"
	"context"
	"fmt"
	"net"
	"os/exec"
	"strconv"
	"strings"
	"time"

	"github.com/veilshard/veilshard/internal/config"
	"github.com/veilshard/veilshard/internal/core/xray"
	"github.com/veilshard/veilshard/internal/firewall/ufw"
	"github.com/veilshard/veilshard/internal/service/systemd"
	"github.com/veilshard/veilshard/internal/state"
	"github.com/veilshard/veilshard/internal/users"
)

func runStatus(args []string) error {
	ui := NewUI()
	ctx := context.Background()

	cfg, _ := config.Load("")
	st, _ := state.Load("")
	svcMgr := systemd.New("vpnctl-proxy")
	svcSt, _ := svcMgr.Status(ctx)
	xr := xray.New("", "")
	fwMgr := ufw.New()

	ui.Header("vpnctl status")

	// Service Section
	ui.Section("Service")
	if svcSt != nil && svcSt.Active {
		ui.Success("Running")
	} else {
		ui.Error("Inactive / Stopped")
	}

	// Core Section
	ui.Section("Core")
	ver, _ := xr.Version(ctx)
	if ver == "" && st != nil {
		ver = st.CoreVersion
	}
	if ver == "" {
		ver = "unknown"
	}
	ui.KeyValue("Provider", cfg.Core.Provider)
	ui.KeyValue("Version", ver)
	if svcSt != nil && svcSt.PID > 0 {
		ui.KeyValue("PID", strconv.Itoa(svcSt.PID))
		ui.KeyValue("Uptime", formatDuration(svcSt.Uptime))
	}

	// Network Section
	ui.Section("Network")
	port := cfg.Server.Port
	if port <= 0 {
		port = 443
	}

	isListening := testLocalPort(port)
	if isListening {
		ui.KeyValue(fmt.Sprintf("TCP/%d", port), fmt.Sprintf("%sLISTEN%s", ColorGreen, ColorReset))
	} else {
		ui.KeyValue(fmt.Sprintf("TCP/%d", port), fmt.Sprintf("%sCLOSED%s", ColorRed, ColorReset))
	}

	conns := countActiveConnections(port)
	ui.KeyValue("Connections", strconv.Itoa(conns))

	// Users Section
	ui.Section("Users")
	userStore, _ := users.NewStore("")
	if userStore != nil {
		enabledCount := len(userStore.ListEnabled())
		totalCount := len(userStore.List())
		ui.KeyValue("Active Users", fmt.Sprintf("%d enabled (out of %d)", enabledCount, totalCount))
	}

	// Firewall Section
	ui.Section("Firewall")
	fwActive, _, _ := fwMgr.Status(ctx)
	portAllowed, _ := fwMgr.IsPortAllowed(ctx, port, "tcp")
	if fwActive && portAllowed {
		ui.Success("Protected (UFW active, proxy port open)")
	} else if fwActive {
		ui.Warn("UFW active but proxy port rule is missing")
	} else {
		ui.Warn("Firewall is inactive")
	}

	// Security Note
	ui.Section("Security")
	ui.KeyValue("Protocol", cfg.Protocol.Type)
	ui.KeyValue("SNI Camouflage", cfg.Protocol.Dest)
	ui.KeyValue("Private Key", "******** (hidden)")

	fmt.Println()
	return nil
}

func testLocalPort(port int) bool {
	conn, err := net.DialTimeout("tcp", fmt.Sprintf("127.0.0.1:%d", port), 500*time.Millisecond)
	if err == nil {
		_ = conn.Close()
		return true
	}
	return false
}

func countActiveConnections(port int) int {
	cmd := exec.Command("ss", "-tn", fmt.Sprintf("sport = :%d or dport = :%d", port, port))
	out, err := cmd.Output()
	if err != nil {
		return 0
	}

	lines := 0
	scanner := bufio.NewScanner(bytes.NewReader(out))
	for scanner.Scan() {
		text := strings.TrimSpace(scanner.Text())
		if text != "" && !strings.HasPrefix(text, "State") {
			lines++
		}
	}
	return lines
}

func formatDuration(d time.Duration) string {
	if d <= 0 {
		return "0s"
	}
	days := int(d.Hours()) / 24
	hours := int(d.Hours()) % 24
	minutes := int(d.Minutes()) % 60

	if days > 0 {
		return fmt.Sprintf("%dd %dh %dm", days, hours, minutes)
	}
	if hours > 0 {
		return fmt.Sprintf("%dh %dm", hours, minutes)
	}
	return fmt.Sprintf("%dm", minutes)
}
