package cli

import (
	"context"
	"fmt"
	"time"

	"github.com/veilshard/veilshard/internal/probe"
	"github.com/veilshard/veilshard/internal/supervisor"
)

func runProbe(args []string) error {
	ui := NewUI()
	ui.Header("vpnctl probe - SNI Camouflage Target Benchmark & Drift Supervisor")

	timeout := 4 * time.Second

	// Drift and certificate health inspection mode
	if len(args) > 0 && (args[0] == "--watch" || args[0] == "-watch" || args[0] == "--drift" || args[0] == "drift") {
		ctx := context.Background()
		health, rot, err := supervisor.DriftToFreshSNIIfUnhealthy(ctx, 15)
		if err != nil {
			ui.Error(fmt.Sprintf("SNI health check error: %v", err))
			return err
		}
		ui.Section("Current Camouflage Target Certificate Health")
		ui.KeyValue("Domain", health.Domain)
		ui.KeyValue("Health Status", health.Status)
		ui.KeyValue("Cert Issuer", health.Issuer)
		ui.KeyValue("Expires In", fmt.Sprintf("%d days (%s)", health.DaysRemaining, health.NotAfter.Format("2006-01-02")))
		ui.KeyValue("TLS Protocol", fmt.Sprintf("%s (H2: %v)", health.TLSVersion, health.SupportsH2))
		ui.KeyValue("Handshake RTT", fmt.Sprintf("%v", health.HandshakeRTT.Round(time.Millisecond)))

		fmt.Println()
		if rot != nil {
			ui.Success(fmt.Sprintf("Active target degraded/expiring soon! Auto-drifted to: %s", rot.NewSNI))
			ui.KeyValue("New ShortID", rot.NewShortID)
			ui.KeyValue("Graceful Core Reload", "COMPLETED (Zero Downtime)")
		} else {
			ui.Success("Camouflage certificate is in good standing. No drift required.")
		}
		fmt.Println()
		return nil
	}

	// Single target mode
	if len(args) > 0 {
		target := args[0]
		fmt.Printf("Probing target: %s%s%s ...\n\n", ColorCyan, target, ColorReset)

		res, err := probe.TestTarget(target, timeout)
		if err != nil {
			ui.Error(fmt.Sprintf("Probe failed: %v", err))
			return err
		}

		ui.KeyValue("Target", res.Target)
		ui.KeyValue("Server Name", res.ServerName)
		ui.KeyValue("TLS Version", res.TLSVersion)
		ui.KeyValue("ALPN", res.ALPN)
		ui.KeyValue("Latency (RTT)", fmt.Sprintf("%v", res.RTT.Round(time.Millisecond)))
		if res.CertSubject != "" {
			ui.KeyValue("Cert Subject", res.CertSubject)
			ui.KeyValue("Cert Issuer", res.CertIssuer)
			ui.KeyValue("Cert Expiry", res.CertExpiry.Format("2006-01-02"))
		}

		fmt.Println()
		if res.Qualified {
			ui.Success("QUALIFIED: This target supports TLS 1.3 and is fully compliant for REALITY camouflage.")
		} else {
			ui.Error(fmt.Sprintf("REJECTED: %s", res.RejectReason))
		}
		fmt.Println()
		return nil
	}

	// Benchmark candidate pool mode
	fmt.Printf("Benchmarking %d candidate targets in parallel...\n\n", len(probe.DefaultCandidatePool))

	results := probe.BenchmarkCandidates(probe.DefaultCandidatePool, timeout)

	headers := []string{"STATUS", "TARGET", "TLS", "ALPN", "RTT", "ISSUER"}
	rows := make([][]string, len(results))

	var best *probe.Result
	for i, r := range results {
		status := fmt.Sprintf("%sPASS%s", ColorGreen, ColorReset)
		if !r.Qualified {
			status = fmt.Sprintf("%sFAIL%s", ColorRed, ColorReset)
		} else if best == nil {
			best = r
		}

		alpnStr := r.ALPN
		if alpnStr == "" {
			alpnStr = "none"
		}

		issuerStr := r.CertIssuer
		if len(issuerStr) > 20 {
			issuerStr = issuerStr[:20]
		}

		rows[i] = []string{
			status,
			r.Target,
			r.TLSVersion,
			alpnStr,
			fmt.Sprintf("%v", r.RTT.Round(time.Millisecond)),
			issuerStr,
		}
	}

	ui.Table(headers, rows)
	fmt.Println()

	if best != nil {
		ui.Success(fmt.Sprintf("Optimal SNI Target: %s%s%s (RTT: %v, %s, ALPN: %s)",
			ColorBold+ColorCyan, best.Target, ColorReset,
			best.RTT.Round(time.Millisecond), best.TLSVersion, best.ALPN))
		fmt.Printf("To use this target in installation:\n  sudo vpnctl install --dest %s\n\n", best.ServerName)
	} else {
		ui.Warn("No qualified targets found in current network environment.")
	}

	return nil
}
