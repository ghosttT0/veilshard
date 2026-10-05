package cli

import (
	"context"
	"flag"
	"fmt"
	"strings"

	"github.com/veilshard/veilshard/internal/rotation"
)

func runRotate(args []string) error {
	fs := flag.NewFlagSet("rotate", flag.ContinueOnError)
	autoSNI := fs.Bool("auto-sni", true, "Automatically benchmark and pick the fastest TLS 1.3 SNI target")
	sni := fs.String("sni", "", "Custom SNI target to rotate to")
	_ = fs.Parse(args)

	ui := NewUI()
	ui.Header("veilshard rotate - Zero-Downtime Credential & SNI Rotation")

	ctx := context.Background()
	res, err := rotation.RotateLocalNode(ctx, *autoSNI, *sni)
	if err != nil {
		ui.Error(fmt.Sprintf("Rotation failed: %v", err))
		return err
	}

	ui.Success("Zero-downtime rotation completed!")
	fmt.Println()
	ui.KeyValue("New ShortID", res.NewShortID)
	ui.KeyValue("Preserved IDs (Grace Period)", strings.Join(res.PreservedIDs, ", "))
	ui.KeyValue("Previous SNI", res.OldSNI)
	ui.KeyValue("New Camouflage SNI", res.NewSNI)
	ui.KeyValue("Graceful Core Reload", fmt.Sprintf("%v", res.ReloadSuccess))

	fmt.Printf("\n%sTip: Client configurations have a grace period to transition without dropping connections.%s\n", ColorCyan, ColorReset)
	fmt.Printf("Export your updated client profile with: %svpnctl export profile%s\n\n", ColorBold, ColorReset)

	return nil
}
