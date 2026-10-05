package cli

import (
	"fmt"
	"os"
)

var Version = "v0.2.0"

// Execute parses the CLI subcommands and executes the corresponding logic.
func Execute(args []string) error {
	if len(args) == 0 {
		printGlobalHelp()
		return nil
	}

	cmd := args[0]
	cmdArgs := args[1:]

	switch cmd {
	case "install":
		return runInstall(cmdArgs)
	case "status":
		return runStatus(cmdArgs)
	case "doctor":
		return runDoctor(cmdArgs)
	case "export":
		return runExport(cmdArgs)
	case "user":
		return runUser(cmdArgs)
	case "update":
		return runUpdate(cmdArgs)
	case "config":
		return runConfig(cmdArgs)
	case "logs":
		return runLogs(cmdArgs)
	case "probe":
		return runProbe(cmdArgs)
	case "apply":
		return runApply(cmdArgs)
	case "rotate":
		return runRotate(cmdArgs)
	case "healthcheck":
		return runHealthcheck(cmdArgs)
	case "publish":
		return runPublish(cmdArgs)
	case "recycle":
		return runRecycle(cmdArgs)
	case "defense":
		return runDefense(cmdArgs)
	case "quota":
		return runQuota(cmdArgs)
	case "init-gitops":
		return runInitGitOps(cmdArgs)
	case "serve-webhook":
		return runServeWebhook(cmdArgs)
	case "uninstall":
		return runUninstall(cmdArgs)
	case "version", "-v", "--version":
		fmt.Printf("veilshard %s (Ubuntu 24.04 amd64)\n", Version)
		return nil
	case "help", "-h", "--help":
		printGlobalHelp()
		return nil
	default:
		fmt.Fprintf(os.Stderr, "Unknown command: %s\n\n", cmd)
		printGlobalHelp()
		return fmt.Errorf("unknown command: %s", cmd)
	}
}

func printGlobalHelp() {
	ui := NewUI()
	ui.Header(fmt.Sprintf("veilshard %s - Single-Binary VPS Mesh Orchestration & Stealth Defense", Version))

	fmt.Printf("Usage:\n")
	fmt.Printf("  veilshard <command> [arguments]\n\n")

	fmt.Printf("Commands:\n")
	fmt.Printf("  %sinstall%s        Perform preflight checks, install Core, configure UFW & systemd\n", ColorCyan, ColorReset)
	fmt.Printf("  %sstatus%s         Display service, core, firewall, and connection statistics\n", ColorCyan, ColorReset)
	fmt.Printf("  %sdoctor%s         Run comprehensive diagnostic checks across system, network, and security\n", ColorCyan, ColorReset)
	fmt.Printf("  %sexport%s         Generate client configuration (Mihomo/Clash YAML, share URI, QR)\n", ColorCyan, ColorReset)
	fmt.Printf("  %suser%s           Manage proxy users (list, add, remove, enable, disable, revoke)\n", ColorCyan, ColorReset)
	fmt.Printf("  %supdate%s         Update proxy core with automatic rollback verification\n", ColorCyan, ColorReset)
	fmt.Printf("  %sconfig%s         Inspect current veilshard configuration\n", ColorCyan, ColorReset)
	fmt.Printf("  %sprobe%s          Benchmark & detect optimal low-latency TLS 1.3 / H2 SNI targets\n", ColorCyan, ColorReset)
	fmt.Printf("  %sapply%s          Declaratively orchestrate and export a multi-node fleet (veilshard.yaml)\n", ColorCyan, ColorReset)
	fmt.Printf("  %srotate%s         Zero-downtime ShortID & SNI rotation with multi-version grace period\n", ColorCyan, ColorReset)
	fmt.Printf("  %shealthcheck%s    Client-perspective TLS/QoS canary probe and block detection\n", ColorCyan, ColorReset)
	fmt.Printf("  %spublish%s        Zero-knowledge AES-256 encrypted subscription distribution (CF Workers)\n", ColorCyan, ColorReset)
	fmt.Printf("  %srecycle%s        Ephemeral cloud IP replacement (Hetzner, Vultr, DigitalOcean)\n", ColorCyan, ColorReset)
	fmt.Printf("  %sdefense%s        Active probing tarpit and slow-window deceptive blackhole\n", ColorCyan, ColorReset)
	fmt.Printf("  %squota%s          Zero-DB kernel bandwidth accounting & burst circuit breaker\n", ColorCyan, ColorReset)
	fmt.Printf("  %sinit-gitops%s    Scaffold zero-trust GitHub Actions headless orchestration\n", ColorCyan, ColorReset)
	fmt.Printf("  %sserve-webhook%s  Start authenticated webhook listener for remote token revocation\n", ColorCyan, ColorReset)
	fmt.Printf("  %suninstall%s      Safely remove service and managed rules without affecting SSH\n", ColorCyan, ColorReset)
	fmt.Printf("  %sversion%s        Show version information\n\n", ColorCyan, ColorReset)

	fmt.Printf("Examples:\n")
	fmt.Printf("  sudo veilshard install\n")
	fmt.Printf("  veilshard status\n")
	fmt.Printf("  sudo veilshard doctor\n")
	fmt.Printf("  veilshard probe\n")
	fmt.Printf("  veilshard apply -f veilshard.yaml\n")
	fmt.Printf("  veilshard healthcheck\n")
	fmt.Printf("  veilshard rotate\n")
	fmt.Printf("  veilshard publish -f dist/clash.yaml\n")
	fmt.Printf("  sudo veilshard defense\n")
	fmt.Printf("  veilshard quota -limit 800GB\n")
	fmt.Printf("  veilshard init-gitops ./my-fleet-repo\n")
	fmt.Printf("  sudo veilshard uninstall\n\n")
}

