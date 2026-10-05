package credentials

import (
	"crypto/ecdh"
	"crypto/rand"
	"encoding/base64"
	"encoding/hex"
	"fmt"
)

// Credentials holds server and client cryptographic secrets for VLESS-REALITY.
type Credentials struct {
	UUID              string `json:"uuid"`
	PrivateKey        string `json:"private_key"`
	PublicKey         string `json:"public_key"`
	ShortID           string `json:"short_id"`
	ServerName        string `json:"server_name"`
	Dest              string `json:"dest"`
}

// Generate creates a complete new set of cryptographic credentials using crypto/rand.
func Generate(destSNI string) (*Credentials, error) {
	u, err := NewUUID()
	if err != nil {
		return nil, fmt.Errorf("generate uuid: %w", err)
	}

	privKey, pubKey, err := GenerateX25519KeyPair()
	if err != nil {
		return nil, fmt.Errorf("generate x25519 key pair: %w", err)
	}

	shortID, err := NewShortID(8) // 8 hex characters (4 bytes)
	if err != nil {
		return nil, fmt.Errorf("generate short id: %w", err)
	}

	if destSNI == "" {
		destSNI = "gateway.icloud.com"
	}

	dest := destSNI
	if dest != "" && !containsPort(dest) {
		dest = dest + ":443"
	}

	return &Credentials{
		UUID:       u,
		PrivateKey: privKey,
		PublicKey:  pubKey,
		ShortID:    shortID,
		ServerName: destSNI,
		Dest:       dest,
	}, nil
}

func containsPort(s string) bool {
	for i := len(s) - 1; i >= 0; i-- {
		if s[i] == ':' {
			return true
		}
	}
	return false
}

// GenerateX25519KeyPair generates a Curve25519 private key and public key.
// Returns base64 RawURLEncoding strings compatible with Xray REALITY.
func GenerateX25519KeyPair() (string, string, error) {
	priv, err := ecdh.X25519().GenerateKey(rand.Reader)
	if err != nil {
		return "", "", fmt.Errorf("ecdh generate key: %w", err)
	}

	privBytes := priv.Bytes()
	pubBytes := priv.PublicKey().Bytes()

	// REALITY standard uses Base64 RawURLEncoding (URL-safe, without padding "=")
	privEncoded := base64.RawURLEncoding.EncodeToString(privBytes)
	pubEncoded := base64.RawURLEncoding.EncodeToString(pubBytes)

	return privEncoded, pubEncoded, nil
}

// NewUUID generates a cryptographically random RFC 4122 Version 4 UUID.
func NewUUID() (string, error) {
	var b [16]byte
	if _, err := rand.Read(b[:]); err != nil {
		return "", fmt.Errorf("crypto rand read: %w", err)
	}

	// Set version to 4
	b[6] = (b[6] & 0x0f) | 0x40
	// Set variant to RFC 4122
	b[8] = (b[8] & 0x3f) | 0x80

	return fmt.Sprintf("%08x-%04x-%04x-%04x-%012x",
		b[0:4], b[4:6], b[6:8], b[8:10], b[10:16]), nil
}

// NewShortID generates a random hex string of given character length (e.g. 8 or 16).
func NewShortID(hexLen int) (string, error) {
	byteLen := (hexLen + 1) / 2
	b := make([]byte, byteLen)
	if _, err := rand.Read(b); err != nil {
		return "", fmt.Errorf("crypto rand read: %w", err)
	}
	res := hex.EncodeToString(b)
	if len(res) > hexLen {
		res = res[:hexLen]
	}
	return res, nil
}
