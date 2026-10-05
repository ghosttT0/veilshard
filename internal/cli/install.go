package cli

import (
	"context"
	"flag"
	"fmt"
	"os"
	"strings"

	"github.com/veilshard/veilshard/internal/exporter"
	"github.com/veilshard/veilshard/internal/installer"
)

func runInstall(args []string) error {
	fs := flag.NewFlagSet("install", flag.ContinueOnError)
	force := fs.Bool("force", false, "Force installation even if non-critical preflight checks warn")
	port := fs.Int("port", 443, "Listening TCP port for proxy inbound")
	dest := fs.String("dest", "gateway.icloud.com", "Camouflage destination domain for REALITY SNI")
	coreVer := fs.String("version", "v25.1.30", "Target Xray-core version")
	listen := fs.String("listen", "0.0.0.0", "Inbound listen address")

	if err := fs.Parse(args); err != nil {
		return err
	}

	ui := NewUI()
	ui.Header("veilshard install - Automated Production Deployment")

	// Must be root
	if os.Geteuid() != 0 && !*force {
		ui.Error("vpnctl install requires root privileges. Please run with: sudo vpnctl install")
		return fmt.Errorf("root required")
	}

	inst := installer.New()
	opts := installer.Options{
		Force:       *force,
		Port:        *port,
		DestSNI:     *dest,
		CoreVersion: *coreVer,
		ListenAddr:  *listen,
	}

	ctx := context.Background()
	err := inst.Install(ctx, opts, ui)
	if err != nil {
		ui.Error(fmt.Sprintf("Installation failed: %v", err))
		return err
	}

	fmt.Println("\nInstallation complete.")

	// Load subscription token and URL
	tokenPath := "/etc/vpnctl/sub_token"
	tokenVal := ""
	if data, err := os.ReadFile(tokenPath); err == nil {
		tokenVal = strings.TrimSpace(string(data))
	}

	expCtx, err := exporter.LoadExportContext("")
	if err == nil && expCtx != nil {
		subURL := fmt.Sprintf("http://%s:8080/sub/%s", expCtx.ServerIP, tokenVal)
		ui.Section("Universal Subscription Link (One-Click Import)")
		fmt.Printf("\n%sYour Ready-to-Use Subscription URL:%s\n%s%s%s\n\n", ColorBold, ColorReset, ColorCyan, subURL, ColorReset)
		fmt.Printf("• %sClash Verge / Mihomo%s: Directly paste this URL into New Subscription and Save!\n", ColorBold, ColorReset)
		fmt.Printf("• %sShadowrocket / v2rayN%s: Directly scan or paste this URL to sync nodes!\n\n", ColorBold, ColorReset)
		ui.Divider()
		fmt.Printf("Tips:\n")
		fmt.Printf("• Export Full Profile YAML:   %sveilshard export profile%s\n", ColorCyan, ColorReset)
		fmt.Printf("• Export Share URI:           %sveilshard export uri%s\n", ColorCyan, ColorReset)
		fmt.Printf("• Check system status:        %sveilshard status%s\n", ColorCyan, ColorReset)
		fmt.Printf("• Run diagnostics:            %sveilshard doctor%s\n\n", ColorCyan, ColorReset)
	}

	return nil
}
