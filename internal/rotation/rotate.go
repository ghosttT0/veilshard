package rotation

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"time"

	"github.com/veilshard/veilshard/internal/config"
	"github.com/veilshard/veilshard/internal/core/xray"
	"github.com/veilshard/veilshard/internal/credentials"
	"github.com/veilshard/veilshard/internal/probe"
	"github.com/veilshard/veilshard/internal/users"
)

// RotationResult holds the outcome of a credential and SNI rotation.
type RotationResult struct {
	NewShortID    string    `json:"new_short_id"`
	PreservedIDs  []string  `json:"preserved_ids"`
	OldSNI        string    `json:"old_sni"`
	NewSNI        string    `json:"new_sni"`
	RotatedAt     time.Time `json:"rotated_at"`
	ReloadSuccess bool      `json:"reload_success"`
}

// RotateLocalNode executes zero-downtime credential and SNI rotation on the local node.
func RotateLocalNode(ctx context.Context, autoSNI bool, customSNI string) (*RotationResult, error) {
	credsPath := "/etc/vpnctl/credentials.json"
	data, err := os.ReadFile(credsPath)
	if err != nil {
		return nil, fmt.Errorf("read credentials %s: %w", credsPath, err)
	}

	var creds credentials.Credentials
	if err := json.Unmarshal(data, &creds); err != nil {
		return nil, fmt.Errorf("parse credentials: %w", err)
	}

	// 1. Generate new 8-character ShortID
	newShortID, err := credentials.NewShortID(8)
	if err != nil {
		return nil, fmt.Errorf("generate new short id: %w", err)
	}

	oldShortID := creds.ShortID
	preserved := []string{newShortID}
	if oldShortID != "" && oldShortID != newShortID {
		preserved = append(preserved, oldShortID)
	}

	// 2. Select or probe SNI target
	oldSNI := creds.ServerName
	newSNI := oldSNI

	if customSNI != "" {
		newSNI = customSNI
	} else if autoSNI {
		best, err := probe.AutoSelectBestSNI(3 * time.Second)
		if err == nil && best != nil {
			newSNI = best.ServerName
		}
	}

	// 3. Update in-memory credentials & file
	creds.ShortID = newShortID
	creds.ServerName = newSNI
	creds.Dest = newSNI + ":443"

	updatedData, err := json.MarshalIndent(creds, "", "  ")
	if err != nil {
		return nil, fmt.Errorf("marshal updated creds: %w", err)
	}
	if err := os.WriteFile(credsPath, updatedData, 0600); err != nil {
		return nil, fmt.Errorf("write credentials: %w", err)
	}

	// 4. Update core config with multi-shortId grace period
	cfg, err := config.Load("")
	if err != nil {
		return nil, fmt.Errorf("load config: %w", err)
	}
	cfg.Protocol.Dest = creds.Dest
	cfg.Protocol.ServerNames = []string{newSNI}
	_ = cfg.Save("")

	userStore, _ := users.NewStore("")
	userList := userStore.List()

	// Build Xray config with multi-shortId list for zero-downtime transition
	xc := xray.BuildXrayConfig(cfg, userList, &creds)
	if len(xc.Inbounds) > 0 {
		xc.Inbounds[0].StreamSettings.RealitySettings.ShortIds = preserved
	}

	coreCfgPath := "/etc/vpnctl/core/config.json"
	if err := xray.WriteConfigFile(coreCfgPath, xc, cfg.Security.RunAsUser); err != nil {
		return nil, fmt.Errorf("write core config: %w", err)
	}

	// 5. Zero-downtime graceful reload
	reloadOK := gracefulReload(ctx)

	return &RotationResult{
		NewShortID:    newShortID,
		PreservedIDs:  preserved,
		OldSNI:        oldSNI,
		NewSNI:        newSNI,
		RotatedAt:     time.Now().UTC(),
		ReloadSuccess: reloadOK,
	}, nil
}

// gracefulReload sends SIGHUP or systemctl reload without terminating active sockets.
func gracefulReload(ctx context.Context) bool {
	// Try systemctl reload vpnctl-proxy
	cmd := exec.CommandContext(ctx, "systemctl", "reload", "vpnctl-proxy")
	if err := cmd.Run(); err == nil {
		return true
	}

	// Fallback to sending SIGHUP directly to Xray main PID
	pidCmd := exec.CommandContext(ctx, "systemctl", "show", "vpnctl-proxy", "--property=MainPID")
	out, err := pidCmd.Output()
	if err == nil {
		var pid int
		if _, err := fmt.Sscanf(string(out), "MainPID=%d", &pid); err == nil && pid > 0 {
			if err := exec.CommandContext(ctx, "kill", "-HUP", fmt.Sprintf("%d", pid)).Run(); err == nil {
				return true
			}
		}
	}

	// Final fallback: restart service
	return exec.CommandContext(ctx, "systemctl", "restart", "vpnctl-proxy").Run() == nil
}
