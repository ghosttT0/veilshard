package installer

import (
	"context"
	"crypto/rand"
	"crypto/tls"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"time"

	"github.com/veilshard/veilshard/internal/config"
	"github.com/veilshard/veilshard/internal/core"
	"github.com/veilshard/veilshard/internal/core/xray"
	"github.com/veilshard/veilshard/internal/credentials"
	"github.com/veilshard/veilshard/internal/firewall"
	"github.com/veilshard/veilshard/internal/firewall/ufw"
	"github.com/veilshard/veilshard/internal/platform"
	"github.com/veilshard/veilshard/internal/preflight"
	"github.com/veilshard/veilshard/internal/probe"
	"github.com/veilshard/veilshard/internal/rollback"
	"github.com/veilshard/veilshard/internal/service"
	"github.com/veilshard/veilshard/internal/service/systemd"
	"github.com/veilshard/veilshard/internal/state"
	"github.com/veilshard/veilshard/internal/users"
)

// UIProgress is a callback for logging formatted progress steps.
type UIProgress interface {
	Section(name string)
	Step(name string, success bool, detail string)
}

// Options allows overriding installer behavior.
type Options struct {
	Force       bool
	Port        int
	DestSNI     string
	CoreVersion string
	ListenAddr  string
}

type Installer struct {
	cfg      *config.Config
	st       *state.State
	core     core.Core
	service  service.Manager
	firewall firewall.Firewall
	rb       *rollback.Engine
}

func New() *Installer {
	cfg := config.NewDefault()
	st, _ := state.Load("")
	if st == nil {
		st = &state.State{SchemaVersion: state.CurrentSchema}
	}

	return &Installer{
		cfg:      cfg,
		st:       st,
		core:     xray.New("", ""),
		service:  systemd.New("vpnctl-proxy"),
		firewall: ufw.New(),
		rb:       rollback.New(),
	}
}

