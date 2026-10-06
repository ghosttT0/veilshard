// Package coresync regenerates the proxy core configuration from the current
// user store and restarts the systemd service so membership changes apply
// immediately. Shared by the CLI (vpnctl user ...) and the admin panel API.
package coresync

import (
	"context"
	"encoding/json"
	"fmt"
	"os"

	"github.com/veilshard/veilshard/internal/config"
	"github.com/veilshard/veilshard/internal/core/xray"
	"github.com/veilshard/veilshard/internal/credentials"
	"github.com/veilshard/veilshard/internal/service/systemd"
	"github.com/veilshard/veilshard/internal/users"
)

// Sync rebuilds /etc/vpnctl/core/config.json with all enabled users and
// restarts vpnctl-proxy.
func Sync() error {
	ctx := context.Background()

	cfg, err := config.Load("")
	if err != nil {
		return fmt.Errorf("load config: %w", err)
	}

	userStore, err := users.NewStore("")
	if err != nil {
		return fmt.Errorf("load users: %w", err)
	}

	credsData, err := os.ReadFile("/etc/vpnctl/credentials.json")
	if err != nil {
		return fmt.Errorf("read credentials: %w", err)
	}
	var creds credentials.Credentials
	if err := json.Unmarshal(credsData, &creds); err != nil {
		return fmt.Errorf("parse credentials: %w", err)
	}

	xr := xray.New("", "")
	if err := xr.GenerateConfig(ctx, cfg, userStore.ListEnabled(), &creds); err != nil {
		return fmt.Errorf("regenerate core config: %w", err)
	}

	svc := systemd.New("vpnctl-proxy")
	if err := svc.Restart(ctx); err != nil {
		return fmt.Errorf("restart vpnctl-proxy: %w", err)
	}

	return nil
}
