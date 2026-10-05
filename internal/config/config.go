package config

import (
	"bufio"
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

const (
	DefaultConfigPath = "/etc/vpnctl/vpnctl.yaml"
	DefaultServerPort = 443
	DefaultCore       = "xray"
	DefaultProtocol   = "vless-reality"
	DefaultFirewall   = "ufw"
	DefaultUser       = "vpnctl-proxy"
	DefaultClient     = "mihomo"
	DefaultDest       = "gateway.icloud.com:443"
	DefaultSNI        = "gateway.icloud.com"
)

// Config represents user-intended configuration.
type Config struct {
	Version  int            `json:"version" yaml:"version"`
	Server   ServerConfig   `json:"server" yaml:"server"`
	Core     CoreConfig     `json:"core" yaml:"core"`
	Protocol ProtocolConfig `json:"protocol" yaml:"protocol"`
	Firewall FirewallConfig `json:"firewall" yaml:"firewall"`
	Security SecurityConfig `json:"security" yaml:"security"`
	Export   ExportConfig   `json:"export" yaml:"export"`
}

type ServerConfig struct {
	Port     int    `json:"port" yaml:"port"`
	Listen   string `json:"listen,omitempty" yaml:"listen,omitempty"`
	PublicIP string `json:"public_ip,omitempty" yaml:"public_ip,omitempty"`
}

type CoreConfig struct {
	Provider   string `json:"provider" yaml:"provider"`
	Version    string `json:"version,omitempty" yaml:"version,omitempty"`
	BinaryPath string `json:"binary_path,omitempty" yaml:"binary_path,omitempty"`
}

type ProtocolConfig struct {
	Type        string   `json:"type" yaml:"type"`
	Dest        string   `json:"dest" yaml:"dest"`
	ServerNames []string `json:"server_names" yaml:"server_names"`
}

type FirewallConfig struct {
	Provider string `json:"provider" yaml:"provider"`
}

type SecurityConfig struct {
	RunAsUser string `json:"run_as_user" yaml:"run_as_user"`
}

type ExportConfig struct {
	DefaultClient string `json:"default_client" yaml:"default_client"`
}

// NewDefault returns the standard default configuration for v0.1.
func NewDefault() *Config {
	return &Config{
		Version: 1,
		Server: ServerConfig{
			Port:   DefaultServerPort,
			Listen: "0.0.0.0",
		},
		Core: CoreConfig{
			Provider:   DefaultCore,
			Version:    "25.1.30",
			BinaryPath: "/usr/local/bin/xray",
		},
		Protocol: ProtocolConfig{
			Type:        DefaultProtocol,
			Dest:        DefaultDest,
			ServerNames: []string{DefaultSNI},
		},
		Firewall: FirewallConfig{
			Provider: DefaultFirewall,
		},
		Security: SecurityConfig{
			RunAsUser: DefaultUser,
		},
		Export: ExportConfig{
			DefaultClient: DefaultClient,
		},
	}
}

// Load loads the YAML config file from disk. If not found, returns default.
func Load(path string) (*Config, error) {
	if path == "" {
		path = DefaultConfigPath
	}

	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return NewDefault(), nil
		}
		return nil, fmt.Errorf("read config file %s: %w", path, err)
	}

	cfg := NewDefault()
	if err := parseYAML(data, cfg); err != nil {
		return nil, fmt.Errorf("parse config yaml: %w", err)
	}

	return cfg, nil
}

