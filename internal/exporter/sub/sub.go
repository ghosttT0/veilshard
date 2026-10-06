package sub

import (
	"encoding/base64"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/veilshard/veilshard/internal/exporter"
	"github.com/veilshard/veilshard/internal/exporter/uri"
	"github.com/veilshard/veilshard/internal/fingerprint"
	"github.com/veilshard/veilshard/internal/panel"
	"github.com/veilshard/veilshard/internal/traffic"
	"github.com/veilshard/veilshard/internal/users"
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

	// Mihomo only accepts known uTLS fingerprints; "desktop" and other
	// platform labels must be normalized, never emitted verbatim.
	fp := "chrome"
	if len(platform) > 0 && platform[0] != "" {
		fp = fingerprint.ResolveOptions(platform[0]).ClientFingerprint
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
	sb.WriteString("    flow: xtls-rprx-vision\n")
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

// StartSubscriptionServer serves per-user universal subscription requests over
// HTTP and mounts the admin panel on the same port. Clash clients receive the
// full YAML profile, everything else gets Base64 for Shadowrocket/v2rayN.
// The legacy global token resolves to the default user; every user also gets
// an individual token from the user store.
func StartSubscriptionServer(addr string, globalToken string, defaultCtx *exporter.ExportContext, adminKey string) error {
	mux := http.NewServeMux()

	mux.HandleFunc("/sub/", func(w http.ResponseWriter, r *http.Request) {
		token := strings.TrimPrefix(r.URL.Path, "/sub/")

		ctx := defaultCtx
		if token != globalToken {
			store, err := users.NewStore("")
			if err != nil {
				tarpit(w, r)
				return
			}
			u, err := store.FindBySubToken(token)
			if err != nil || !u.IsActive() {
				// Unknown or inactive token: same tarpit as any other invalid
				// path so probes cannot distinguish existing users.
				tarpit(w, r)
				return
			}
			loaded, err := exporter.LoadExportContext(u.Name)
			if err != nil {
				tarpit(w, r)
				return
			}
			ctx = loaded
		}
		if ctx == nil {
			tarpit(w, r)
			return
		}

		userAgent := strings.ToLower(r.UserAgent())
		platform := "desktop"
		if strings.Contains(userAgent, "iphone") || strings.Contains(userAgent, "ipad") || strings.Contains(userAgent, "shadowrocket") {
			platform = "ios"
		}

		userinfo := UserInfoHeader(ctx)

		// If requester is Clash / Mihomo / ClashVerge or requests YAML
		if strings.Contains(userAgent, "clash") || strings.Contains(userAgent, "mihomo") || r.URL.Query().Get("type") == "clash" || strings.HasSuffix(r.URL.Path, ".yaml") || strings.HasSuffix(r.URL.Path, "/clash") {
			w.Header().Set("Content-Type", "text/yaml; charset=utf-8")
			w.Header().Set("Subscription-Userinfo", userinfo)
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(GenerateFullClashProfile(ctx, platform)))
			return
		}

		// Otherwise, serve universal Base64 string for Shadowrocket, v2rayN, etc.
		w.Header().Set("Content-Type", "text/plain; charset=utf-8")
		w.Header().Set("Subscription-Userinfo", userinfo)
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(GenerateBase64Subscription(ctx)))
	})

	panel.Mount(mux, adminKey)

	// Anti-probing tarpit for any unauthorized / invalid paths:
	// Slow down brute-force scanners by delaying response by 3 seconds
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		tarpit(w, r)
	})

	return http.ListenAndServe(addr, mux)
}

func tarpit(w http.ResponseWriter, r *http.Request) {
	time.Sleep(3 * time.Second)
	http.NotFound(w, r)
}

// UserInfoHeader builds the Subscription-Userinfo header from live per-user
// counters and the user's quota/expiry. Clash renders it as the traffic bar.
func UserInfoHeader(ctx *exporter.ExportContext) string {
	upload, download := int64(0), int64(0)
	if snap, err := traffic.Query(); err == nil {
		if usage, ok := traffic.ForUser(snap, ctx.UserName); ok {
			upload, download = usage.Up, usage.Down
		}
	}
	var expire int64
	if ctx.ExpiresAt != nil {
		expire = ctx.ExpiresAt.Unix()
	}
	return fmt.Sprintf("upload=%d; download=%d; total=%d; expire=%d", upload, download, ctx.QuotaBytes, expire)
}
