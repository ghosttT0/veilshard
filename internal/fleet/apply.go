package fleet

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"strconv"
	"strings"

	"github.com/veilshard/veilshard/internal/credentials"
)

type UIProgress interface {
	Section(name string)
	Step(name string, success bool, detail string)
	Success(msg string)
	Error(msg string)
}

// ApplyFleet executes declarative orchestration for all nodes declared in vpnctl.yaml.
func ApplyFleet(ctx context.Context, fleetPath string, ui UIProgress) (*FleetConfig, error) {
	cfg, err := LoadFleetConfig(fleetPath)
	if err != nil {
		return nil, fmt.Errorf("load fleet config: %w", err)
	}

	if ui != nil {
		ui.Section(fmt.Sprintf("Deploying Fleet (%d nodes)", len(cfg.Nodes)))
	}

	// Two-pass orchestration: 1. Deploy all Exit nodes first
	exitNodesMap := make(map[string]*NodeConfig)
	for i := range cfg.Nodes {
		node := &cfg.Nodes[i]
		if node.Role == "exit" || node.Role == "" {
			if ui != nil {
				ui.Step(fmt.Sprintf("Configuring exit node: %s (%s:%d)", node.Name, node.Host, node.Port), true, "")
			}
			var err error
			if isLocalHost(node.Host) {
				err = configureLocalNode(node)
			} else {
				err = configureRemoteNode(ctx, node)
			}
			if err != nil {
				if ui != nil {
					ui.Step(fmt.Sprintf("Exit node %s failed", node.Name), false, err.Error())
				}
				return nil, fmt.Errorf("exit node %s: %w", node.Name, err)
			}
			exitNodesMap[node.Name] = node
			if ui != nil && len(node.UUID) >= 8 {
				ui.Step(fmt.Sprintf("Exit node %s ready", node.Name), true, fmt.Sprintf("UUID: %s...", node.UUID[:8]))
			}
		}
	}

	// 2. Deploy Relay nodes (connecting to target exit nodes)
	for i := range cfg.Nodes {
		node := &cfg.Nodes[i]
		if node.Role == "relay" {
			targetExit, exists := exitNodesMap[node.Target]
			if !exists {
				return nil, fmt.Errorf("relay node %s references unknown or failed exit node %s", node.Name, node.Target)
			}

			if node.ListenPort == 0 {
				node.ListenPort = 8443
			}

			if ui != nil {
				ui.Step(fmt.Sprintf("Configuring relay node: %s (:%d ➔ %s:%d)", node.Name, node.ListenPort, targetExit.Host, targetExit.Port), true, "")
			}

			var err error
			if isLocalHost(node.Host) {
				err = configureLocalRelay(node, targetExit)
			} else {
				err = configureRemoteRelay(ctx, node, targetExit)
			}
			if err != nil {
				if ui != nil {
					ui.Step(fmt.Sprintf("Relay node %s failed", node.Name), false, err.Error())
				}
				return nil, fmt.Errorf("relay node %s: %w", node.Name, err)
			}

			// Copy target credentials to relay node representation
			node.UUID = targetExit.UUID
			node.PublicKey = targetExit.PublicKey
			node.ShortID = targetExit.ShortID
			node.SNITarget = targetExit.SNITarget

			if ui != nil {
				ui.Step(fmt.Sprintf("Relay node %s ready", node.Name), true, fmt.Sprintf("Forwarding to %s", targetExit.Name))
			}
		}
	}

	// 1. Generate multi-node Clash Meta YAML
	clashContent := GenerateMultiNodeClashProfile(cfg.Nodes)
	clashPath := cfg.Output.ClashMeta
	if clashPath == "" {
		clashPath = "./dist/clash.yaml"
	}
	if err := WriteClashFile(clashPath, clashContent); err != nil {
		return nil, fmt.Errorf("write clash profile: %w", err)
	}

	// 2. Generate multi-node sing-box JSON
	singboxContent, err := GenerateSingBoxProfile(cfg.Nodes)
	if err == nil {
		singboxPath := cfg.Output.SingBox
		if singboxPath == "" {
			singboxPath = "./dist/sing-box.json"
		}
		_ = WriteSingBoxFile(singboxPath, singboxContent)
	}

	if ui != nil {
		ui.Section("Outputs Generated")
		ui.Step("Clash Meta Profile", true, clashPath)
		ui.Step("sing-box Profile", true, cfg.Output.SingBox)
		if cfg.Output.SubscriptionURL != "" {
			ui.Step("Subscription URL", true, cfg.Output.SubscriptionURL)
		}
	}

	return cfg, nil
}

func isLocalHost(host string) bool {
	return host == "" || host == "localhost" || host == "127.0.0.1" || host == "::1"
}

func configureLocalNode(node *NodeConfig) error {
	credsPath := "/etc/vpnctl/credentials.json"
	data, err := os.ReadFile(credsPath)
	if err != nil {
		// Auto-generate if not exists
		sni := node.SNITarget
		if sni == "" || sni == "auto" {
			sni = "gateway.icloud.com"
		}
		newCreds, err := credentials.Generate(sni)
		if err != nil {
			return err
		}
		node.UUID = newCreds.UUID
		node.PublicKey = newCreds.PublicKey
		node.ShortID = newCreds.ShortID
		return nil
	}

	var creds credentials.Credentials
	if err := json.Unmarshal(data, &creds); err != nil {
		return err
	}

	node.UUID = creds.UUID
	node.PublicKey = creds.PublicKey
	node.ShortID = creds.ShortID
	return nil
}