// Save writes the configuration as formatted YAML to the specified path.
func (c *Config) Save(path string) error {
	if path == "" {
		path = DefaultConfigPath
	}

	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0700); err != nil {
		return fmt.Errorf("mkdir config dir %s: %w", dir, err)
	}

	var sb strings.Builder
	sb.WriteString("# vpnctl configuration\n")
	sb.WriteString(fmt.Sprintf("version: %d\n\n", c.Version))

	sb.WriteString("server:\n")
	sb.WriteString(fmt.Sprintf("  port: %d\n", c.Server.Port))
	if c.Server.Listen != "" {
		sb.WriteString(fmt.Sprintf("  listen: \"%s\"\n", c.Server.Listen))
	}
	if c.Server.PublicIP != "" {
		sb.WriteString(fmt.Sprintf("  public_ip: \"%s\"\n", c.Server.PublicIP))
	}

	sb.WriteString("\ncore:\n")
	sb.WriteString(fmt.Sprintf("  provider: %s\n", c.Core.Provider))
	if c.Core.Version != "" {
		sb.WriteString(fmt.Sprintf("  version: \"%s\"\n", c.Core.Version))
	}
	if c.Core.BinaryPath != "" {
		sb.WriteString(fmt.Sprintf("  binary_path: \"%s\"\n", c.Core.BinaryPath))
	}

	sb.WriteString("\nprotocol:\n")
	sb.WriteString(fmt.Sprintf("  type: %s\n", c.Protocol.Type))
	sb.WriteString(fmt.Sprintf("  dest: \"%s\"\n", c.Protocol.Dest))
	sb.WriteString("  server_names:\n")
	for _, sn := range c.Protocol.ServerNames {
		sb.WriteString(fmt.Sprintf("    - \"%s\"\n", sn))
	}

	sb.WriteString("\nfirewall:\n")
	sb.WriteString(fmt.Sprintf("  provider: %s\n", c.Firewall.Provider))

	sb.WriteString("\nsecurity:\n")
	sb.WriteString(fmt.Sprintf("  run_as_user: %s\n", c.Security.RunAsUser))

	sb.WriteString("\nexport:\n")
	sb.WriteString(fmt.Sprintf("  default_client: %s\n", c.Export.DefaultClient))

	tmpFile := fmt.Sprintf("%s.tmp.%d", path, os.Getpid())
	if err := os.WriteFile(tmpFile, []byte(sb.String()), 0600); err != nil {
		return fmt.Errorf("write tmp config file: %w", err)
	}

	if err := os.Rename(tmpFile, path); err != nil {
		_ = os.Remove(tmpFile)
		return fmt.Errorf("atomic rename config file: %w", err)
	}

	return nil
}

// parseYAML provides a lightweight YAML parser tailored to the vpnctl configuration structure.
func parseYAML(data []byte, cfg *Config) error {
	scanner := bufio.NewScanner(bytes.NewReader(data))
	currentSection := ""

	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}

		if !strings.HasPrefix(line, "-") && strings.HasSuffix(line, ":") {
			currentSection = strings.TrimSuffix(line, ":")
			continue
		}

		parts := strings.SplitN(line, ":", 2)
		if len(parts) == 2 {
			k := strings.TrimSpace(parts[0])
			v := strings.Trim(strings.TrimSpace(parts[1]), "\"'\r\n")

			switch currentSection {
			case "":
				if k == "version" {
					if n, err := strconv.Atoi(v); err == nil {
						cfg.Version = n
					}
				}
			case "server":
				switch k {
				case "port":
					if p, err := strconv.Atoi(v); err == nil {
						cfg.Server.Port = p
					}
				case "listen":
					cfg.Server.Listen = v
				case "public_ip":
					cfg.Server.PublicIP = v
				}
			case "core":
				switch k {
				case "provider":
					cfg.Core.Provider = v
				case "version":
					cfg.Core.Version = v
				case "binary_path":
					cfg.Core.BinaryPath = v
				}
			case "protocol":
				switch k {
				case "type":
					cfg.Protocol.Type = v
				case "dest":
					cfg.Protocol.Dest = v
				}
			case "firewall":
				if k == "provider" {
					cfg.Firewall.Provider = v
				}
			case "security":
				if k == "run_as_user" {
					cfg.Security.RunAsUser = v
				}
			case "export":
				if k == "default_client" {
					cfg.Export.DefaultClient = v
				}
			}
		} else if strings.HasPrefix(line, "- ") && currentSection == "protocol" {
			val := strings.Trim(strings.TrimPrefix(line, "- "), "\"'\r\n ")
			if val != "" {
				cfg.Protocol.ServerNames = append(cfg.Protocol.ServerNames, val)
			}
		}
	}

	return scanner.Err()
}
