package cli

import (
	"flag"
	"fmt"
	"os"

	"github.com/veilshard/veilshard/internal/users"
)

func runServeWebhook(args []string) error {
	fs := flag.NewFlagSet("serve-webhook", flag.ContinueOnError)
	port := fs.Int("port", 9091, "Webhook listener port")
	secret := fs.String("secret", "", "Bearer authorization secret token")

	if err := fs.Parse(args); err != nil {
		return err
	}

	ui := NewUI()
	ui.Header("vpnctl serve-webhook - Remote Revocation & Telegram Gateway")

	authSecret := *secret
	if authSecret == "" {
		authSecret = os.Getenv("WEBHOOK_SECRET")
	}

	addr := fmt.Sprintf(":%d", *port)
	ui.Success(fmt.Sprintf("Webhook listener active on %s", addr))
	fmt.Println()
	ui.KeyValue("Endpoint (Revoke)", fmt.Sprintf("POST http://localhost:%d/api/user/revoke", *port))
	ui.KeyValue("Endpoint (Telegram)", fmt.Sprintf("POST http://localhost:%d/api/telegram", *port))
	if authSecret != "" {
		ui.KeyValue("Authentication", "Bearer <SECRET>")
	} else {
		ui.KeyValue("Authentication", "None (Configure -secret in production)")
	}

	fmt.Printf("\n%sRevocation Example:%s\n", ColorCyan, ColorReset)
	fmt.Printf("curl -X POST http://localhost:%d/api/user/revoke \\\n", *port)
	fmt.Printf("  -H 'Authorization: Bearer %s' \\\n", authSecret)
	fmt.Printf("  -H 'Content-Type: application/json' \\\n")
	fmt.Printf("  -d '{\"username\": \"guest_bob\"}'\n\n")

	return users.StartRevocationWebhookServer(addr, authSecret, func(username string) error {
		ui.Warn(fmt.Sprintf("Remote webhook revoked user: %s (syncing core configuration...)", username))
		return syncCoreConfig(ui)
	})
}
