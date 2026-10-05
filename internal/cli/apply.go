package cli

import (
	"context"
	"flag"
	"fmt"
	"os"

	"github.com/veilshard/veilshard/internal/fleet"
)

func runApply(args []string) error {
	fs := flag.NewFlagSet("apply", flag.ContinueOnError)
	defaultFile := "veilshard.yaml"
	if _, err := os.Stat(defaultFile); os.IsNotExist(err) {
		if _, err2 := os.Stat("vpnctl.yaml"); err2 == nil {
			defaultFile = "vpnctl.yaml"
		}
	}
	file := fs.String("f", defaultFile, "Path to declarative fleet configuration file")

	if err := fs.Parse(args); err != nil {
		return err
	}

	ui := NewUI()
	ui.Header("veilshard apply - Declarative Fleet Orchestration")

	if _, err := os.Stat(*file); err != nil {
		ui.Error(fmt.Sprintf("Configuration file not found: %s", *file))
		fmt.Printf("Tip: Create a veilshard.yaml or vpnctl.yaml file declaring your fleet nodes.\n\n")
		return err
	}

	ctx := context.Background()
	cfg, err := fleet.ApplyFleet(ctx, *file, ui)
	if err != nil {
		ui.Error(fmt.Sprintf("Fleet deployment failed: %v", err))
		return err
	}

	ui.Success(fmt.Sprintf("Successfully orchestrated %d nodes!", len(cfg.Nodes)))
	fmt.Printf("\nReady-to-use profiles have been saved to:\n")
	fmt.Printf("• %s%s%s (Clash Meta / Mihomo with auto-test groups)\n", ColorCyan, cfg.Output.ClashMeta, ColorReset)
	fmt.Printf("• %s%s%s (sing-box client configuration)\n\n", ColorCyan, cfg.Output.SingBox, ColorReset)

	return nil
}
