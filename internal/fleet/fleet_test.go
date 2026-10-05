package fleet

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestFleetSchemaAndMultiNodeGenerators(t *testing.T) {
	tempDir := t.TempDir()
	yamlPath := filepath.Join(tempDir, "vpnctl.yaml")

	yamlContent := `
version: "1"
nodes:
  - name: "us-west-01"
    host: "198.51.100.2"
    ssh: { user: "root", port: 22, key: "~/.ssh/id_ed25519" }
    protocol: vless-reality
    port: 443
    sni_target: "auto"

  - name: "jp-tokyo-01"
    host: "203.0.113.5"
    ssh: { user: "root", port: 62505, key: "~/.ssh/id_ed25519" }
    protocol: vless-reality
    port: 443
    sni_target: "gateway.icloud.com"

output:
  clash_meta: "./dist/clash.yaml"
  sing_box: "./dist/sing-box.json"
`
	if err := os.WriteFile(yamlPath, []byte(yamlContent), 0644); err != nil {
		t.Fatalf("write yaml: %v", err)
	}

	cfg, err := LoadFleetConfig(yamlPath)
	if err != nil {
		t.Fatalf("load fleet config failed: %v", err)
	}

	if len(cfg.Nodes) != 2 {
		t.Fatalf("expected 2 nodes, got %d", len(cfg.Nodes))
	}

	if cfg.Nodes[0].Name != "us-west-01" || cfg.Nodes[0].Host != "198.51.100.2" {
		t.Errorf("node 0 mismatch: %+v", cfg.Nodes[0])
	}
	if cfg.Nodes[1].Name != "jp-tokyo-01" || cfg.Nodes[1].SSH.Port != 62505 {
		t.Errorf("node 1 mismatch: %+v", cfg.Nodes[1])
	}

	// Populate mock credentials
	cfg.Nodes[0].UUID = "11111111-2222-3333-4444-555566667777"
	cfg.Nodes[0].PublicKey = "mock-pub-key-1"
	cfg.Nodes[0].ShortID = "1234abcd"

	cfg.Nodes[1].UUID = "99999999-8888-7777-6666-555544443333"
	cfg.Nodes[1].PublicKey = "mock-pub-key-2"
	cfg.Nodes[1].ShortID = "5678ef01"

	// 1. Generate multi-node Clash Meta YAML
	clashYaml := GenerateMultiNodeClashProfile(cfg.Nodes)
	if !strings.Contains(clashYaml, "us-west-01") || !strings.Contains(clashYaml, "jp-tokyo-01") {
		t.Errorf("clash yaml missing nodes: %s", clashYaml)
	}
	if !strings.Contains(clashYaml, "⚡ 自动优选") || !strings.Contains(clashYaml, "🚀 节点选择") || !strings.Contains(clashYaml, "🛡️ 故障转移") {
		t.Errorf("clash yaml missing proxy groups")
	}

	// 2. Generate multi-node sing-box JSON
	singboxJSON, err := GenerateSingBoxProfile(cfg.Nodes)
	if err != nil {
		t.Fatalf("generate singbox json: %v", err)
	}
	if !strings.Contains(singboxJSON, "us-west-01") || !strings.Contains(singboxJSON, "jp-tokyo-01") {
		t.Errorf("singbox json missing nodes")
	}

	// 3. Test SaveFleetConfig
	cfg.Nodes[0].Host = "198.51.100.99"
	savedPath := filepath.Join(tempDir, "saved_fleet.yaml")
	if err := SaveFleetConfig(savedPath, cfg); err != nil {
		t.Fatalf("save fleet config: %v", err)
	}
	reloaded, err := LoadFleetConfig(savedPath)
	if err != nil {
		t.Fatalf("reload saved fleet config: %v", err)
	}
	if reloaded.Nodes[0].Host != "198.51.100.99" {
		t.Errorf("expected host 198.51.100.99, got %s", reloaded.Nodes[0].Host)
	}
}

func TestRelayMeshPipelineResolution(t *testing.T) {
	meshNodes := []NodeConfig{
		{
			Name:       "hk-transit",
			Role:       "relay",
			Host:       "103.200.1.5",
			ListenPort: 8443,
			Target:     "us-exit",
			Platform:   "ios",
		},
		{
			Name:      "us-exit",
			Role:      "exit",
			Host:      "198.51.100.2",
			Port:      443,
			Protocol:  "vless-reality",
			SNITarget: "gateway.icloud.com",
			UUID:      "a1b2c3d4-e5f6-7890-abcd-ef1234567890",
			PublicKey: "mock-exit-reality-key",
			ShortID:   "abcd1234",
		},
	}

	clashProfile := GenerateMultiNodeClashProfile(meshNodes)
	if !strings.Contains(clashProfile, "hk-transit ➔ us-exit") {
		t.Errorf("clash profile missing resolved relay proxy: %s", clashProfile)
	}
	if !strings.Contains(clashProfile, "server: \"103.200.1.5\"") {
		t.Errorf("relay proxy did not use transit host")
	}
	if !strings.Contains(clashProfile, "port: 8443") {
		t.Errorf("relay proxy did not use transit listen port")
	}
	if !strings.Contains(clashProfile, "public-key: \"mock-exit-reality-key\"") {
		t.Errorf("relay proxy did not borrow exit node reality public key")
	}
	if !strings.Contains(clashProfile, "client-fingerprint: safari") {
		t.Errorf("relay proxy did not align iOS platform fingerprint to safari")
	}

	singboxProfile, err := GenerateSingBoxProfile(meshNodes)
	if err != nil {
		t.Fatalf("singbox generate failed: %v", err)
	}
	if !strings.Contains(singboxProfile, "hk-transit ➔ us-exit") {
		t.Errorf("singbox missing relay proxy: %s", singboxProfile)
	}
	if !strings.Contains(singboxProfile, "103.200.1.5") || !strings.Contains(singboxProfile, "8443") {
		t.Errorf("singbox missing relay transit endpoint")
	}
}

