package canary

import (
	"crypto/rand"
	"crypto/rsa"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"math/big"
	"net"
	"testing"
	"time"

	"github.com/veilshard/veilshard/internal/fleet"
)

func createSelfSignedTLSListener(t *testing.T) (net.Listener, int) {
	priv, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatalf("generate key: %v", err)
	}

	template := x509.Certificate{
		SerialNumber: big.NewInt(1),
		Subject: pkix.Name{
			Organization: []string{"Canary Test Org"},
		},
		NotBefore:             time.Now().Add(-1 * time.Hour),
		NotAfter:              time.Now().Add(24 * time.Hour),
		KeyUsage:              x509.KeyUsageKeyEncipherment | x509.KeyUsageDigitalSignature,
		ExtKeyUsage:           []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
		BasicConstraintsValid: true,
		DNSNames:              []string{"localhost", "test.vpnctl.internal"},
	}

	certDER, err := x509.CreateCertificate(rand.Reader, &template, &template, &priv.PublicKey, priv)
	if err != nil {
		t.Fatalf("create cert: %v", err)
	}

	cert := tls.Certificate{
		Certificate: [][]byte{certDER},
		PrivateKey:  priv,
	}

	tlsCfg := &tls.Config{Certificates: []tls.Certificate{cert}}
	ln, err := tls.Listen("tcp", "127.0.0.1:0", tlsCfg)
	if err != nil {
		t.Fatalf("tls listen: %v", err)
	}

	go func() {
		for {
			conn, err := ln.Accept()
			if err != nil {
				return
			}
			go func(c net.Conn) {
				defer c.Close()
				buf := make([]byte, 1024)
				_, _ = c.Read(buf)
			}(conn)
		}
	}()

	port := ln.Addr().(*net.TCPAddr).Port
	return ln, port
}

func TestProbeNodeFromClientSideAlive(t *testing.T) {
	ln, port := createSelfSignedTLSListener(t)
	defer ln.Close()

	node := fleet.NodeConfig{
		Name:      "local-mock-node",
		Host:      "127.0.0.1",
		Port:      port,
		SNITarget: "test.vpnctl.internal",
	}

	res := ProbeNodeFromClientSide(node, 2*time.Second)
	if res.State != StateAlive {
		t.Errorf("expected state ALIVE, got %s (err: %s)", res.State, res.ErrorMessage)
	}
	if res.TCPRTT == 0 || res.TLSRTT == 0 {
		t.Errorf("expected non-zero RTT, got TCP: %v, TLS: %v", res.TCPRTT, res.TLSRTT)
	}
}

func TestProbeNodeFromClientSideRefused(t *testing.T) {
	// Pick an unused local port
	node := fleet.NodeConfig{
		Name:      "dead-node",
		Host:      "127.0.0.1",
		Port:      64999,
		SNITarget: "gateway.icloud.com",
	}

	res := ProbeNodeFromClientSide(node, 500*time.Millisecond)
	if res.State != StateBlockedRST && res.State != StateTimeout {
		t.Errorf("expected state BLOCKED or TIMEOUT, got %s", res.State)
	}
}
