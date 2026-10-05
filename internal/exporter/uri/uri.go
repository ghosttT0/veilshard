package uri

import (
	"fmt"
	"net/url"

	"github.com/veilshard/veilshard/internal/exporter"
)

// GenerateURI produces a standard VLESS-REALITY share link.
// Format: vless://<uuid>@<host>:<port>?type=tcp&security=reality&pbk=<pubkey>&fp=chrome&sni=<sni>&sid=<shortid>&encryption=none#<name>
func GenerateURI(ctx *exporter.ExportContext) string {
	q := url.Values{}
	q.Set("type", "tcp")
	q.Set("security", "reality")
	q.Set("pbk", ctx.PublicKey)
	q.Set("fp", "chrome")
	q.Set("sni", ctx.ServerName)
	q.Set("sid", ctx.ShortID)
	q.Set("encryption", "none")
	q.Set("headerType", "none")

	u := url.URL{
		Scheme:   "vless",
		User:     url.User(ctx.UUID),
		Host:     fmt.Sprintf("%s:%d", ctx.ServerIP, ctx.Port),
		RawQuery: q.Encode(),
		Fragment: ctx.NodeName,
	}

	return u.String()
}
