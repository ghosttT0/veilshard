package credentials

import (
	"encoding/base64"
	"strings"
	"testing"
)

func TestGenerateCredentials(t *testing.T) {
	creds, err := Generate("gateway.icloud.com")
	if err != nil {
		t.Fatalf("Generate failed: %v", err)
	}

	if len(creds.UUID) != 36 {
		t.Errorf("expected UUID len 36, got %d (%s)", len(creds.UUID), creds.UUID)
	}

	if len(creds.ShortID) != 8 {
		t.Errorf("expected ShortID len 8, got %d (%s)", len(creds.ShortID), creds.ShortID)
	}

	if creds.PrivateKey == "" || creds.PublicKey == "" {
		t.Fatalf("empty private or public key")
	}

	privBytes, err := base64.RawURLEncoding.DecodeString(creds.PrivateKey)
	if err != nil {
		t.Fatalf("failed to decode private key: %v", err)
	}
	if len(privBytes) != 32 {
		t.Errorf("expected 32 bytes private key, got %d", len(privBytes))
	}

	pubBytes, err := base64.RawURLEncoding.DecodeString(creds.PublicKey)
	if err != nil {
		t.Fatalf("failed to decode public key: %v", err)
	}
	if len(pubBytes) != 32 {
		t.Errorf("expected 32 bytes public key, got %d", len(pubBytes))
	}

	if !strings.HasSuffix(creds.Dest, ":443") {
		t.Errorf("expected Dest to have :443, got %s", creds.Dest)
	}
}
