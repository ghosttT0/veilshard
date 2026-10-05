package supervisor

import (
	"context"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"net"
	"os"
	"strings"
	"time"

	"github.com/veilshard/veilshard/internal/credentials"
	"github.com/veilshard/veilshard/internal/rotation"
)

type SNICertHealth struct {
	Domain         string        `json:"domain"`
	Issuer         string        `json:"issuer"`
	NotAfter       time.Time     `json:"not_after"`
	DaysRemaining  int           `json:"days_remaining"`
	TLSVersion     string        `json:"tls_version"`
	SupportsH2     bool          `json:"supports_h2"`
	HandshakeRTT   time.Duration `json:"handshake_rtt"`
	NeedsDrift     bool          `json:"needs_drift"`
	Status         string        `json:"status"` // "HEALTHY", "EXPIRING_SOON", "UNHEALTHY"
	ErrorMessage   string        `json:"error_message,omitempty"`
}

// InspectTargetCertificate verifies live certificate health and expiration of a REALITY camouflage target.
func InspectTargetCertificate(targetDomain string, timeout time.Duration) *SNICertHealth {
	domain := strings.TrimSpace(targetDomain)
	if strings.Contains(domain, ":") {
		domain = strings.Split(domain, ":")[0]
	}

	res := &SNICertHealth{
		Domain: domain,
	}

	dialer := &net.Dialer{Timeout: timeout}
	start := time.Now()
	conn, err := tls.DialWithDialer(dialer, "tcp", domain+":443", &tls.Config{
		ServerName:         domain,
		NextProtos:         []string{"h2", "http/1.1"},
		InsecureSkipVerify: false, // Must be genuinely valid CA certificate for REALITY
	})

	if err != nil {
		res.NeedsDrift = true
		res.Status = "UNHEALTHY"
		res.ErrorMessage = fmt.Sprintf("TLS dial error: %v", err)
		return res
	}
	defer conn.Close()

	res.HandshakeRTT = time.Since(start)
	cs := conn.ConnectionState()

	if cs.Version == tls.VersionTLS13 {
		res.TLSVersion = "TLS 1.3"
	} else if cs.Version == tls.VersionTLS12 {
		res.TLSVersion = "TLS 1.2"
	} else {
		res.TLSVersion = fmt.Sprintf("0x%04x", cs.Version)
	}

	for _, p := range cs.NegotiatedProtocol {
		_ = p
	}
	res.SupportsH2 = (cs.NegotiatedProtocol == "h2")

	if len(cs.PeerCertificates) > 0 {
		leaf := cs.PeerCertificates[0]
		res.Issuer = leaf.Issuer.CommonName
		if res.Issuer == "" && len(leaf.Issuer.Organization) > 0 {
			res.Issuer = leaf.Issuer.Organization[0]
		}
		res.NotAfter = leaf.NotAfter
		days := int(time.Until(leaf.NotAfter).Hours() / 24)
		res.DaysRemaining = days

		if days < 15 || !res.SupportsH2 || cs.Version != tls.VersionTLS13 {
			res.NeedsDrift = true
			if days < 15 {
				res.Status = "EXPIRING_SOON"
			} else {
				res.Status = "UNHEALTHY"
			}
		} else {
			res.NeedsDrift = false
			res.Status = "HEALTHY"
		}
	} else {
		res.NeedsDrift = true
		res.Status = "UNHEALTHY"
		res.ErrorMessage = "No peer certificates presented"
	}

	return res
}

// DriftToFreshSNIIfUnhealthy inspects the local node and triggers zero-downtime rotation if camouflage health degrades.
func DriftToFreshSNIIfUnhealthy(ctx context.Context, thresholdDays int) (*SNICertHealth, *rotation.RotationResult, error) {
	credsPath := "/etc/vpnctl/credentials.json"
	data, err := os.ReadFile(credsPath)
	if err != nil {
		return nil, nil, fmt.Errorf("read credentials: %w", err)
	}

	var creds credentials.Credentials
	if err := json.Unmarshal(data, &creds); err != nil {
		return nil, nil, fmt.Errorf("parse credentials: %w", err)
	}

	health := InspectTargetCertificate(creds.ServerName, 4*time.Second)

	if health.NeedsDrift || (thresholdDays > 0 && health.DaysRemaining <= thresholdDays) {
		// Target website certificate is expiring or unviable; drift to optimal fresh candidate
		rotResult, err := rotation.RotateLocalNode(ctx, true, "")
		if err != nil {
			return health, nil, fmt.Errorf("auto-drift rotation failed: %w", err)
		}
		return health, rotResult, nil
	}

	return health, nil, nil
}
