package cli

import (
	"fmt"
	"os"
	"time"

	"github.com/veilshard/veilshard/internal/canary"
	"github.com/veilshard/veilshard/internal/fleet"
)

func runHealthcheck(args []string) error {
	ui := NewUI()
	ui.Header("veilshard healthcheck - Client-Perspective Canary Health Dashboard")

	fleetPath := "veilshard.yaml"
	if _, err := os.Stat("veilshard.yaml"); err != nil {
		if _, errOld := os.Stat("vpnctl.yaml"); errOld == nil {
			fleetPath = "vpnctl.yaml"
		}
	}
	if len(args) > 0 && args[0] != "" {
		fleetPath = args[0]
	}

	cfg, err := fleet.LoadFleetConfig(fleetPath)
	if err != nil {
		// Fallback to local node probe if vpnctl.yaml not present
		ui.Warn(fmt.Sprintf("Fleet file %s not found, checking local host...", fleetPath))
		cfg = &fleet.FleetConfig{
			Nodes: []fleet.NodeConfig{
				{
					Name:      "local-node",
					Host:      "127.0.0.1",
					Port:      443,
					SNITarget: "gateway.icloud.com",
				},
			},
		}
	}

	fmt.Printf("Probing %d nodes from local perspective (TLS handshake & QoS detection)...\n\n", len(cfg.Nodes))

	results := canary.ProbeFleetHealth(cfg.Nodes, 5*time.Second)

	headers := []string{"NODE", "STATE", "TARGET", "TCP RTT", "TLS RTT", "TOTAL RTT", "DIAGNOSIS"}
	rows := make([][]string, len(results))

	aliveCount := 0
	blockedCount := 0
	qosCount := 0

	for i, r := range results {
		stateStr := fmt.Sprintf("%s%s%s", ColorGreen, r.State, ColorReset)
		switch r.State {
		case canary.StateAlive:
			aliveCount++
		case canary.StateBlockedRST, canary.StateTimeout:
			stateStr = fmt.Sprintf("%s%s%s", ColorRed, r.State, ColorReset)
			blockedCount++
		case canary.StateQoSThrottled:
			stateStr = fmt.Sprintf("%s%s%s", ColorYellow, r.State, ColorReset)
			qosCount++
		}

		diag := "Normal"
		if r.ErrorMessage != "" {
			diag = r.ErrorMessage
			if len(diag) > 30 {
				diag = diag[:30] + "..."
			}
		} else if r.State == canary.StateQoSThrottled {
			diag = "Carrier throttling detected"
		}

		rows[i] = []string{
			r.Name,
			stateStr,
			fmt.Sprintf("%s:%d", r.Host, r.Port),
			fmt.Sprintf("%v", r.TCPRTT.Round(time.Millisecond)),
			fmt.Sprintf("%v", r.TLSRTT.Round(time.Millisecond)),
			fmt.Sprintf("%v", r.TotalRTT.Round(time.Millisecond)),
			diag,
		}
	}

	ui.Table(headers, rows)
	fmt.Println()

	ui.Section("Health Summary")
	fmt.Printf("• %sALIVE:%s %d   • %sBLOCKED / RST:%s %d   • %sHIGH_LATENCY / QOS:%s %d\n\n",
		ColorGreen, ColorReset, aliveCount,
		ColorRed, ColorReset, blockedCount,
		ColorYellow, ColorReset, qosCount)

	if blockedCount > 0 {
		fmt.Printf("%sWarning: %d node(s) appear blocked or unreachable from your network location.%s\n", ColorRed, blockedCount, ColorReset)
		fmt.Printf("Tip: You can quickly recycle blocked cloud instances with: %svpnctl recycle <node-name>%s\n\n", ColorCyan, ColorReset)
	}

	return nil
}
