package cli

import (
	"bufio"
	"context"
	"flag"
	"fmt"
	"os"
	"strings"

	"github.com/veilshard/veilshard/internal/firewall/ufw"
	"github.com/veilshard/veilshard/internal/service/systemd"
	"github.com/veilshard/veilshard/internal/state"
)

func runUninstall(args []string) error {
	fs := flag.NewFlagSet("uninstall", flag.ContinueOnError)
	keepConfig := fs.Bool("keep-config", false, "Retain configuration and user database")
	yes := fs.Bool("y", false, "Confirm uninstallation without prompting")

	if err := fs.Parse(args); err != nil {
		return err
	}

	ui := NewUI()
	ui.Header("vpnctl uninstall")

	if os.Geteuid() != 0 {
		ui.Error("vpnctl uninstall requires root privileges (sudo)")
		return fmt.Errorf("root required")
	}

	st, _ := state.Load("")

	// Display clear removal and preservation lists
	ui.Section("Will remove:")
	fmt.Printf("• vpnctl proxy service (systemd)\n")
	fmt.Printf("• proxy core binary (%s)\n", st.CoreBinaryPath)
	for _, rule := range st.ManagedFirewallRules {
		fmt.Printf("• managed firewall rule (%s)\n", rule)
	}
	if !*keepConfig {
		fmt.Printf("• vpnctl configuration and credentials (/etc/vpnctl)\n")
		fmt.Printf("• runtime logs and state data (/var/lib/vpnctl, /var/log/vpnctl)\n")
	}

	ui.Section("Will NOT modify:")
	fmt.Printf("• SSH access rules & configuration\n")
	fmt.Printf("• Existing user-defined UFW rules\n")
	fmt.Printf("• Docker or other server services\n")

	fmt.Println()

	if !*yes {
		fmt.Printf("%sAre you sure you want to uninstall vpnctl? [y/N]: %s", ColorBold+ColorYellow, ColorReset)
		reader := bufio.NewReader(os.Stdin)
		input, _ := reader.ReadString('\n')
		input = strings.TrimSpace(strings.ToLower(input))
		if input != "y" && input != "yes" {
			fmt.Println("Uninstall cancelled.")
			return nil
		}
	}

	ctx := context.Background()
	ui.Section("Teardown Progress")

	// 1. Stop and remove service
	svc := systemd.New("vpnctl-proxy")
	_ = svc.Stop(ctx)
	_ = svc.Remove(ctx)
	ui.Step("Service stopped and removed", true, "vpnctl-proxy")

	// 2. Remove managed firewall rules only
	fw := ufw.New()
	for _, r := range st.ManagedFirewallRules {
		// r is e.g. "443/tcp"
		parts := strings.Split(r, "/")
		port := 443
		proto := "tcp"
		if len(parts) >= 1 {
			_, _ = fmt.Sscanf(parts[0], "%d", &port)
		}
		if len(parts) >= 2 {
			proto = parts[1]
		}
		_ = fw.RemoveProxyPort(ctx, port, proto)
		ui.Step(fmt.Sprintf("Removed firewall rule %s", r), true, "")
	}

	// 3. Remove core binary
	if st.CoreBinaryPath != "" {
		_ = os.Remove(st.CoreBinaryPath)
		ui.Step("Removed core binary", true, st.CoreBinaryPath)
	}

	// 4. Remove configs or state
	_ = os.Remove(state.DefaultStatePath)

	if !*keepConfig {
		_ = os.RemoveAll("/etc/vpnctl")
		_ = os.RemoveAll("/var/lib/vpnctl")
		_ = os.RemoveAll("/var/log/vpnctl")
		ui.Step("Removed configurations and logs", true, "/etc/vpnctl")
	} else {
		ui.Step("Retained configuration directory", true, "/etc/vpnctl (keep-config)")
	}

	fmt.Printf("\n%svpnctl has been cleanly uninstalled.%s\n\n", ColorGreen, ColorReset)
	return nil
}
