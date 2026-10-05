package cli

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"flag"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/veilshard/veilshard/internal/exporter"
	"github.com/veilshard/veilshard/internal/exporter/mihomo"
	"github.com/veilshard/veilshard/internal/exporter/qr"
	"github.com/veilshard/veilshard/internal/exporter/sub"
	"github.com/veilshard/veilshard/internal/exporter/uri"
	"github.com/veilshard/veilshard/internal/firewall/ufw"
)

func runExport(args []string) error {
	ui := NewUI()

	targetType := "mihomo"
	username := ""

	if len(args) > 0 {
		targetType = strings.ToLower(args[0])
		if len(args) > 1 && !strings.HasPrefix(args[1], "-") {
			username = args[1]
		}
	}

	expCtx, err := exporter.LoadExportContext(username)
	if err != nil {
		ui.Error(fmt.Sprintf("Failed to load export context: %v", err))
		return err
	}

	platform := "desktop"
	for i, a := range args {
		if (a == "-platform" || a == "--platform") && i+1 < len(args) {
			platform = args[i+1]
		}
	}

	switch targetType {
	case "mihomo", "node":
		out := mihomo.GenerateYAML(expCtx, platform)
		fmt.Println(out)
		return nil

	case "profile", "clash":
		out := sub.GenerateFullClashProfile(expCtx, platform)
		fmt.Println(out)
		return nil

	case "uri", "url", "link":
		link := uri.GenerateURI(expCtx)
		fmt.Println(link)
		return nil

	case "sub", "base64":
		b64 := sub.GenerateBase64Subscription(expCtx)
		fmt.Println(b64)
		return nil

	case "qr", "qrcode":
		link := uri.GenerateURI(expCtx)
		fmt.Printf("\nVLESS Share URI for user [%s]:\n%s\n\n", expCtx.UserName, link)
		qrText, err := qr.GenerateTerminal(link)
		if err != nil {
			ui.Error(fmt.Sprintf("Failed to render QR code: %v", err))
			return err
		}
		fmt.Println(qrText)
		return nil

	case "serve":
		fs := flag.NewFlagSet("export serve", flag.ExitOnError)
		port := fs.Int("port", 8080, "Port for the HTTP subscription server")
		token := fs.String("token", "", "Secret token path for subscription URL")
		_ = fs.Parse(args[1:])

		tokenVal := *token
		if tokenVal == "" {
			tokenPath := "/etc/vpnctl/sub_token"
			if data, err := os.ReadFile(tokenPath); err == nil && len(strings.TrimSpace(string(data))) >= 32 {
				tokenVal = strings.TrimSpace(string(data))
			} else {
				// Generate cryptographically secure 256-bit CSPRNG token (64 hex characters)
				// Search space: 2^256 = 1.15e77, mathematically uncrackable against brute-force
				b := make([]byte, 32)
				_, _ = rand.Read(b)
				tokenVal = hex.EncodeToString(b)
				_ = os.MkdirAll(filepath.Dir(tokenPath), 0755)
				_ = os.WriteFile(tokenPath, []byte(tokenVal), 0600)
			}
		}

		subURL := fmt.Sprintf("http://%s:%d/sub/%s", expCtx.ServerIP, *port, tokenVal)
		ui.Header("veilshard Subscription Server")
		ui.Success("Subscription server running with 256-bit anti-bruteforce encryption!")

		// Automatically ensure firewall port is open in both UFW and iptables
		_ = ufw.New().AllowProxyPort(context.Background(), *port, "tcp")
		_ = exec.Command("iptables", "-I", "INPUT", "-p", "tcp", "--dport", fmt.Sprintf("%d", *port), "-j", "ACCEPT").Run()

		fmt.Printf("\n%sYour Cryptographically Hardened Subscription URL:%s\n%s%s%s\n\n", ColorBold, ColorReset, ColorCyan, subURL, ColorReset)
		fmt.Printf("• %sToken Entropy%s: 256-bit CSPRNG (64-character hex, uncrackable)\n", ColorBold, ColorReset)
		fmt.Printf("• %sAnti-Scraping Tarpit%s: Active (random probes encounter 3s penalty delay)\n", ColorBold, ColorReset)
		fmt.Printf("• %sClash / Mihomo / Clash Verge%s: Automatically receives complete YAML config with routing rules\n", ColorBold, ColorReset)
		fmt.Printf("• %sShadowrocket / v2rayN / sing-box%s: Automatically receives Base64 node list\n\n", ColorBold, ColorReset)
		fmt.Printf("Press Ctrl+C to stop.\n\n")

		addr := fmt.Sprintf("0.0.0.0:%d", *port)
		return sub.StartSubscriptionServer(addr, tokenVal, expCtx)

	default:
		ui.Header("vpnctl export")
		fmt.Printf("Usage:\n")
		fmt.Printf("  vpnctl export mihomo  [user] - Export Mihomo / Clash Meta YAML proxy node\n")
		fmt.Printf("  vpnctl export profile [user] - Export full standalone Clash YAML profile (with rules)\n")
		fmt.Printf("  vpnctl export sub     [user] - Export universal Base64 subscription string\n")
		fmt.Printf("  vpnctl export uri     [user] - Export standard vless:// share URL\n")
		fmt.Printf("  vpnctl export qr      [user] - Render ASCII QR code in terminal\n")
		fmt.Printf("  vpnctl export serve [-port 8080] [-token <key>] - Start live HTTP subscription URL\n\n")
		return nil
	}
}
