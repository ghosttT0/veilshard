package fleet

import (
	"bufio"
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

type FleetConfig struct {
	Version string       `yaml:"version" json:"version"`
	Nodes   []NodeConfig `yaml:"nodes" json:"nodes"`
	Output  OutputConfig `yaml:"output" json:"output"`
}

type NodeConfig struct {
	Name         string    `yaml:"name" json:"name"`
	Role         string    `yaml:"role" json:"role"` // "exit" or "relay" (default: "exit")
	Host         string    `yaml:"host" json:"host"`
	SSH          SSHConfig `yaml:"ssh" json:"ssh"`
	Protocol     string    `yaml:"protocol" json:"protocol"`
	Port         int       `yaml:"port" json:"port"`
	ListenPort   int       `yaml:"listen_port,omitempty" json:"listen_port,omitempty"` // For relay nodes
	Target       string    `yaml:"target,omitempty" json:"target,omitempty"`           // Next hop node name for relay
	SNITarget    string    `yaml:"sni_target" json:"sni_target"`
	IPv6         string    `yaml:"ipv6,omitempty" json:"ipv6,omitempty"`                   // Dual-stack Happy Eyeballs
	MonthlyQuota string    `yaml:"monthly_quota,omitempty" json:"monthly_quota,omitempty"` // e.g. "800GB"
	Platform     string    `yaml:"platform,omitempty" json:"platform,omitempty"`           // "auto", "ios", "desktop"

	// Runtime extracted credentials
	UUID      string `json:"uuid,omitempty"`
	PublicKey string `json:"public_key,omitempty"`
	ShortID   string `json:"short_id,omitempty"`
}

type SSHConfig struct {
	User     string `yaml:"user" json:"user"`
	Port     int    `yaml:"port" json:"port"`
	KeyPath  string `yaml:"key" json:"key"`
	Password string `yaml:"password,omitempty" json:"password,omitempty"`
}

type OutputConfig struct {
	ClashMeta       string `yaml:"clash_meta" json:"clash_meta"`
	SingBox         string `yaml:"sing_box" json:"sing_box"`
	SubscriptionURL string `yaml:"subscription_url,omitempty" json:"subscription_url,omitempty"`
	Platform        string `yaml:"platform,omitempty" json:"platform,omitempty"` // "auto", "ios", "desktop"
}

// LoadFleetConfig parses a declarative multi-node vpnctl.yaml file.
func LoadFleetConfig(path string) (*FleetConfig, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read %s: %w", path, err)
	}

	cfg := &FleetConfig{
		Version: "1",
		Nodes:   make([]NodeConfig, 0),
		Output: OutputConfig{
			ClashMeta: "./dist/clash.yaml",
			SingBox:   "./dist/sing-box.json",
		},
	}

	scanner := bufio.NewScanner(bytes.NewReader(data))
	currentSection := ""
	var currentNode *NodeConfig

	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}

		if line == "nodes:" {
			currentSection = "nodes"
			continue
		} else if line == "output:" {
			currentSection = "output"
			if currentNode != nil {
				cfg.Nodes = append(cfg.Nodes, *currentNode)
				currentNode = nil
			}
			continue
		}

		switch currentSection {
		case "output":
			parts := strings.SplitN(line, ":", 2)
			if len(parts) == 2 {
				k := strings.TrimSpace(parts[0])
				v := strings.Trim(strings.TrimSpace(parts[1]), "\"'\r\n")
				switch k {
				case "clash_meta":
					cfg.Output.ClashMeta = v
				case "sing_box":
					cfg.Output.SingBox = v
				case "subscription_url":
					cfg.Output.SubscriptionURL = v
				case "platform":
					cfg.Output.Platform = v
				}
			}

		case "nodes":
			if strings.HasPrefix(line, "- name:") {
				if currentNode != nil {
					cfg.Nodes = append(cfg.Nodes, *currentNode)
				}
				name := strings.Trim(strings.TrimPrefix(line, "- name:"), "\"'\r\n ")
				currentNode = &NodeConfig{
					Name:      name,
					Role:      "exit",
					Port:      443,
					Protocol:  "vless-reality",
					SNITarget: "auto",
					SSH:       SSHConfig{User: "root", Port: 22},
				}
				continue
			}

			if currentNode == nil {
				continue
			}

			parts := strings.SplitN(line, ":", 2)
			if len(parts) == 2 {
				k := strings.TrimSpace(parts[0])
				v := strings.Trim(strings.TrimSpace(parts[1]), "\"'\r\n")

				switch k {
				case "role":
					currentNode.Role = v
				case "host":
					currentNode.Host = v
				case "port":
					if p, err := strconv.Atoi(v); err == nil {
						currentNode.Port = p
					}
				case "listen_port":
					if p, err := strconv.Atoi(v); err == nil {
						currentNode.ListenPort = p
					}
				case "target":
					currentNode.Target = v
				case "protocol":
					currentNode.Protocol = v
				case "sni_target":
					currentNode.SNITarget = v
				case "ipv6":
					currentNode.IPv6 = v
				case "monthly_quota":
					currentNode.MonthlyQuota = v
				case "platform":
					currentNode.Platform = v
				case "ssh":
					// Parse inline: { user: "root", key: "~/.ssh/id_ed25519", port: 22 }
					parseInlineSSH(v, &currentNode.SSH)
				}
			}
		}
	}

	if currentNode != nil {
		cfg.Nodes = append(cfg.Nodes, *currentNode)
	}

	return cfg, nil
}

