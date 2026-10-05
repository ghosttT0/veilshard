package cli

import (
	"context"
	"flag"
	"fmt"

	"github.com/veilshard/veilshard/internal/defense"
)

func runDefense(args []string) error {
	fs := flag.NewFlagSet("defense", flag.ContinueOnError)
	port := fs.Int("port", 443, "Target proxy port to protect")
	disable := fs.Bool("disable", false, "Disable active probing tarpit and deception rules")

	if err := fs.Parse(args); err != nil {
		return err
	}

	ui := NewUI()
	ui.Header("vpnctl defense - Active Probing Tarpit & Deception Engine")

	ctx := context.Background()

	if *disable {
		if err := defense.DisableTarpitDefense(ctx, *port); err != nil {
			ui.Error(fmt.Sprintf("Failed to disable defense: %v", err))
			return err
		}
		ui.Success(fmt.Sprintf("Tarpit defense rules disabled on port %d", *port))
		return nil
	}

	if err := defense.EnableTarpitDefense(ctx, *port); err != nil {
		ui.Error(fmt.Sprintf("Failed to enable defense: %v", err))
		return err
	}

	status, _ := defense.GetDefenseStatus(ctx, *port)

	ui.Success(fmt.Sprintf("Kernel-level tarpit & deception active on port %d!", *port))
	fmt.Println()
	ui.KeyValue("Connection State Scrubbing", "DROP INVALID")
	ui.KeyValue("Port Scanner Defenses", "DROP NULL/XMAS/SYN+FIN")
	ui.KeyValue("SYN Flood & Burst Limiting", "15/sec (mode: srcip)")
	ui.KeyValue("TCP Window / MSS Clamping", "MSS 1200 / Slow-Window")
	if status != nil {
		ui.KeyValue("Total Dropped Scans", fmt.Sprintf("%d packets", status.DroppedProbes))
	}

	fmt.Printf("\n%sDeception Impact:%s\n", ColorCyan, ColorReset)
	fmt.Println("• Active GFW probe nodes exhaust socket handles waiting for slow responses.")
	fmt.Println("• Scanners mark your VPS as an unresponsive or dead node, reducing deep audit priority.")
	fmt.Println("• Real camouflage target is shielded from rate-limiting bans caused by malicious probe floods.")
	fmt.Println()

	return nil
}
