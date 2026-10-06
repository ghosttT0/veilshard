// Package traffic queries the xray StatsService for per-user traffic counters.
// Counters are cumulative since the last core restart; the raw numbers are
// meant for display in subscription headers and the admin panel.
package traffic

import (
	"context"
	"encoding/json"
	"fmt"
	"os/exec"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/veilshard/veilshard/internal/config"
	"github.com/veilshard/veilshard/internal/core/xray"
)

// Usage holds cumulative byte counters for one user (email in xray stats).
type Usage struct {
	Up   int64 `json:"up"`
	Down int64 `json:"down"`
}

// Total returns uplink plus downlink bytes.
func (u Usage) Total() int64 { return u.Up + u.Down }

// Snapshot is a point-in-time aggregate of all user counters.
type Snapshot struct {
	At    time.Time        `json:"at"`
	Users map[string]Usage `json:"users"`
}

var (
	mu      sync.Mutex
	cache   *Snapshot
	fetched time.Time
)

const cacheTTL = 10 * time.Second

// Query returns a cached snapshot of per-user counters, refreshing it via the
// xray CLI when older than the cache TTL. Stats are read over the loopback
// gRPC API that the generated core config exposes on 127.0.0.1.
func Query() (*Snapshot, error) {
	mu.Lock()
	defer mu.Unlock()

	if cache != nil && time.Since(fetched) < cacheTTL {
		return cache, nil
	}

	bin := BinaryPath()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	out, err := exec.CommandContext(ctx, bin, "api", "statsquery",
		"--server", fmt.Sprintf("127.0.0.1:%d", xray.APIPort)).Output()
	if err != nil {
		return nil, fmt.Errorf("xray statsquery: %w", err)
	}

	snap, err := parseStatsQuery(out)
	if err != nil {
		return nil, err
	}

	cache = snap
	fetched = time.Now()
	return cache, nil
}

// ForUser looks up one user's usage in a snapshot.
func ForUser(snap *Snapshot, name string) (Usage, bool) {
	if snap == nil {
		return Usage{}, false
	}
	u, ok := snap.Users[name]
	return u, ok
}

// BinaryPath resolves the xray binary location from config with safe fallbacks.
func BinaryPath() string {
	if cfg, err := config.Load(""); err == nil && cfg.Core.BinaryPath != "" {
		return cfg.Core.BinaryPath
	}
	if path, err := exec.LookPath("xray"); err == nil {
		return path
	}
	return "/usr/local/bin/xray"
}

func parseStatsQuery(out []byte) (*Snapshot, error) {
	var payload struct {
		Stat []struct {
			Name  string      `json:"name"`
			Value json.Number `json:"value"` // xray emits the counter as a JSON number
		} `json:"stat"`
	}
	if err := json.Unmarshal(out, &payload); err != nil {
		return nil, fmt.Errorf("parse statsquery output: %w", err)
	}

	snap := &Snapshot{At: time.Now(), Users: make(map[string]Usage)}
	for _, st := range payload.Stat {
		// Counter names look like: user>>><email>>>traffic>>>uplink|downlink
		parts := strings.Split(st.Name, ">>>")
		if len(parts) != 4 || parts[0] != "user" || parts[2] != "traffic" {
			continue
		}
		raw := strings.TrimSpace(st.Value.String())
		val, err := strconv.ParseInt(raw, 10, 64)
		if err != nil {
			f, ferr := strconv.ParseFloat(raw, 64)
			if ferr != nil {
				continue
			}
			val = int64(f)
		}
		name := parts[1]
		usage := snap.Users[name]
		switch parts[3] {
		case "uplink":
			usage.Up = val
		case "downlink":
			usage.Down = val
		default:
			continue
		}
		snap.Users[name] = usage
	}
	return snap, nil
}

// Human renders a byte count in human-readable units.
func Human(b int64) string {
	const unit = 1024
	if b < unit {
		return fmt.Sprintf("%d B", b)
	}
	div, exp := int64(unit), 0
	for n := b / unit; n >= unit; n /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.2f %cB", float64(b)/float64(div), "KMGTPE"[exp])
}
