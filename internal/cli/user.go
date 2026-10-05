package cli

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/veilshard/veilshard/internal/config"
	"github.com/veilshard/veilshard/internal/core/xray"
	"github.com/veilshard/veilshard/internal/credentials"
	"github.com/veilshard/veilshard/internal/service/systemd"
	"github.com/veilshard/veilshard/internal/users"
)

func runUser(args []string) error {
	ui := NewUI()
	store, err := users.NewStore("")
	if err != nil {
		ui.Error(fmt.Sprintf("Failed to load users: %v", err))
		return err
	}

	if len(args) == 0 {
		printUserHelp()
		return nil
	}

	action := strings.ToLower(args[0])

	switch action {
	case "list", "ls":
		list := store.List()
		if len(list) == 0 {
			fmt.Println("No users configured.")
			return nil
		}
		headers := []string{"NAME", "STATUS", "EXPIRES", "UUID", "CREATED"}
		rows := make([][]string, len(list))
		for i, u := range list {
			st := "active"
			if u.IsRevoked {
				st = fmt.Sprintf("%srevoked%s", ColorRed, ColorReset)
			} else if !u.Enabled {
				st = fmt.Sprintf("%sdisabled%s", ColorYellow, ColorReset)
			} else if u.ExpiresAt != nil && time.Now().After(*u.ExpiresAt) {
				st = fmt.Sprintf("%sexpired%s", ColorRed, ColorReset)
			} else {
				st = fmt.Sprintf("%sactive%s", ColorGreen, ColorReset)
			}

			expStr := "never"
			if u.ExpiresAt != nil {
				rem := time.Until(*u.ExpiresAt)
				if rem > 0 {
					expStr = fmt.Sprintf("%dd %dh", int(rem.Hours()/24), int(rem.Hours())%24)
				} else {
					expStr = "expired"
				}
			}

			rows[i] = []string{
				u.Name,
				st,
				expStr,
				u.UUID,
				u.CreatedAt.Format("2006-01-02 15:04"),
			}
		}
		fmt.Println()
		ui.Table(headers, rows)
		fmt.Println()
		return nil

	case "add":
		if len(args) < 2 {
			ui.Error("Usage: vpnctl user add <username> [uuid] [--expire 7d|24h]")
			return fmt.Errorf("username required")
		}
		username := args[1]
		customUUID := ""
		if len(args) > 2 && !strings.HasPrefix(args[2], "-") {
			customUUID = args[2]
		}

		var expireDuration time.Duration
		for i, a := range args {
			if (a == "--expire" || a == "-expire") && i+1 < len(args) {
				expStr := args[i+1]
				if strings.HasSuffix(expStr, "d") {
					var days int
					_, _ = fmt.Sscanf(strings.TrimSuffix(expStr, "d"), "%d", &days)
					expireDuration = time.Duration(days) * 24 * time.Hour
				} else {
					expireDuration, _ = time.ParseDuration(expStr)
				}
			}
		}

		u, err := store.AddWithExpiry(username, customUUID, expireDuration)
		if err != nil {
			ui.Error(fmt.Sprintf("Failed to add user %s: %v", username, err))
			return err
		}
		expDetail := "never"
		if u.ExpiresAt != nil {
			expDetail = u.ExpiresAt.Format("2006-01-02 15:04 UTC")
		}
		ui.Success(fmt.Sprintf("Added user '%s' (UUID: %s, Expires: %s)", u.Name, u.UUID, expDetail))

		// Apply changes to proxy core
		_ = syncCoreConfig(ui)
		return nil

	case "revoke":
		if len(args) < 2 {
			ui.Error("Usage: vpnctl user revoke <username>")
			return fmt.Errorf("username required")
		}
		username := args[1]
		_, err := store.Revoke(username)
		if err != nil {
			ui.Error(fmt.Sprintf("Failed to revoke user: %v", err))
			return err
		}
		ui.Success(fmt.Sprintf("User '%s' credentials revoked immediately", username))
		_ = syncCoreConfig(ui)
		return nil

	case "remove", "rm", "del":
		if len(args) < 2 {
			ui.Error("Usage: vpnctl user remove <username>")
			return fmt.Errorf("username required")
		}
		username := args[1]
		err := store.Remove(username)
		if err != nil {
			ui.Error(fmt.Sprintf("Failed to remove user: %v", err))
			return err
		}
		ui.Success(fmt.Sprintf("Removed user '%s'", username))

		// Apply changes to proxy core
		_ = syncCoreConfig(ui)
		return nil

	case "enable":
		if len(args) < 2 {
			ui.Error("Usage: vpnctl user enable <username>")
			return fmt.Errorf("username required")
		}
		username := args[1]
		_, err := store.SetEnabled(username, true)
		if err != nil {
			ui.Error(fmt.Sprintf("Failed to enable user: %v", err))
			return err
		}
		ui.Success(fmt.Sprintf("User '%s' is now enabled", username))
		_ = syncCoreConfig(ui)
		return nil

	case "disable":
		if len(args) < 2 {
			ui.Error("Usage: vpnctl user disable <username>")
			return fmt.Errorf("username required")
		}
		username := args[1]
		_, err := store.SetEnabled(username, false)
		if err != nil {
			ui.Error(fmt.Sprintf("Failed to disable user: %v", err))
			return err
		}
		ui.Success(fmt.Sprintf("User '%s' is now disabled", username))
		_ = syncCoreConfig(ui)
		return nil

	default:
		printUserHelp()
		return nil
	}
}

func printUserHelp() {
	fmt.Printf("Usage: vpnctl user <subcommand> [arguments]\n\n")
	fmt.Printf("Subcommands:\n")
	fmt.Printf("  list                             - List all proxy users (active/revoked/expires)\n")
	fmt.Printf("  add <name> [uuid] [--expire 7d]  - Add a permanent or ephemeral proxy user\n")
	fmt.Printf("  revoke <name>                    - Instantly revoke credentials and sever active socket\n")
	fmt.Printf("  remove <name>                    - Remove an existing proxy user\n")
	fmt.Printf("  enable <name>                    - Enable an existing proxy user\n")
	fmt.Printf("  disable <name>                   - Disable an existing proxy user\n\n")
}

func syncCoreConfig(ui *TerminalUI) error {
	ctx := context.Background()
	cfg, err := config.Load("")
	if err != nil {
		return err
	}

	userStore, err := users.NewStore("")
	if err != nil {
		return err
	}

	credsData, err := os.ReadFile("/etc/vpnctl/credentials.json")
	if err != nil {
		return err
	}
	var creds credentials.Credentials
	_ = json.Unmarshal(credsData, &creds)

	xr := xray.New("", "")
	err = xr.GenerateConfig(ctx, cfg, userStore.ListEnabled(), &creds)
	if err != nil {
		ui.Warn(fmt.Sprintf("Failed to regenerate core config: %v", err))
		return err
	}

	svc := systemd.New("vpnctl-proxy")
	err = svc.Restart(ctx)
	if err != nil {
		ui.Warn(fmt.Sprintf("Failed to reload service: %v", err))
		return err
	}

	ui.Success("Proxy configuration regenerated and service reloaded.")
	return nil
}
