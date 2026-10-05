package cli

import (
	"flag"
	"fmt"
	"strings"

	"github.com/veilshard/veilshard/internal/exporter"
	"github.com/veilshard/veilshard/internal/exporter/mihomo"
	"github.com/veilshard/veilshard/internal/exporter/qr"
	"github.com/veilshard/veilshard/internal/exporter/sub"
	"github.com/veilshard/veilshard/internal/exporter/uri"
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
			tokenVal = expCtx.ShortID
		}
		if tokenVal == "" {
			tokenVal = "secret"
		}

		subURL := fmt.Sprintf("http://%s:%d/sub/%s", expCtx.ServerIP, *port, tokenVal)
		ui.Header("vpnctl Subscription Server")
		ui.Success("Subscription server running!")
		fmt.Printf("\n%sYour Universal Subscription URL:%s\n%s%s%s\n\n", ColorBold, ColorReset, ColorCyan, subURL, ColorReset)
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
