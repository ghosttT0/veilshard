package doctor

type Status string

const (
	StatusPass Status = "PASS"
	StatusWarn Status = "WARN"
	StatusFail Status = "FAIL"
)

type CheckItem struct {
	ID             string `json:"id"`
	Category       string `json:"category"`
	Name           string `json:"name"`
	Status         Status `json:"status"`
	Message        string `json:"message"`
	Recommendation string `json:"recommendation,omitempty"`
}

type Report struct {
	Items []CheckItem `json:"items"`
	Pass  int         `json:"pass"`
	Warn  int         `json:"warn"`
	Fail  int         `json:"fail"`
}

// Explanations stores in-depth reasoning and recommendations for rules.
var Explanations = map[string]CheckItem{
	"SYS_OS": {
		ID:             "SYS_OS",
		Category:       "System",
		Name:           "Supported OS",
		Recommendation: "vpnctl v0.1 targets Ubuntu 24.04 LTS. Other distributions may experience package or service incompatibilities.",
	},
	"SYS_ARCH": {
		ID:             "SYS_ARCH",
		Category:       "System",
		Name:           "Architecture",
		Recommendation: "Ensure you are running on an amd64 (x86_64) CPU architecture.",
	},
	"SYS_CLOCK": {
		ID:             "SYS_CLOCK",
		Category:       "System",
		Name:           "Clock synchronization",
		Recommendation: "Proxy protocols (especially TLS/REALITY and VMess/VLESS) require server time to be accurate within 60 seconds. Enable systemd-timesyncd or chrony.",
	},
	"SEC_SSH_RULE": {
		ID:             "SEC_SSH_RULE",
		Category:       "Security",
		Name:           "SSH rule preserved",
		Recommendation: "UFW must explicitly allow your active SSH port before other changes so you do not get locked out.",
	},
	"SEC_ROOT": {
		ID:             "SEC_ROOT",
		Category:       "Security",
		Name:           "Core isn't running as root",
		Recommendation: "For defense-in-depth, Xray core should run under the non-privileged system user 'vpnctl-proxy' with CAP_NET_BIND_SERVICE.",
	},
	"SEC_PERM": {
		ID:             "SEC_PERM",
		Category:       "Security",
		Name:           "Config permission",
		Recommendation: "Core configurations containing private keys must have file permissions 0600 or 0640.",
	},
	"NET_PORT": {
		ID:             "NET_PORT",
		Category:       "Network",
		Name:           "Proxy port listening",
		Recommendation: "Check if another service (such as nginx, caddy, or apache) is occupying port 443.",
	},
	"SRV_LOOP": {
		ID:             "SRV_LOOP",
		Category:       "Service",
		Name:           "No restart loop",
		Recommendation: "High restart counts indicate service crash loops. Check logs with 'vpnctl logs' or 'journalctl -u vpnctl-proxy'.",
	},
}
