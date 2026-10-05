package mihomo

import (
	"fmt"
	"strings"

	"github.com/veilshard/veilshard/internal/exporter"
)

// GenerateYAML produces standard Mihomo / Clash Meta proxy node configuration.
func GenerateYAML(ctx *exporter.ExportContext, platform ...string) string {
	var sb strings.Builder

	fp := "chrome"
	if len(platform) > 0 && (platform[0] == "ios" || strings.Contains(strings.ToLower(platform[0]), "safari")) {
		fp = "safari"
	} else if len(platform) > 0 && platform[0] != "" {
		fp = platform[0]
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
	sb.WriteString("    flow: \"\"\n")
	sb.WriteString(fmt.Sprintf("    servername: \"%s\"\n", ctx.ServerName))
	sb.WriteString(fmt.Sprintf("    client-fingerprint: %s\n", fp))
	sb.WriteString("    reality-opts:\n")
	sb.WriteString(fmt.Sprintf("      public-key: \"%s\"\n", ctx.PublicKey))
	sb.WriteString(fmt.Sprintf("      short-id: \"%s\"\n", ctx.ShortID))

	return sb.String()
}
