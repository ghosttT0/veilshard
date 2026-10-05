package fleet

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// GenerateSingBoxProfile produces standard sing-box client JSON configuration.
func GenerateSingBoxProfile(nodes []NodeConfig) (string, error) {
	var nodeTags []string
	var outbounds []map[string]interface{}

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
				continue
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
				continue
			}
		}

		nodeTags = append(nodeTags, displayName)

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
			fp = platform
		}

		nodeOutbound := map[string]interface{}{
			"type":        "vless",
			"tag":         displayName,
			"server":      serverHost,
			"server_port": serverPort,
			"uuid":        uuid,
			"tls": map[string]interface{}{
				"enabled":     true,
				"server_name": sni,
				"reality": map[string]interface{}{
					"enabled":    true,
					"public_key": pubKey,
					"short_id":   shortID,
				},
				"utls": map[string]interface{}{
					"enabled":     true,
					"fingerprint": fp,
				},
			},
		}
		outbounds = append(outbounds, nodeOutbound)
	}

	// Selector group
	selectorOutbounds := append([]string{"auto"}, nodeTags...)
	selectorOutbounds = append(selectorOutbounds, "direct")

	fullOutbounds := []map[string]interface{}{
		{
			"type":      "selector",
			"tag":       "select",
			"outbounds": selectorOutbounds,
		},
		{
			"type":      "urltest",
			"tag":       "auto",
			"outbounds": nodeTags,
			"url":       "http://www.gstatic.com/generate_204",
			"interval":  "5m",
		},
	}

	fullOutbounds = append(fullOutbounds, outbounds...)
	fullOutbounds = append(fullOutbounds, map[string]interface{}{
		"type": "direct",
		"tag":  "direct",
	})

	config := map[string]interface{}{
		"log": map[string]interface{}{
			"level": "info",
		},
		"inbounds": []map[string]interface{}{
			{
				"type":        "mixed",
				"tag":         "mixed-in",
				"listen":      "127.0.0.1",
				"listen_port": 7890,
			},
		},
		"outbounds": fullOutbounds,
		"route": map[string]interface{}{
			"rules": []map[string]interface{}{
				{
					"geoip":    []string{"private", "cn"},
					"outbound": "direct",
				},
				{
					"geosite":  []string{"cn"},
					"outbound": "direct",
				},
			},
			"final":                 "select",
			"auto_detect_interface": true,
		},
	}

	data, err := json.MarshalIndent(config, "", "  ")
	if err != nil {
		return "", fmt.Errorf("marshal singbox json: %w", err)
	}

	return string(data), nil
}

// WriteSingBoxFile writes the sing-box profile to disk.
func WriteSingBoxFile(outputPath string, content string) error {
	dir := filepath.Dir(outputPath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("mkdir %s: %w", dir, err)
	}
	return os.WriteFile(outputPath, []byte(content), 0644)
}
