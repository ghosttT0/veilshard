package probe

import (
	"crypto/tls"
	"fmt"
	"net"
	"sort"
	"strings"
	"sync"
	"time"
)

// DefaultCandidatePool lists high-reputation, global CDN targets supporting TLS 1.3 + H2.
var DefaultCandidatePool = []string{
	"gateway.icloud.com:443",
	"www.apple.com:443",
	"configuration.apple.com:443",
	"www.microsoft.com:443",
	"azure.microsoft.com:443",
	"aws.amazon.com:443",
	"s3.amazonaws.com:443",
	"cloudflare-dns.com:443",
	"finance.yahoo.com:443",
}

// Result contains benchmark and qualification details for an SNI camouflage target.
type Result struct {
	Target       string        `json:"target"`
	ServerName   string        `json:"server_name"`
	TLSVersion   string        `json:"tls_version"`
	IsTLS13      bool          `json:"is_tls13"`
	ALPN         string        `json:"alpn"`
	SupportsH2   bool          `json:"supports_h2"`
	CertSubject  string        `json:"cert_subject"`
	CertIssuer   string        `json:"cert_issuer"`
	CertExpiry   time.Time     `json:"cert_expiry"`
	RTT          time.Duration `json:"rtt"`
	Qualified    bool          `json:"qualified"`
	RejectReason string        `json:"reject_reason,omitempty"`
}

// TestTarget performs a full TLS 1.3 & ALPN H2 inspection on a target host:port.
func TestTarget(target string, timeout time.Duration) (*Result, error) {
	if !strings.Contains(target, ":") {
		target = target + ":443"
	}

	host, _, err := net.SplitHostPort(target)
	if err != nil {
		return nil, fmt.Errorf("invalid host:port: %w", err)
	}

	res := &Result{
		Target:     target,
		ServerName: host,
	}

	dialer := &net.Dialer{
		Timeout: timeout,
	}

	tlsConfig := &tls.Config{
		ServerName:         host,
		MinVersion:         tls.VersionTLS12,
		NextProtos:         []string{"h2", "http/1.1"},
		InsecureSkipVerify: false,
	}

	start := time.Now()
	conn, err := tls.DialWithDialer(dialer, "tcp", target, tlsConfig)
	if err != nil {
		res.RejectReason = fmt.Sprintf("TLS dial failed: %v", err)
		return res, nil
	}
	defer conn.Close()

	res.RTT = time.Since(start)

	state := conn.ConnectionState()
	res.ALPN = state.NegotiatedProtocol
	res.SupportsH2 = (res.ALPN == "h2")

	switch state.Version {
	case tls.VersionTLS13:
		res.TLSVersion = "TLS 1.3"
		res.IsTLS13 = true
	case tls.VersionTLS12:
		res.TLSVersion = "TLS 1.2"
		res.IsTLS13 = false
	default:
		res.TLSVersion = fmt.Sprintf("0x%04x", state.Version)
		res.IsTLS13 = false
	}

	if len(state.PeerCertificates) > 0 {
		leaf := state.PeerCertificates[0]
		res.CertSubject = leaf.Subject.CommonName
		res.CertIssuer = leaf.Issuer.CommonName
		res.CertExpiry = leaf.NotAfter
	}

	// Qualification check for REALITY: Must be TLS 1.3 and preferably H2
	if !res.IsTLS13 {
		res.RejectReason = "Target does not negotiate TLS 1.3"
	} else if time.Now().After(res.CertExpiry) {
		res.RejectReason = "Certificate is expired"
	} else {
		res.Qualified = true
	}

	return res, nil
}

// BenchmarkCandidates tests multiple candidate domains in parallel and returns qualified targets sorted by RTT.
func BenchmarkCandidates(candidates []string, timeout time.Duration) []*Result {
	if len(candidates) == 0 {
		candidates = DefaultCandidatePool
	}

	var results []*Result
	var mu sync.Mutex
	var wg sync.WaitGroup

	for _, c := range candidates {
		wg.Add(1)
		go func(target string) {
			defer wg.Done()
			r, err := TestTarget(target, timeout)
			if err == nil && r != nil {
				mu.Lock()
				results = append(results, r)
				mu.Unlock()
			}
		}(c)
	}

	wg.Wait()

	// Sort: Qualified first, then by lowest RTT
	sort.Slice(results, func(i, j int) bool {
		if results[i].Qualified != results[j].Qualified {
			return results[i].Qualified
		}
		return results[i].RTT < results[j].RTT
	})

	return results
}

// AutoSelectBestSNI benchmarks the pool and returns the optimal qualified candidate.
func AutoSelectBestSNI(timeout time.Duration) (*Result, error) {
	results := BenchmarkCandidates(DefaultCandidatePool, timeout)
	for _, r := range results {
		if r.Qualified {
			return r, nil
		}
	}
	return nil, fmt.Errorf("no qualified TLS 1.3 SNI targets found in candidate pool")
}
