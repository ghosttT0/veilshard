package cli

import (
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/veilshard/veilshard/internal/config"
	"github.com/veilshard/veilshard/internal/coresync"
	"github.com/veilshard/veilshard/internal/traffic"
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
			ui.Error("Usage: vpnctl user add <username> [uuid] [--expire 7d|24h] [--quota 100GB|0]")
			return fmt.Errorf("username required")
		}
		username := args[1]
		customUUID := ""
		if len(args) > 2 && !strings.HasPrefix(args[2], "-") {
			customUUID = args[2]
		}

		var expireDuration time.Duration
		var quotaBytes int64
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
			if (a == "--quota" || a == "-quota") && i+1 < len(args) {
				quotaBytes, _ = parseTrafficLimit(args[i+1])
			}
		}

		u, err := store.AddWithExpiry(username, customUUID, expireDuration)
		if err != nil {
			ui.Error(fmt.Sprintf("Failed to add user %s: %v", username, err))
			return err
		}
		if quotaBytes > 0 {
			if _, err := store.SetQuotaBytes(username, quotaBytes); err != nil {
				ui.Error(fmt.Sprintf("Failed to set quota: %v", err))
				return err
			}
		}
		expDetail := "never"
		if u.ExpiresAt != nil {
			expDetail = u.ExpiresAt.Format("2006-01-02 15:04 UTC")
		}
		ui.Success(fmt.Sprintf("Added user '%s' (UUID: %s, Expires: %s, Quota: %s)", u.Name, u.UUID, expDetail, traffic.Human(quotaBytes)))
		ui.KeyValue("Subscription URL", fmt.Sprintf("http://%s:8080/sub/%s", subURLHost(), u.SubToken))

		// Apply changes to proxy core
		_ = syncCoreConfig(ui)
		return nil

	case "token":
		if len(args) < 2 {
			ui.Error("Usage: vpnctl user token <username> [--port 8080]")
			return fmt.Errorf("username required")
		}
		port := 8080
		for i, a := range args {
			if (a == "--port" || a == "-port") && i+1 < len(args) {
				port, _ = strconv.Atoi(args[i+1])
			}
		}
		u, err := store.Get(args[1])
		if err != nil {
			ui.Error(fmt.Sprintf("User not found: %v", err))
			return err
		}
		ui.KeyValue(fmt.Sprintf("Subscription URL for '%s'", u.Name), fmt.Sprintf("http://%s:%d/sub/%s", subURLHost(), port, u.SubToken))
		return nil

	case "reset-token":
		if len(args) < 2 {
			ui.Error("Usage: vpnctl user reset-token <username>")
			return fmt.Errorf("username required")
		}
		u, err := store.ResetSubToken(args[1])
		if err != nil {
			ui.Error(fmt.Sprintf("Failed to reset token: %v", err))
			return err
		}
		ui.Success(fmt.Sprintf("Subscription link rotated for '%s'", u.Name))
		ui.KeyValue("New Subscription URL", fmt.Sprintf("http://%s:8080/sub/%s", subURLHost(), u.SubToken))
		return nil

	case "quota":
		if len(args) < 3 {
			ui.Error("Usage: vpnctl user quota <username> <100GB|1TB|0>   (0 = unlimited)")
			return fmt.Errorf("username and limit required")
		}
		limit, err := parseTrafficLimit(args[2])
		if err != nil {
			ui.Error(fmt.Sprintf("Invalid limit: %v", err))
			return err
		}
		u, err := store.SetQuotaBytes(args[1], limit)
		if err != nil {
			ui.Error(fmt.Sprintf("Failed to set quota: %v", err))
			return err
		}
		ui.Success(fmt.Sprintf("Quota for '%s' set to %s", u.Name, traffic.Human(u.QuotaBytes)))
		return nil

	case "traffic":
		return showTraffic(ui, store)

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
	fmt.Printf("  add <name> [uuid] [--expire 7d] [--quota 100GB] - Add a user with optional expiry and traffic quota\n")
	fmt.Printf("  token <name> [--port 8080]       - Print the user's personal subscription URL\n")
	fmt.Printf("  reset-token <name>               - Rotate the user's subscription link (old link dies)\n")
	fmt.Printf("  quota <name> <100GB|1TB|0>       - Set per-user traffic quota (0 = unlimited)\n")
	fmt.Printf("  traffic                          - Show live per-user traffic counters\n")
	fmt.Printf("  revoke <name>                    - Instantly revoke credentials and sever active socket\n")
	fmt.Printf("  remove <name>                    - Remove an existing proxy user\n")
	fmt.Printf("  enable <name>                    - Enable an existing proxy user\n")
	fmt.Printf("  disable <name>                   - Disable an existing proxy user\n\n")
	fmt.Printf("Tip: open the web panel at http://<server>:8080/panel for point-and-click management.\n\n")
}

// parseTrafficLimit accepts "500MB", "100GB", "1TB", raw bytes, or "0" (unlimited).
func parseTrafficLimit(s string) (int64, error) {
	s = strings.TrimSpace(strings.ToUpper(s))
	if s == "" || s == "0" {
		return 0, nil
	}
	mult := int64(1)
	num := s
	switch {
	case strings.HasSuffix(s, "KB"):
		mult, num = 1024, strings.TrimSuffix(s, "KB")
	case strings.HasSuffix(s, "MB"):
		mult, num = 1024*1024, strings.TrimSuffix(s, "MB")
	case strings.HasSuffix(s, "GB"):
		mult, num = 1024*1024*1024, strings.TrimSuffix(s, "GB")
	case strings.HasSuffix(s, "TB"):
		mult, num = 1024*1024*1024*1024, strings.TrimSuffix(s, "TB")
	}
	v, err := strconv.ParseFloat(num, 64)
	if err != nil || v < 0 {
		return 0, fmt.Errorf("cannot parse traffic limit %q", s)
	}
	return int64(v * float64(mult)), nil
}

// subURLHost returns the public address to build subscription links against.
func subURLHost() string {
	if cfg, err := config.Load(""); err == nil && cfg.Server.PublicIP != "" {
		return cfg.Server.PublicIP
	}
	return "127.0.0.1"
}

// showTraffic prints live per-user counters from the xray stats API.
func showTraffic(ui *TerminalUI, store *users.Store) error {
	snap, err := traffic.Query()
	if err != nil {
		ui.Error(fmt.Sprintf("Traffic query failed (stats API needs one core sync after upgrade): %v", err))
		return err
	}

	headers := []string{"NAME", "UPLINK", "DOWNLINK", "TOTAL", "QUOTA", "LEFT"}
	list := store.List()
	rows := make([][]string, 0, len(list))
	for _, u := range list {
		usage, _ := traffic.ForUser(snap, u.Name)
		left := "∞"
		if u.QuotaBytes > 0 {
			rem := u.QuotaBytes - usage.Total()
			if rem < 0 {
				rem = 0
			}
			left = traffic.Human(rem)
		}
		rows = append(rows, []string{
			u.Name,
			traffic.Human(usage.Up),
			traffic.Human(usage.Down),
			traffic.Human(usage.Total()),
			traffic.Human(u.QuotaBytes),
			left,
		})
	}
	fmt.Println()
	ui.Table(headers, rows)
	fmt.Println()
	return nil
}

func syncCoreConfig(ui *TerminalUI) error {
	if err := coresync.Sync(); err != nil {
		ui.Warn(fmt.Sprintf("Failed to sync core config: %v", err))
		return err
	}
	ui.Success("Proxy configuration regenerated and service reloaded.")
	return nil
}
