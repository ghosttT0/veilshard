package sub

import (
	"encoding/base64"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/veilshard/veilshard/internal/exporter"
	"github.com/veilshard/veilshard/internal/exporter/uri"
)

// GenerateBase64Subscription generates universal Base64 subscription content
// compatible with Shadowrocket (小火箭), v2rayN, v2rayNG, sing-box, Quantumult X, etc.
func GenerateBase64Subscription(ctx *exporter.ExportContext) string {
	rawURI := uri.GenerateURI(ctx)
	// Universal subscription format: newline-delimited share URIs encoded in standard Base64
	return base64.StdEncoding.EncodeToString([]byte(rawURI + "\n"))
}

// GenerateFullClashProfile produces a complete, production-ready Clash / Mihomo configuration
// with DNS, routing rules, and auto-fallback proxy groups.
func GenerateFullClashProfile(ctx *exporter.ExportContext, platform ...string) string {
	var sb strings.Builder

	fp := "chrome"
	if len(platform) > 0 && (platform[0] == "ios" || strings.Contains(strings.ToLower(platform[0]), "safari")) {
		fp = "safari"
	} else if len(platform) > 0 && platform[0] != "" {
		fp = platform[0]
	}

	sb.WriteString("# vpnctl Full Profile for Mihomo / Clash Meta\n")
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
	sb.WriteString(fmt.Sprintf("  - name: \"%s\"\n", ctx.NodeName))
	sb.WriteString("    type: vless\n")
	sb.WriteString(fmt.Sprintf("    server: \"%s\"\n", ctx.ServerIP))
	sb.WriteString(fmt.Sprintf("    port: %d\n", ctx.Port))
	sb.WriteString(fmt.Sprintf("    uuid: \"%s\"\n", ctx.UUID))
	sb.WriteString("    network: tcp\n")
	sb.WriteString("    tls: true\n")
	sb.WriteString("    udp: true\n")
	sb.WriteString("    flow: \"\"\n")
	sb.WriteString(fmt.Sprintf("    servername: \"%s\"\n", ctx.ServerName))
	sb.WriteString(fmt.Sprintf("    client-fingerprint: %s\n", fp))
	sb.WriteString("    reality-opts:\n")
	sb.WriteString(fmt.Sprintf("      public-key: \"%s\"\n", ctx.PublicKey))
	sb.WriteString(fmt.Sprintf("      short-id: \"%s\"\n\n", ctx.ShortID))

	sb.WriteString("proxy-groups:\n")
	sb.WriteString("  - name: \"🚀 节点选择\"\n")
	sb.WriteString("    type: select\n")
	sb.WriteString(fmt.Sprintf("    proxies:\n      - \"%s\"\n      - DIRECT\n\n", ctx.NodeName))

	sb.WriteString("  - name: \"⚡ 自动选择\"\n")
	sb.WriteString("    type: url-test\n")
	sb.WriteString("    url: http://www.gstatic.com/generate_204\n")
	sb.WriteString("    interval: 300\n")
	sb.WriteString(fmt.Sprintf("    proxies:\n      - \"%s\"\n\n", ctx.NodeName))

	sb.WriteString("rules:\n")
	sb.WriteString("  - DOMAIN-SUFFIX,local,DIRECT\n")
	sb.WriteString("  - IP-CIDR,127.0.0.0/8,DIRECT\n")
	sb.WriteString("  - IP-CIDR,172.16.0.0/12,DIRECT\n")
	sb.WriteString("  - IP-CIDR,192.168.0.0/16,DIRECT\n")
	sb.WriteString("  - IP-CIDR,10.0.0.0/8,DIRECT\n")
	sb.WriteString("  - GEOIP,CN,DIRECT\n")
	sb.WriteString("  - MATCH,🚀 节点选择\n")

	return sb.String()
}

// StartSubscriptionServer serves universal subscription requests over HTTP.
// Automatically serves Clash YAML for Clash clients and Base64 for Shadowrocket/v2rayN!
func StartSubscriptionServer(addr string, token string, ctx *exporter.ExportContext) error {
	mux := http.NewServeMux()

	path := fmt.Sprintf("/sub/%s", token)
	mux.HandleFunc(path, func(w http.ResponseWriter, r *http.Request) {
		userAgent := strings.ToLower(r.UserAgent())
		platform := "desktop"
		if strings.Contains(userAgent, "iphone") || strings.Contains(userAgent, "ipad") || strings.Contains(userAgent, "shadowrocket") {
			platform = "ios"
		}

		// If requester is Clash / Mihomo / ClashVerge
		if strings.Contains(userAgent, "clash") || strings.Contains(userAgent, "mihomo") || r.URL.Query().Get("type") == "clash" {
			w.Header().Set("Content-Type", "text/yaml; charset=utf-8")
			w.Header().Set("Subscription-Userinfo", "upload=0; download=0; total=107374182400; expire=0")
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(GenerateFullClashProfile(ctx, platform)))
			return
		}

		// Otherwise, serve universal Base64 string for Shadowrocket, v2rayN, etc.
		w.Header().Set("Content-Type", "text/plain; charset=utf-8")
		w.Header().Set("Subscription-Userinfo", "upload=0; download=0; total=107374182400; expire=0")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(GenerateBase64Subscription(ctx)))
	})

	// Anti-probing tarpit for any unauthorized / invalid paths:
	// Slow down brute-force scanners by delaying response by 3 seconds
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(3 * time.Second)
		http.NotFound(w, r)
	})

	return http.ListenAndServe(addr, mux)
}
