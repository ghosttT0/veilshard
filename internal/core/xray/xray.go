package xray

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"strings"

	"github.com/veilshard/veilshard/internal/config"
	"github.com/veilshard/veilshard/internal/core"
	"github.com/veilshard/veilshard/internal/credentials"
	"github.com/veilshard/veilshard/internal/users"
)

type CoreImpl struct {
	binaryPath string
	configPath string
	downloader *Downloader
}

func New(binaryPath, configPath string) core.Core {
	if binaryPath == "" {
		binaryPath = "/usr/local/bin/xray"
	}
	if configPath == "" {
		configPath = "/etc/vpnctl/core/config.json"
	}
	return &CoreImpl{
		binaryPath: binaryPath,
		configPath: configPath,
		downloader: NewDownloader(),
	}
}

func (x *CoreImpl) Name() string {
	return "xray"
}

func (x *CoreImpl) BinaryPath() string {
	return x.binaryPath
}

func (x *CoreImpl) ConfigPath() string {
	return x.configPath
}

func (x *CoreImpl) Install(ctx context.Context, version string) error {
	return x.downloader.DownloadAndVerify(ctx, version, x.binaryPath)
}

func (x *CoreImpl) GenerateConfig(ctx context.Context, cfg *config.Config, userList []*users.User, creds *credentials.Credentials) error {
	xc := BuildXrayConfig(cfg, userList, creds)
	return WriteConfigFile(x.configPath, xc, cfg.Security.RunAsUser)
}

func (x *CoreImpl) Validate(ctx context.Context) error {
	if _, err := os.Stat(x.binaryPath); err != nil {
		return fmt.Errorf("xray binary not found at %s: %w", x.binaryPath, err)
	}
	if _, err := os.Stat(x.configPath); err != nil {
		return fmt.Errorf("xray config not found at %s: %w", x.configPath, err)
	}

	cmd := exec.CommandContext(ctx, x.binaryPath, "run", "-test", "-config", x.configPath)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("configuration test failed: %w (%s)", err, strings.TrimSpace(string(out)))
	}
	return nil
}

func (x *CoreImpl) Version(ctx context.Context) (string, error) {
	if _, err := os.Stat(x.binaryPath); err != nil {
		return "", fmt.Errorf("xray binary not found: %w", err)
	}

	cmd := exec.CommandContext(ctx, x.binaryPath, "version")
	out, err := cmd.Output()
	if err != nil {
		return "", fmt.Errorf("check version: %w", err)
	}

	lines := strings.Split(string(out), "\n")
	if len(lines) > 0 {
		// First line is like: Xray 25.1.30 (Xray, Penetrates Everything.) Custom ...
		return strings.TrimSpace(lines[0]), nil
	}
	return "unknown", nil
}

func (x *CoreImpl) Status(ctx context.Context) (*core.CoreStatus, error) {
	st := &core.CoreStatus{
		Running: false,
	}

	v, err := x.Version(ctx)
	if err == nil {
		st.Version = v
	}

	return st, nil
}
