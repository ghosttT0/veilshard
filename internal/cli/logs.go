package cli

import (
	"context"
	"flag"
	"fmt"
	"io"
	"os"

	"github.com/veilshard/veilshard/internal/service/systemd"
)

func runLogs(args []string) error {
	fs := flag.NewFlagSet("logs", flag.ContinueOnError)
	lines := fs.Int("n", 50, "Number of log lines to show")
	follow := fs.Bool("f", false, "Follow log output")

	if err := fs.Parse(args); err != nil {
		return err
	}

	svc := systemd.New("vpnctl-proxy")
	ctx := context.Background()

	if *follow {
		reader, err := svc.FollowLogs(ctx, *lines)
		if err != nil {
			return fmt.Errorf("follow logs: %w", err)
		}
		defer reader.Close()
		_, _ = io.Copy(os.Stdout, reader)
		return nil
	}

	out, err := svc.GetLogs(ctx, *lines)
	if err != nil {
		return fmt.Errorf("get logs: %w", err)
	}
	fmt.Print(out)
	return nil
}
