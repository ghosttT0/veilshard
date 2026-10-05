package cli

import (
	"context"
	"flag"
	"fmt"
	"os"

	"github.com/veilshard/veilshard/internal/updater"
)

func runUpdate(args []string) error {
	fs := flag.NewFlagSet("update", flag.ContinueOnError)
	version := fs.String("version", "", "Specific core version to upgrade/downgrade to (default: latest stable)")

	if err := fs.Parse(args); err != nil {
		return err
	}

	ui := NewUI()
	ui.Header("vpnctl update")

	if os.Geteuid() != 0 {
		ui.Error("vpnctl update requires root privileges (sudo)")
		return fmt.Errorf("root required")
	}

	upd := updater.New()
	ctx := context.Background()

	err := upd.UpdateCore(ctx, *version, ui)
	if err != nil {
		ui.Error(fmt.Sprintf("Update failed: %v", err))
		return err
	}

	return nil
}
