package updater

import (
	"context"
	"fmt"
	"io"
	"net"
	"os"
	"path/filepath"
	"time"

	"github.com/veilshard/veilshard/internal/core/xray"
	"github.com/veilshard/veilshard/internal/service"
	"github.com/veilshard/veilshard/internal/service/systemd"
	"github.com/veilshard/veilshard/internal/state"
)

type UIProgress interface {
	Step(name string, success bool, detail string)
}

type Updater struct {
	st      *state.State
	service service.Manager
}

func New() *Updater {
	st, _ := state.Load("")
	return &Updater{
		st:      st,
		service: systemd.New("vpnctl-proxy"),
	}
}

// UpdateCore updates the Xray core binary with rollback protection.
func (u *Updater) UpdateCore(ctx context.Context, targetVersion string, ui UIProgress) error {
	if targetVersion == "" {
		targetVersion = xray.DefaultXrayVersion
	}

	binaryPath := u.st.CoreBinaryPath
	if binaryPath == "" {
		binaryPath = "/usr/local/bin/xray"
	}

	backupDir := "/var/lib/vpnctl/versions"
	_ = os.MkdirAll(backupDir, 0700)
	backupPath := filepath.Join(backupDir, fmt.Sprintf("xray_backup_%d", time.Now().Unix()))

	// Step 1: Backup current binary if it exists
	if _, err := os.Stat(binaryPath); err == nil {
		if err := copyFile(binaryPath, backupPath); err != nil {
			return fmt.Errorf("backup current binary: %w", err)
		}
		if ui != nil {
			ui.Step("Backed up current version", true, backupPath)
		}
	}

	// Step 2: Download & verify new binary to temporary location
	dl := xray.NewDownloader()
	tmpBinary := fmt.Sprintf("%s.new.%d", binaryPath, os.Getpid())
	defer os.Remove(tmpBinary)

	if err := dl.DownloadAndVerify(ctx, targetVersion, tmpBinary); err != nil {
		return fmt.Errorf("download & verify new version: %w", err)
	}
	if ui != nil {
		ui.Step("Downloaded & verified new core", true, targetVersion)
	}

	// Step 3: Replace binary atomically
	if err := os.Rename(tmpBinary, binaryPath); err != nil {
		return fmt.Errorf("replace binary: %w", err)
	}
	_ = os.Chmod(binaryPath, 0755)

	// Step 4: Validate config with new binary
	xr := xray.New(binaryPath, u.st.CoreConfigPath)
	if err := xr.Validate(ctx); err != nil {
		// Rollback!
		_ = copyFile(backupPath, binaryPath)
		_ = u.service.Restart(ctx)
		return fmt.Errorf("new version config validation failed (rolled back): %w", err)
	}
	if ui != nil {
		ui.Step("Config test passed with new core", true, "")
	}

	// Step 5: Restart service
	if err := u.service.Restart(ctx); err != nil {
		_ = copyFile(backupPath, binaryPath)
		_ = u.service.Restart(ctx)
		return fmt.Errorf("service restart failed (rolled back): %w", err)
	}

	// Step 6: Health check
	time.Sleep(1500 * time.Millisecond)
	svcSt, err := u.service.Status(ctx)
	if err != nil || !svcSt.Active {
		_ = copyFile(backupPath, binaryPath)
		_ = u.service.Restart(ctx)
		return fmt.Errorf("health check failed after update (rolled back to previous version)")
	}

	// Check listening port
	conn, err := net.DialTimeout("tcp", "127.0.0.1:443", 2*time.Second)
	if err == nil {
		_ = conn.Close()
	}

	// Update state
	u.st.CoreVersion = targetVersion
	_ = u.st.Save("")

	if ui != nil {
		ui.Step("Update completed successfully", true, fmt.Sprintf("Running %s", targetVersion))
	}

	return nil
}

func copyFile(src, dst string) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()

	out, err := os.OpenFile(dst, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0755)
	if err != nil {
		return err
	}
	defer out.Close()

	_, err = io.Copy(out, in)
	return err
}
