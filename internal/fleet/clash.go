package fleet

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/veilshard/veilshard/internal/fingerprint"
)

// GenerateMultiNodeClashProfile builds a complete multi-node Clash Meta profile with automatic failover groups.
func GenerateMultiNodeClashProfile(nodes []NodeConfig) string {
	var sb strings.Builder

	sb.WriteString("# vpnctl Fleet Multi-Node Profile for Mihomo / Clash Meta\n")
	sb.WriteString("port: 7890\n")
	sb.WriteString("socks-port: 7891\n")
	sb.WriteString("allow-lan: false\n")
	sb.WriteString("mode: rule\n")
	sb.WriteString("log-level: info\n")
	sb.WriteString("external-controller: 127.0.0.1:9090\n\n")

	sb.WriteString("dns:\n")
	sb.WriteString("  enable: true\n")
	sb.WriteString("  ipv6: false\n")
	sb.WriteString("  default-nameserver:\n")
	sb.WriteString("    - 223.5.5.5\n")
	sb.WriteString("    - 119.29.29.29\n")
	sb.WriteString("  nameserver:\n")
	sb.WriteString("    - https://dns.alidns.com/dns-query\n")
	sb.WriteString("    - https://doh.pub/dns-query\n")
	sb.WriteString("  fallback:\n")
	sb.WriteString("    - https://1.1.1.1/dns-query\n")
	sb.WriteString("    - https://dns.google/dns-query\n\n")

	sb.WriteString("proxies:\n")
	var nodeNames []string

	// Index exit nodes for relay target resolution
	exitNodes := make(map[string]NodeConfig)
	for _, n := range nodes {
		if n.Role == "exit" || n.Role == "" {
			exitNodes[n.Name] = n
		}
	}

	for _, n := range nodes {
		displayName := n.Name
		serverHost := n.Host
		serverPort := n.Port
		uuid := n.UUID
		pubKey := n.PublicKey
		shortID := n.ShortID
		sniTarget := n.SNITarget
		platform := n.Platform

		if n.Role == "relay" {
			targetExit, ok := exitNodes[n.Target]
			if !ok || targetExit.PublicKey == "" {
				continue // Skip if target exit node not resolved or uninitialized
			}
			displayName = fmt.Sprintf("%s ➔ %s", n.Name, targetExit.Name)
			if n.ListenPort > 0 {
				serverPort = n.ListenPort
			}
			uuid = targetExit.UUID
			pubKey = targetExit.PublicKey
			shortID = targetExit.ShortID
			sniTarget = targetExit.SNITarget
			if platform == "" {
				platform = targetExit.Platform
			}
		} else {
			if uuid == "" || pubKey == "" {
				continue // Skip uninitialized exit nodes
			}
		}

		nodeNames = append(nodeNames, displayName)

		sni := sniTarget
		if sni == "" || sni == "auto" {
			sni = "gateway.icloud.com"
		}
		if strings.Contains(sni, ":") {
			sni = strings.Split(sni, ":")[0]
		}

		fp := "chrome"
		if platform == "ios" || strings.Contains(strings.ToLower(displayName), "ios") {
			fp = "safari"
		} else if platform != "" {
			// Normalize platform labels ("desktop", etc.) to valid uTLS
			// fingerprints — Mihomo rejects unknown values.
			fp = fingerprint.ResolveOptions(platform).ClientFingerprint
		}

		sb.WriteString(fmt.Sprintf("  - name: \"%s\"\n", displayName))
		sb.WriteString("    type: vless\n")
		sb.WriteString(fmt.Sprintf("    server: \"%s\"\n", serverHost))
		sb.WriteString(fmt.Sprintf("    port: %d\n", serverPort))
		sb.WriteString(fmt.Sprintf("    uuid: \"%s\"\n", uuid))
		sb.WriteString("    network: tcp\n")
		sb.WriteString("    tls: true\n")
		sb.WriteString("    udp: true\n")
		sb.WriteString("    flow: xtls-rprx-vision\n")
		sb.WriteString(fmt.Sprintf("    servername: \"%s\"\n", sni))
		sb.WriteString(fmt.Sprintf("    client-fingerprint: %s\n", fp))
		sb.WriteString("    reality-opts:\n")
		sb.WriteString(fmt.Sprintf("      public-key: \"%s\"\n", pubKey))
		sb.WriteString(fmt.Sprintf("      short-id: \"%s\"\n\n", shortID))

		// Emit IPv6 dual-stack proxy if IPv6 is declared
		if n.IPv6 != "" {
			ipv6Name := fmt.Sprintf("%s [IPv6]", displayName)
			nodeNames = append(nodeNames, ipv6Name)

			sb.WriteString(fmt.Sprintf("  - name: \"%s\"\n", ipv6Name))
			sb.WriteString("    type: vless\n")
			sb.WriteString(fmt.Sprintf("    server: \"%s\"\n", n.IPv6))
			sb.WriteString(fmt.Sprintf("    port: %d\n", serverPort))
			sb.WriteString(fmt.Sprintf("    uuid: \"%s\"\n", uuid))
			sb.WriteString("    network: tcp\n")
			sb.WriteString("    tls: true\n")
			sb.WriteString("    udp: true\n")
			sb.WriteString("    flow: xtls-rprx-vision\n")
			sb.WriteString(fmt.Sprintf("    servername: \"%s\"\n", sni))
			sb.WriteString(fmt.Sprintf("    client-fingerprint: %s\n", fp))
			sb.WriteString("    reality-opts:\n")
			sb.WriteString(fmt.Sprintf("      public-key: \"%s\"\n", pubKey))
			sb.WriteString(fmt.Sprintf("      short-id: \"%s\"\n\n", shortID))
		}
	}

	sb.WriteString("proxy-groups:\n")
	// 1. Manual Selector Group
	sb.WriteString("  - name: \"🚀 节点选择\"\n")
	sb.WriteString("    type: select\n")
	sb.WriteString("    proxies:\n")
	sb.WriteString("      - \"⚡ 自动优选\"\n")
	sb.WriteString("      - \"🛡️ 故障转移\"\n")
	sb.WriteString("      - \"⚡ Happy-Eyeballs (RFC 8305)\"\n")
	sb.WriteString("      - \"⚖️ 负载均衡\"\n")
	for _, name := range nodeNames {
		sb.WriteString(fmt.Sprintf("      - \"%s\"\n", name))
	}
	sb.WriteString("      - DIRECT\n\n")

	// 2. Happy Eyeballs Group (Dual-Stack IPv4/IPv6 Fast Failover)
	sb.WriteString("  - name: \"⚡ Happy-Eyeballs (RFC 8305)\"\n")
	sb.WriteString("    type: url-test\n")
	sb.WriteString("    url: http://www.gstatic.com/generate_204\n")
	sb.WriteString("    interval: 180\n")
	sb.WriteString("    tolerance: 50\n")
	sb.WriteString("    proxies:\n")
	for _, name := range nodeNames {
		sb.WriteString(fmt.Sprintf("      - \"%s\"\n", name))
	}
	sb.WriteString("\n")

	// 2. Lowest Latency Group (URL-Test)
	sb.WriteString("  - name: \"⚡ 自动优选\"\n")
	sb.WriteString("    type: url-test\n")
	sb.WriteString("    url: http://www.gstatic.com/generate_204\n")
	sb.WriteString("    interval: 300\n")
	sb.WriteString("    tolerance: 50\n")
	sb.WriteString("    proxies:\n")
	for _, name := range nodeNames {
		sb.WriteString(fmt.Sprintf("      - \"%s\"\n", name))
	}
	sb.WriteString("\n")

	// 3. Fallback Group (Priority failover)
	sb.WriteString("  - name: \"🛡️ 故障转移\"\n")
	sb.WriteString("    type: fallback\n")
	sb.WriteString("    url: http://www.gstatic.com/generate_204\n")
	sb.WriteString("    interval: 180\n")
	sb.WriteString("    proxies:\n")
	for _, name := range nodeNames {
		sb.WriteString(fmt.Sprintf("      - \"%s\"\n", name))
	}
	sb.WriteString("\n")

	// 4. Load Balance Group
	sb.WriteString("  - name: \"⚖️ 负载均衡\"\n")
	sb.WriteString("    type: load-balance\n")
	sb.WriteString("    strategy: round-robin\n")
	sb.WriteString("    url: http://www.gstatic.com/generate_204\n")
	sb.WriteString("    interval: 300\n")
	sb.WriteString("    proxies:\n")
	for _, name := range nodeNames {
		sb.WriteString(fmt.Sprintf("      - \"%s\"\n", name))
	}
	sb.WriteString("\n")

	sb.WriteString("rules:\n")
	sb.WriteString("  - DOMAIN-SUFFIX,local,DIRECT\n")
	sb.WriteString("  - IP-CIDR,127.0.0.0/8,DIRECT\n")
	sb.WriteString("  - IP-CIDR,172.16.0.0/12,DIRECT\n")
	sb.WriteString("  - IP-CIDR,192.168.0.0/16,DIRECT\n")
	sb.WriteString("  - IP-CIDR,10.0.0.0/8,DIRECT\n")
	sb.WriteString("  - GEOSITE,category-ads-all,REJECT\n")
	sb.WriteString("  - GEOSITE,cn,DIRECT\n")
	sb.WriteString("  - GEOIP,CN,DIRECT\n")
	sb.WriteString("  - MATCH,🚀 节点选择\n")

	return sb.String()
}

// WriteClashFile writes the generated profile to disk.
func WriteClashFile(outputPath string, content string) error {
	dir := filepath.Dir(outputPath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("mkdir %s: %w", dir, err)
	}
	return os.WriteFile(outputPath, []byte(content), 0644)
}
