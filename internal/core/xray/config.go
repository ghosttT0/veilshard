package xray

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/veilshard/veilshard/internal/config"
	"github.com/veilshard/veilshard/internal/credentials"
	"github.com/veilshard/veilshard/internal/users"
)

// APIPort is the loopback-only gRPC stats API port used for per-user traffic metering.
const APIPort = 15490

// XrayConfig matches Xray JSON configuration schema.
type XrayConfig struct {
	Log       LogConfig        `json:"log"`
	Stats     *StatsConfig     `json:"stats,omitempty"`
	Policy    *PolicyConfig    `json:"policy,omitempty"`
	API       *APIConfig       `json:"api,omitempty"`
	Routing   *RoutingConfig   `json:"routing,omitempty"`
	Inbounds  []InboundConfig  `json:"inbounds"`
	Outbounds []OutboundConfig `json:"outbounds"`
}

type StatsConfig struct{}

type PolicyConfig struct {
	Levels map[string]PolicyLevel `json:"levels"`
}

type PolicyLevel struct {
	StatsUserUplink   bool `json:"statsUserUplink"`
	StatsUserDownlink bool `json:"statsUserDownlink"`
}

type APIConfig struct {
	Tag      string   `json:"tag"`
	Services []string `json:"services"`
}

type RoutingConfig struct {
	Rules []RoutingRule `json:"rules"`
}

type RoutingRule struct {
	Type        string   `json:"type"`
	InboundTag  []string `json:"inboundTag"`
	OutboundTag string   `json:"outboundTag"`
}

type LogConfig struct {
	LogLevel string `json:"loglevel"`
	Access   string `json:"access"`
	Error    string `json:"error"`
}

type InboundConfig struct {
	Port           int                   `json:"port"`
	Listen         string                `json:"listen,omitempty"`
	Protocol       string                `json:"protocol"`
	Tag            string                `json:"tag,omitempty"`
	Settings       InboundSettings       `json:"settings"`
	StreamSettings *InboundStreamSettings `json:"streamSettings,omitempty"`
	Sniffing       *SniffingConfig       `json:"sniffing,omitempty"`
}

type InboundSettings struct {
	Clients    []ClientConfig `json:"clients,omitempty"`
	Decryption string         `json:"decryption,omitempty"`
	Address    string         `json:"address,omitempty"` // dokodemo-door follow target
}

type ClientConfig struct {
	ID    string `json:"id"`
	Flow  string `json:"flow"`
	Email string `json:"email,omitempty"`
}

type InboundStreamSettings struct {
	Network         string          `json:"network"`
	Security        string          `json:"security"`
	RealitySettings RealitySettings `json:"realitySettings"`
}

type RealitySettings struct {
	Show        bool     `json:"show"`
	Dest        string   `json:"dest"`
	Xver        int      `json:"xver"`
	ServerNames []string `json:"serverNames"`
	PrivateKey  string   `json:"privateKey"`
	ShortIds    []string `json:"shortIds"`
}

type SniffingConfig struct {
	Enabled      bool     `json:"enabled"`
	DestOverride []string `json:"destOverride"`
}

type OutboundConfig struct {
	Protocol string `json:"protocol"`
	Tag      string `json:"tag"`
}

// BuildXrayConfig constructs the struct for VLESS-REALITY-TCP.
func BuildXrayConfig(cfg *config.Config, userList []*users.User, creds *credentials.Credentials) *XrayConfig {
	var clients []ClientConfig
	for _, u := range userList {
		if u.Enabled {
			clients = append(clients, ClientConfig{
				ID:    u.UUID,
				Flow:  "xtls-rprx-vision",
				Email: u.Name,
			})
		}
	}

	// Fallback to credentials UUID if no user enabled
	if len(clients) == 0 && creds.UUID != "" {
		clients = append(clients, ClientConfig{
			ID:    creds.UUID,
			Flow:  "xtls-rprx-vision",
			Email: "default",
		})
	}

	dest := cfg.Protocol.Dest
	if dest == "" {
		dest = creds.Dest
	}
	if dest == "" {
		dest = "gateway.icloud.com:443"
	}

	serverNames := cfg.Protocol.ServerNames
	if len(serverNames) == 0 {
		sni := creds.ServerName
		if sni == "" {
			sni = strings.Split(dest, ":")[0]
		}
		serverNames = []string{sni}
	}

	shortIDs := []string{creds.ShortID}

	listenAddr := cfg.Server.Listen
	if listenAddr == "0.0.0.0" {
		listenAddr = ""
	}

	return &XrayConfig{
		Log: LogConfig{
			LogLevel: "warning",
			Access:   "",
			Error:    "",
		},
		// Per-user traffic metering: expose StatsService on loopback only.
		Stats: &StatsConfig{},
		Policy: &PolicyConfig{
			Levels: map[string]PolicyLevel{
				"0": {StatsUserUplink: true, StatsUserDownlink: true},
			},
		},
		API: &APIConfig{
			Tag:      "api",
			Services: []string{"StatsService"},
		},
		Routing: &RoutingConfig{
			Rules: []RoutingRule{
				{Type: "field", InboundTag: []string{"api"}, OutboundTag: "api"},
			},
		},
		Inbounds: []InboundConfig{
			{
				Port:     cfg.Server.Port,
				Listen:   listenAddr,
				Protocol: "vless",
				Settings: InboundSettings{
					Clients:    clients,
					Decryption: "none",
				},
				StreamSettings: &InboundStreamSettings{
					Network:  "tcp",
					Security: "reality",
					RealitySettings: RealitySettings{
						Show:        false,
						Dest:        dest,
						Xver:        0,
						ServerNames: serverNames,
						PrivateKey:  creds.PrivateKey,
						ShortIds:    shortIDs,
					},
				},
				Sniffing: &SniffingConfig{
					Enabled:      true,
					DestOverride: []string{"http", "tls", "quic"},
				},
			},
			{
				Port:     APIPort,
				Listen:   "127.0.0.1",
				Protocol: "dokodemo-door",
				Tag:      "api",
				Settings: InboundSettings{
					Address: "127.0.0.1",
				},
			},
		},
		Outbounds: []OutboundConfig{
			{Protocol: "freedom", Tag: "direct"},
			{Protocol: "blackhole", Tag: "block"},
		},
	}
}

// WriteConfigFile serializes the configuration and sets secure permissions (0644, vpnctl-proxy).
func WriteConfigFile(targetPath string, xc *XrayConfig, runAsUser string) error {
	dir := filepath.Dir(targetPath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("mkdir %s: %w", dir, err)
	}
	_ = os.Chmod(dir, 0755)

	data, err := json.MarshalIndent(xc, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal xray config: %w", err)
	}

	tmpFile := fmt.Sprintf("%s.tmp.%d", targetPath, os.Getpid())
	if err := os.WriteFile(tmpFile, data, 0644); err != nil {
		return fmt.Errorf("write tmp xray config: %w", err)
	}

	if err := os.Rename(tmpFile, targetPath); err != nil {
		_ = os.Remove(tmpFile)
		return fmt.Errorf("atomic rename xray config: %w", err)
	}

	_ = os.Chmod(targetPath, 0644)

	// Change ownership if possible
	if runAsUser != "" && runAsUser != "root" {
		_ = exec.Command("chown", fmt.Sprintf("%s:%s", runAsUser, runAsUser), targetPath).Run()
	}

	return nil
}
