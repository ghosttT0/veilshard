package cli

import (
	"context"
	"flag"
	"fmt"
	"os"

	"github.com/veilshard/veilshard/internal/quota"
)

func runQuota(args []string) error {
	fs := flag.NewFlagSet("quota", flag.ContinueOnError)
	limit := fs.String("limit", "800GB", "Monthly traffic limit (e.g. 500GB, 800GB, 1TB)")
	port := fs.Int("port", 443, "Monitored proxy port")
	webhook := fs.String("webhook", "", "Webhook URL for alerts (Telegram, Bark, or generic HTTP POST)")
	reset := fs.Bool("reset", false, "Reset accounting byte counters for new billing cycle")

	if err := fs.Parse(args); err != nil {
		return err
	}

	ui := NewUI()
	ui.Header("vpnctl quota - Zero-DB Bandwidth Accounting & Edge Circuit Breaker")

	if *reset {
		if err := quota.ResetCounters(); err != nil {
			ui.Error(fmt.Sprintf("Failed to reset counters: %v", err))
			return err
		}
		ui.Success("Kernel accounting counters reset to zero for new billing cycle!")
		return nil
	}

	// Ensure accounting rules are active
	_ = quota.EnsureAccountingRules(*port)

	targetWebhook := *webhook
	if targetWebhook == "" {
		targetWebhook = os.Getenv("WEBHOOK_URL")
	}

	ctx := context.Background()
	status, err := quota.CheckAndEnforce(ctx, *limit, *port, targetWebhook)
	if err != nil {
		ui.Error(fmt.Sprintf("Quota check failed: %v", err))
		return err
	}

	stateColor := ColorGreen
	switch status.State {
	case "DRAINING":
		stateColor = ColorYellow
	case "BLOCKED":
		stateColor = ColorRed
	}

	ui.Section("Bandwidth Utilization (Zero-DB Kernel Accounting)")
	ui.KeyValue("Monthly Quota", status.QuotaHuman)
	ui.KeyValue("Current Usage", status.UsedHuman)
	ui.KeyValue("Utilization", fmt.Sprintf("%.2f%%", status.UsagePercent))
	ui.KeyValue("Node State", fmt.Sprintf("%s%s%s", stateColor, status.State, ColorReset))

	// Render progress bar
	renderProgressBar(status.UsagePercent)

	fmt.Println()
	if status.State == "BLOCKED" {
		fmt.Printf("%s[CRITICAL ALERT]%s Usage exceeded 98%%! Edge circuit breaker triggered to prevent burst billing.\n", ColorRed, ColorReset)
	} else if status.State == "DRAINING" {
		fmt.Printf("%s[WARNING]%s Usage exceeded 90%%! Node marked as draining for smooth failover.\n", ColorYellow, ColorReset)
	} else {
		fmt.Printf("%s[NORMAL]%s Node bandwidth within safe operating parameters.\n", ColorGreen, ColorReset)
	}
	fmt.Println()

	return nil
}

func renderProgressBar(pct float64) {
	width := 30
	filled := int((pct / 100.0) * float64(width))
	if filled > width {
		filled = width
	}
	if filled < 0 {
		filled = 0
	}

	bar := ""
	for i := 0; i < filled; i++ {
		bar += "█"
	}
	for i := filled; i < width; i++ {
		bar += "░"
	}

	color := ColorGreen
	if pct >= 90.0 {
		color = ColorYellow
	}
	if pct >= 98.0 {
		color = ColorRed
	}

	fmt.Printf("Progress: [%s%s%s] %.1f%%\n", color, bar, ColorReset, pct)
}
