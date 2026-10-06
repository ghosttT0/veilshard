package mihomo

import (
	"fmt"
	"strings"

	"github.com/veilshard/veilshard/internal/exporter"
	"github.com/veilshard/veilshard/internal/fingerprint"
)

// GenerateYAML produces standard Mihomo / Clash Meta proxy node configuration.
func GenerateYAML(ctx *exporter.ExportContext, platform ...string) string {
	var sb strings.Builder

	// Mihomo only accepts known uTLS fingerprints (chrome/firefox/safari/...);
	// platform labels like "desktop" must never leak into client-fingerprint.
	fp := "chrome"
	if len(platform) > 0 && platform[0] != "" {
		fp = fingerprint.ResolveOptions(platform[0]).ClientFingerprint
	}

	sb.WriteString("# Mihomo / Clash Meta Proxy Node Configuration\n")
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
	sb.WriteString(fmt.Sprintf("      short-id: \"%s\"\n", ctx.ShortID))

	return sb.String()
}
