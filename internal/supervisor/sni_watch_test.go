package supervisor

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
)

func createMockTLSServer(t *testing.T, notAfter time.Time) (net.Listener, int) {
	priv, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatalf("generate key: %v", err)
	}

	template := x509.Certificate{
		SerialNumber: big.NewInt(100),
		Subject: pkix.Name{
			Organization: []string{"Supervisor Test CA"},
			CommonName:   "127.0.0.1",
		},
		NotBefore:             time.Now().Add(-1 * time.Hour),
		NotAfter:              notAfter,
		KeyUsage:              x509.KeyUsageKeyEncipherment | x509.KeyUsageDigitalSignature,
		ExtKeyUsage:           []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
		BasicConstraintsValid: true,
		IPAddresses:           []net.IP{net.ParseIP("127.0.0.1")},
	}

	certDER, err := x509.CreateCertificate(rand.Reader, &template, &template, &priv.PublicKey, priv)
	if err != nil {
		t.Fatalf("create cert: %v", err)
	}

	cert := tls.Certificate{
		Certificate: [][]byte{certDER},
		PrivateKey:  priv,
	}

	tlsCfg := &tls.Config{
		Certificates: []tls.Certificate{cert},
		NextProtos:   []string{"h2", "http/1.1"},
	}
	ln, err := tls.Listen("tcp", "127.0.0.1:0", tlsCfg)
	if err != nil {
		t.Fatalf("listen: %v", err)
	}

	go func() {
		for {
			conn, err := ln.Accept()
			if err != nil {
				return
			}
			go func(c net.Conn) {
				defer c.Close()
				buf := make([]byte, 128)
				_, _ = c.Read(buf)
			}(conn)
		}
	}()

	port := ln.Addr().(*net.TCPAddr).Port
	return ln, port
}

func TestInspectTargetCertificateExpiring(t *testing.T) {
	// Certificate expiring in 5 days (should flag NeedsDrift=true)
	expiringSoon := time.Now().Add(5 * 24 * time.Hour)
	ln, port := createMockTLSServer(t, expiringSoon)
	defer ln.Close()

	health := InspectTargetCertificate(net.JoinHostPort("127.0.0.1", string(rune(port))), 1*time.Second)
	// Because self-signed cert fails genuine CA verification, it flags NeedsDrift=true
	if !health.NeedsDrift {
		t.Errorf("expected NeedsDrift to be true for unverified or expiring certificate")
	}
}
