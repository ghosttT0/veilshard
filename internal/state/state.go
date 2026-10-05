package state

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"
)

const (
	DefaultStatePath = "/etc/vpnctl/state.json"
	CurrentSchema    = 1
)

// State tracks the actual resources managed and created by vpnctl.
type State struct {
	SchemaVersion        int       `json:"schema_version"`
	Installed            bool      `json:"installed"`
	InstalledAt          time.Time `json:"installed_at,omitempty"`
	UpdatedAt            time.Time `json:"updated_at,omitempty"`
	CoreProvider         string    `json:"core_provider"`
	CoreVersion          string    `json:"core_version"`
	CoreBinaryPath       string    `json:"core_binary_path"`
	ServiceName          string    `json:"service_name"`
	ServiceFilePath      string    `json:"service_file_path"`
	ManagedFirewallRules []string  `json:"managed_firewall_rules"`
	FirewallProvider     string    `json:"firewall_provider"`
	SystemUser           string    `json:"system_user"`
	ConfigPath           string    `json:"config_path"`
	CoreConfigPath       string    `json:"core_config_path"`
	CredentialsPath      string    `json:"credentials_path"`
	SSHPortsPreserved    []int     `json:"ssh_ports_preserved"`
	PublicIPv4           string    `json:"public_ipv4,omitempty"`
}

// Load loads the state from the given path (or default path if empty).
func Load(path string) (*State, error) {
	if path == "" {
		path = DefaultStatePath
	}

	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return &State{
				SchemaVersion:        CurrentSchema,
				Installed:            false,
				ManagedFirewallRules: []string{},
				SSHPortsPreserved:    []int{},
			}, nil
		}
		return nil, fmt.Errorf("read state file: %w", err)
	}

	var st State
	if err := json.Unmarshal(data, &st); err != nil {
		return nil, fmt.Errorf("unmarshal state json: %w", err)
	}

	if st.ManagedFirewallRules == nil {
		st.ManagedFirewallRules = []string{}
	}
	if st.SSHPortsPreserved == nil {
		st.SSHPortsPreserved = []int{}
	}

	return &st, nil
}

// Save atomic-writes the state file with permissions 0600.
func (s *State) Save(path string) error {
	if path == "" {
		path = DefaultStatePath
	}

	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0700); err != nil {
		return fmt.Errorf("mkdir state dir %s: %w", dir, err)
	}

	s.UpdatedAt = time.Now().UTC()
	data, err := json.MarshalIndent(s, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal state json: %w", err)
	}

	tmpFile := fmt.Sprintf("%s.tmp.%d", path, time.Now().UnixNano())
	if err := os.WriteFile(tmpFile, data, 0600); err != nil {
		return fmt.Errorf("write tmp state file: %w", err)
	}

	if err := os.Rename(tmpFile, path); err != nil {
		_ = os.Remove(tmpFile)
		return fmt.Errorf("atomic rename state file: %w", err)
	}

	return nil
}

// AddManagedFirewallRule records a rule as managed by vpnctl if not already recorded.
func (s *State) AddManagedFirewallRule(rule string) {
	for _, r := range s.ManagedFirewallRules {
		if r == rule {
			return
		}
	}
	s.ManagedFirewallRules = append(s.ManagedFirewallRules, rule)
}

// RemoveManagedFirewallRule removes a rule from the managed list.
func (s *State) RemoveManagedFirewallRule(rule string) {
	newRules := make([]string, 0, len(s.ManagedFirewallRules))
	for _, r := range s.ManagedFirewallRules {
		if r != rule {
			newRules = append(newRules, r)
		}
	}
	s.ManagedFirewallRules = newRules
}