func configureRemoteNode(ctx context.Context, node *NodeConfig) error {
	// Construct SSH command
	sshArgs := []string{
		"-o", "StrictHostKeyChecking=no",
		"-o", "ConnectTimeout=10",
	}

	if node.SSH.Port > 0 {
		sshArgs = append(sshArgs, "-p", strconv.Itoa(node.SSH.Port))
	}

	if node.SSH.KeyPath != "" {
		sshArgs = append(sshArgs, "-i", node.SSH.KeyPath)
	}

	user := node.SSH.User
	if user == "" {
		user = "root"
	}
	target := fmt.Sprintf("%s@%s", user, node.Host)
	sshArgs = append(sshArgs, target)

	// Remote command: install or read credentials
	remoteCmd := fmt.Sprintf("if [ ! -f /etc/vpnctl/credentials.json ]; then sudo vpnctl install --dest %s --port %d; fi; cat /etc/vpnctl/credentials.json", node.SNITarget, node.Port)
	sshArgs = append(sshArgs, remoteCmd)

	cmd := exec.CommandContext(ctx, "ssh", sshArgs...)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("ssh execute on %s: %w (%s)", node.Host, err, string(bytes.TrimSpace(out)))
	}

	// Extract JSON payload
	var creds credentials.Credentials
	startIdx := strings.Index(string(out), "{")
	endIdx := strings.LastIndex(string(out), "}")
	if startIdx != -1 && endIdx != -1 && endIdx > startIdx {
		jsonStr := out[startIdx : endIdx+1]
		if err := json.Unmarshal(jsonStr, &creds); err == nil {
			node.UUID = creds.UUID
			node.PublicKey = creds.PublicKey
			node.ShortID = creds.ShortID
			return nil
		}
	}

	return fmt.Errorf("could not parse credentials from remote node output: %s", string(out))
}

func configureLocalRelay(relay *NodeConfig, exit *NodeConfig) error {
	// Enable IP forwarding
	_ = exec.Command("sysctl", "-w", "net.ipv4.ip_forward=1").Run()

	listenPort := relay.ListenPort
	if listenPort == 0 {
		listenPort = 8443
	}

	// Setup iptables DNAT and MASQUERADE
	cmdDNAT := fmt.Sprintf("iptables -t nat -C PREROUTING -p tcp --dport %d -j DNAT --to-destination %s:%d 2>/dev/null || iptables -t nat -A PREROUTING -p tcp --dport %d -j DNAT --to-destination %s:%d",
		listenPort, exit.Host, exit.Port, listenPort, exit.Host, exit.Port)
	_ = exec.Command("sh", "-c", cmdDNAT).Run()

	cmdSNAT := fmt.Sprintf("iptables -t nat -C POSTROUTING -p tcp -d %s --dport %d -j MASQUERADE 2>/dev/null || iptables -t nat -A POSTROUTING -p tcp -d %s --dport %d -j MASQUERADE",
		exit.Host, exit.Port, exit.Host, exit.Port)
	_ = exec.Command("sh", "-c", cmdSNAT).Run()

	return nil
}

func configureRemoteRelay(ctx context.Context, relay *NodeConfig, exit *NodeConfig) error {
	sshArgs := []string{
		"-o", "StrictHostKeyChecking=no",
		"-o", "ConnectTimeout=10",
	}

	if relay.SSH.Port > 0 {
		sshArgs = append(sshArgs, "-p", strconv.Itoa(relay.SSH.Port))
	}
	if relay.SSH.KeyPath != "" {
		sshArgs = append(sshArgs, "-i", relay.SSH.KeyPath)
	}

	user := relay.SSH.User
	if user == "" {
		user = "root"
	}
	target := fmt.Sprintf("%s@%s", user, relay.Host)
	sshArgs = append(sshArgs, target)

	listenPort := relay.ListenPort
	if listenPort == 0 {
		listenPort = 8443
	}

	// Remote script: enable ip_forward, configure DNAT/MASQUERADE, allow port in UFW
	script := fmt.Sprintf(`sudo sysctl -w net.ipv4.ip_forward=1 >/dev/null 2>&1
sudo iptables -t nat -C PREROUTING -p tcp --dport %d -j DNAT --to-destination %s:%d 2>/dev/null || sudo iptables -t nat -A PREROUTING -p tcp --dport %d -j DNAT --to-destination %s:%d
sudo iptables -t nat -C POSTROUTING -p tcp -d %s --dport %d -j MASQUERADE 2>/dev/null || sudo iptables -t nat -A POSTROUTING -p tcp -d %s --dport %d -j MASQUERADE
if command -v ufw >/dev/null 2>&1 && sudo ufw status | grep -q 'Status: active'; then
  sudo ufw allow %d/tcp comment 'vpnctl:relay' >/dev/null 2>&1
fi
echo "OK"`, listenPort, exit.Host, exit.Port, listenPort, exit.Host, exit.Port, exit.Host, exit.Port, exit.Host, exit.Port, listenPort)

	sshArgs = append(sshArgs, script)

	cmd := exec.CommandContext(ctx, "ssh", sshArgs...)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("ssh relay config on %s: %w (%s)", relay.Host, err, string(bytes.TrimSpace(out)))
	}

	return nil
}

