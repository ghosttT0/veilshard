package cli

import (
	"context"
	"flag"
	"fmt"
	"os"

	"github.com/veilshard/veilshard/internal/exporter"
	"github.com/veilshard/veilshard/internal/exporter/mihomo"
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

	// Export Mihomo snippet
	expCtx, err := exporter.LoadExportContext("")
	if err == nil && expCtx != nil {
		ui.Section("Client Configuration (Mihomo / Clash Meta)")
		fmt.Println(mihomo.GenerateYAML(expCtx))
		ui.Divider()
		fmt.Printf("Tips:\n")
		fmt.Printf("• Export Mihomo config again: %svpnctl export mihomo%s\n", ColorCyan, ColorReset)
		fmt.Printf("• Export Share URI:          %svpnctl export uri%s\n", ColorCyan, ColorReset)
		fmt.Printf("• Export Terminal QR code:    %svpnctl export qr%s\n", ColorCyan, ColorReset)
		fmt.Printf("• Check system status:        %svpnctl status%s\n", ColorCyan, ColorReset)
		fmt.Printf("• Run diagnostics:            %svpnctl doctor%s\n\n", ColorCyan, ColorReset)
	}

	return nil
}