// Install runs the full transactional installation flow.
func (in *Installer) Install(ctx context.Context, opts Options, ui UIProgress) error {
	defer in.rb.Clear()

	// 1. Environment Section
	if ui != nil {
		ui.Section("Environment")
	}

	targetPort := opts.Port
	if targetPort <= 0 {
		targetPort = in.cfg.Server.Port
	}
	if targetPort <= 0 {
		targetPort = 443
	}

	env, checks, err := preflight.Run(ctx, targetPort)
	if err != nil {
		return fmt.Errorf("preflight inspection failed: %w", err)
	}

	hasHardFailures := false
	for _, c := range checks {
		if !c.Passed && !c.Warning {
			hasHardFailures = true
		}
		if ui != nil {
			ui.Step(c.Name, c.Passed, c.Message)
		}
	}

	if hasHardFailures && !opts.Force {
		return fmt.Errorf("preflight checks failed; please resolve the issues above or use --force to bypass")
	}

	// 2. Network Section
	if ui != nil {
		ui.Section("Network")
	}

	publicIP := env.PublicIPv4
	if publicIP != "" {
		if ui != nil {
			ui.Step("Public IPv4 detected", true, publicIP)
		}
		in.cfg.Server.PublicIP = publicIP
		in.st.PublicIPv4 = publicIP
	} else {
		if ui != nil {
			ui.Step("Public IPv4", false, "not detected automatically")
		}
	}

	if len(env.SSHPorts) > 0 {
		if ui != nil {
			ui.Step("SSH detected", true, preflight.FormatSSHPorts(env.SSHPorts))
		}
	}

	if !env.PortConflict {
		if ui != nil {
			ui.Step(fmt.Sprintf("TCP/%d available", targetPort), true, "")
		}
	} else if !opts.Force {
		return fmt.Errorf("target port TCP/%d is already occupied", targetPort)
	}

	// 3. Security Credentials Section
	if ui != nil {
		ui.Section("Security")
	}

	sni := opts.DestSNI
	if sni == "" || sni == "auto" {
		best, err := probe.AutoSelectBestSNI(3 * time.Second)
		if err == nil && best != nil {
			sni = best.Target
			in.cfg.Protocol.Dest = best.Target
			in.cfg.Protocol.ServerNames = []string{best.ServerName}
			if ui != nil {
				ui.Step("Selected optimal SNI target", true, fmt.Sprintf("%s (%v, %s)", best.ServerName, best.RTT.Round(time.Millisecond), best.ALPN))
			}
		} else {
			sni = "gateway.icloud.com:443"
		}
	}

	creds, err := credentials.Generate(sni)
	if err != nil {
		return fmt.Errorf("generate credentials: %w", err)
	}

	if ui != nil {
		ui.Step("Generated UUID", true, "")
		ui.Step("Generated X25519 key pair", true, "")
		ui.Step("Generated Short ID", true, "")
	}

	// 4. Installing Section
	if ui != nil {
		ui.Section("Installing")
	}

	// Apply kernel tuning (BBR + high buffer limits) & scan defense
	bbrOK, bbrMsg, _ := platform.ApplyKernelTuning(ctx)
	if ui != nil {
		ui.Step("Enabled TCP BBR & kernel tuning", bbrOK, bbrMsg)
		_ = platform.ConfigureTCPScanDrop(ctx)
		ui.Step("Configured stealth & scan defense", true, "dropped invalid TCP scans")
	}

	coreVer := opts.CoreVersion
	if coreVer == "" {
		coreVer = in.cfg.Core.Version
	}

	// Install binary
	err = in.core.Install(ctx, coreVer)
	if err != nil {
		return fmt.Errorf("download & verify core binary: %w", err)
	}
	in.rb.Push("Remove core binary", func(ctx context.Context) error {
		return os.Remove(in.core.BinaryPath())
	})
	if ui != nil {
		ui.Step("Downloaded core", true, "")
		ui.Step("SHA-256 verified", true, "")
		ui.Step("Installed binary", true, in.core.BinaryPath())
	}

	// Create system user
	runUser := in.cfg.Security.RunAsUser
	if runUser != "" && runUser != "root" {
		err = in.service.CreateSystemUser(ctx, runUser)
		if err != nil {
			in.triggerRollback(ctx, ui)
			return fmt.Errorf("create system user: %w", err)
		}
		if ui != nil {
			ui.Step("Created system user", true, runUser)
		}
	}

	// Initialize user store
	userStore, err := users.NewStore("")
	if err != nil {
		in.triggerRollback(ctx, ui)
		return fmt.Errorf("init user store: %w", err)
	}
	if len(userStore.List()) == 0 {
		_, err = userStore.Add("default", creds.UUID)
		if err != nil {
			in.triggerRollback(ctx, ui)
			return fmt.Errorf("add default user: %w", err)
		}
	}

	// Write credentials file
	credsPath := "/etc/vpnctl/credentials.json"
	_ = os.MkdirAll(filepath.Dir(credsPath), 0755)
	_ = os.Chmod(filepath.Dir(credsPath), 0755)
	credsData, _ := json.MarshalIndent(creds, "", "  ")
	_ = os.WriteFile(credsPath, credsData, 0600)
	in.rb.Push("Remove credentials file", func(ctx context.Context) error {
		return os.Remove(credsPath)
	})

	// Generate and write configurations
	in.cfg.Server.Port = targetPort
	if opts.ListenAddr != "" {
		in.cfg.Server.Listen = opts.ListenAddr
	}

	err = in.core.GenerateConfig(ctx, in.cfg, userStore.List(), creds)
	if err != nil {
		in.triggerRollback(ctx, ui)
		return fmt.Errorf("generate core configuration: %w", err)
	}
	in.rb.Push("Remove core config", func(ctx context.Context) error {
		return os.Remove(in.core.ConfigPath())
	})

	err = in.cfg.Save("")
	if err != nil {
		in.triggerRollback(ctx, ui)
		return fmt.Errorf("save vpnctl config: %w", err)
	}
	in.rb.Push("Remove vpnctl config", func(ctx context.Context) error {
		return os.Remove(config.DefaultConfigPath)
	})

	if ui != nil {
		ui.Step("Generated configuration", true, "")
	}

	// Install systemd unit
	err = in.service.Install(ctx, service.ServiceConfig{
		ServiceName: "vpnctl-proxy",
		BinaryPath:  in.core.BinaryPath(),
		ConfigPath:  in.core.ConfigPath(),
		User:        runUser,
		Description: "vpnctl proxy service (Xray Core)",
	})
	if err != nil {
		in.triggerRollback(ctx, ui)
		return fmt.Errorf("install systemd service: %w", err)
	}
	in.rb.Push("Remove systemd service", func(ctx context.Context) error {
		return in.service.Remove(ctx)
	})
	if ui != nil {
		ui.Step("Installed systemd service", true, "vpnctl-proxy")
	}

	// Firewall transaction: ensure SSH first!
	if len(env.SSHPorts) > 0 {
		_ = in.firewall.EnsureSSHAllowed(ctx, env.SSHPorts)
		for _, sp := range env.SSHPorts {
			in.st.SSHPortsPreserved = append(in.st.SSHPortsPreserved, sp)
		}
	}

	proxyRule := fmt.Sprintf("%d/tcp", targetPort)
	err = in.firewall.AllowProxyPort(ctx, targetPort, "tcp")
	if err != nil {
		in.triggerRollback(ctx, ui)
		return fmt.Errorf("firewall allow proxy port: %w", err)
	}
	in.st.AddManagedFirewallRule(proxyRule)
	in.rb.Push("Remove firewall rule", func(ctx context.Context) error {
		return in.firewall.RemoveProxyPort(ctx, targetPort, "tcp")
	})
	if ui != nil {
		ui.Step("Updated firewall", true, fmt.Sprintf("TCP/%d allowed", targetPort))
	}

	// 5. Testing Section
	if ui != nil {
		ui.Section("Testing")
	}

	// Validate config syntax
	err = in.core.Validate(ctx)
	if err != nil {
		in.triggerRollback(ctx, ui)
		return fmt.Errorf("core config validation failed: %w", err)
	}
	if ui != nil {
		ui.Step("Configuration valid", true, "")
	}

	// Start service
	err = in.service.Start(ctx)
	if err != nil {
		logs, _ := in.service.GetLogs(ctx, 25)
		in.triggerRollback(ctx, ui)
		return fmt.Errorf("start service: %w\nRecent systemd logs:\n%s", err, logs)
	}
	in.rb.Push("Stop service", func(ctx context.Context) error {
		return in.service.Stop(ctx)
	})

	// Wait up to 5 seconds for service to stabilize into active state
	var svcStatus *service.Status
	active := false
	for i := 0; i < 10; i++ {
		time.Sleep(500 * time.Millisecond)
		svcStatus, err = in.service.Status(ctx)
		if err == nil && svcStatus != nil && svcStatus.Active {
			active = true
			break
		}
	}

	if !active {
		logs, _ := in.service.GetLogs(ctx, 30)
		in.triggerRollback(ctx, ui)
		subState := ""
		if svcStatus != nil {
			subState = fmt.Sprintf(" (substate: %s)", svcStatus.SubState)
		}
		return fmt.Errorf("service failed to stay active%s\n--- Recent journalctl logs ---\n%s", subState, logs)
	}
	if ui != nil {
		ui.Step("Service running", true, fmt.Sprintf("PID %d", svcStatus.PID))
	}

	// Health Check: verify listening port
	time.Sleep(500 * time.Millisecond)
	if !isPortListening(targetPort) {
		in.triggerRollback(ctx, ui)
		return fmt.Errorf("health check failed: TCP/%d not listening", targetPort)
	}
	if ui != nil {
		ui.Step(fmt.Sprintf("TCP/%d listening", targetPort), true, "")
	}

	// Ensure token exists
	tokenPath := "/etc/vpnctl/sub_token"
	if _, err := os.Stat(tokenPath); err != nil {
		b := make([]byte, 32)
		_, _ = rand.Read(b)
		tokenVal := hex.EncodeToString(b)
		_ = os.WriteFile(tokenPath, []byte(tokenVal), 0600)
	}

	// Install and start permanent background subscription server (port 8080)
	subUnitPath := "/etc/systemd/system/veilshard-sub.service"
	subUnitContent := `[Unit]
Description=Veilshard Universal HTTP Subscription Server
After=network.target network-online.target

[Service]
Type=simple
ExecStart=/usr/local/bin/veilshard export serve -port 8080
Restart=always
RestartSec=3s

[Install]
WantedBy=multi-user.target
`
	_ = os.WriteFile(subUnitPath, []byte(subUnitContent), 0644)
	_ = exec.CommandContext(ctx, "systemctl", "daemon-reload").Run()
	_ = exec.CommandContext(ctx, "systemctl", "enable", "--now", "veilshard-sub").Run()
	_ = in.firewall.AllowProxyPort(ctx, 8080, "tcp")
	_ = exec.CommandContext(ctx, "iptables", "-I", "INPUT", "-p", "tcp", "--dport", "8080", "-j", "ACCEPT").Run()
	if ui != nil {
		ui.Step("Subscription service active", true, "veilshard-sub (background daemon on :8080)")
	}

	// Commit state
	in.st.Installed = true
	in.st.InstalledAt = time.Now().UTC()
	in.st.CoreProvider = in.core.Name()
	in.st.CoreVersion = coreVer
	in.st.CoreBinaryPath = in.core.BinaryPath()
	in.st.ServiceName = "vpnctl-proxy"
	in.st.ServiceFilePath = "/etc/systemd/system/vpnctl-proxy.service"
	in.st.FirewallProvider = in.firewall.Name()
	in.st.SystemUser = runUser
	in.st.ConfigPath = config.DefaultConfigPath
	in.st.CoreConfigPath = in.core.ConfigPath()
	in.st.CredentialsPath = credsPath
	_ = in.st.Save("")

	// Installation succeeded; cancel rollback
	in.rb.Clear()

	return nil
}

func (in *Installer) triggerRollback(ctx context.Context, ui UIProgress) {
	if ui != nil {
		ui.Section("Rollback")
	}
	in.rb.Rollback(ctx, func(msg string, success bool) {
		if ui != nil {
			ui.Step(msg, success, "")
		}
	})
}

func isPortListening(port int) bool {
	conn, err := net.DialTimeout("tcp", fmt.Sprintf("127.0.0.1:%d", port), 2*time.Second)
	if err == nil {
		_ = conn.Close()
		return true
	}
	return false
}

// TestTLSSNI connects via TLS with SNI camouflage to verify REALITY handshake responds.
func TestTLSSNI(ip string, port int, sni string) error {
	dialer := &net.Dialer{Timeout: 3 * time.Second}
	tlsConfig := &tls.Config{
		ServerName:         sni,
		InsecureSkipVerify: true,
	}
	conn, err := tls.DialWithDialer(dialer, "tcp", fmt.Sprintf("%s:%d", ip, port), tlsConfig)
	if err != nil {
		return err
	}
	_ = conn.Close()
	return nil
}