func parseInlineSSH(val string, ssh *SSHConfig) {
	val = strings.Trim(val, "{}")
	items := strings.Split(val, ",")
	for _, it := range items {
		kv := strings.SplitN(strings.TrimSpace(it), ":", 2)
		if len(kv) == 2 {
			k := strings.TrimSpace(kv[0])
			v := strings.Trim(strings.TrimSpace(kv[1]), "\"'\r\n")
			switch k {
			case "user":
				ssh.User = v
			case "key":
				ssh.KeyPath = expandTilde(v)
			case "port":
				if p, err := strconv.Atoi(v); err == nil {
					ssh.Port = p
				}
			case "password":
				ssh.Password = v
			}
		}
	}
}

func expandTilde(path string) string {
	if strings.HasPrefix(path, "~/") {
		home, err := os.UserHomeDir()
		if err == nil {
			return filepath.Join(home, path[2:])
		}
	}
	return path
}

// SaveFleetConfig saves the fleet configuration back to vpnctl.yaml.
func SaveFleetConfig(path string, cfg *FleetConfig) error {
	var sb strings.Builder
	sb.WriteString("version: \"1\"\n\n")
	sb.WriteString("nodes:\n")
	for _, n := range cfg.Nodes {
		sb.WriteString(fmt.Sprintf("  - name: \"%s\"\n", n.Name))
		if n.Role != "" && n.Role != "exit" {
			sb.WriteString(fmt.Sprintf("    role: \"%s\"\n", n.Role))
		}
		sb.WriteString(fmt.Sprintf("    host: \"%s\"\n", n.Host))
		if n.ListenPort > 0 {
			sb.WriteString(fmt.Sprintf("    listen_port: %d\n", n.ListenPort))
		}
		if n.Target != "" {
			sb.WriteString(fmt.Sprintf("    target: \"%s\"\n", n.Target))
		}
		sb.WriteString(fmt.Sprintf("    port: %d\n", n.Port))
		sb.WriteString(fmt.Sprintf("    protocol: \"%s\"\n", n.Protocol))
		sb.WriteString(fmt.Sprintf("    sni_target: \"%s\"\n", n.SNITarget))
		if n.IPv6 != "" {
			sb.WriteString(fmt.Sprintf("    ipv6: \"%s\"\n", n.IPv6))
		}
		if n.MonthlyQuota != "" {
			sb.WriteString(fmt.Sprintf("    monthly_quota: \"%s\"\n", n.MonthlyQuota))
		}
		if n.Platform != "" {
			sb.WriteString(fmt.Sprintf("    platform: \"%s\"\n", n.Platform))
		}
		sb.WriteString(fmt.Sprintf("    ssh: { user: \"%s\", key: \"%s\", port: %d }\n\n", n.SSH.User, n.SSH.KeyPath, n.SSH.Port))
	}
	sb.WriteString("output:\n")
	sb.WriteString(fmt.Sprintf("  clash_meta: \"%s\"\n", cfg.Output.ClashMeta))
	sb.WriteString(fmt.Sprintf("  sing_box: \"%s\"\n", cfg.Output.SingBox))
	if cfg.Output.SubscriptionURL != "" {
		sb.WriteString(fmt.Sprintf("  subscription_url: \"%s\"\n", cfg.Output.SubscriptionURL))
	}
	if cfg.Output.Platform != "" {
		sb.WriteString(fmt.Sprintf("  platform: \"%s\"\n", cfg.Output.Platform))
	}

	return os.WriteFile(path, []byte(sb.String()), 0644)
}

