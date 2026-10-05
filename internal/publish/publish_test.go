package publish

import (
	"crypto/aes"
	"crypto/cipher"
	"encoding/base64"
	"encoding/hex"
	"strings"
	"testing"
)

func TestEncryptSubscriptionAndDecryption(t *testing.T) {
	originalConfig := `# Test Clash Configuration
proxies:
  - name: "Node-1"
    type: vless
    server: 1.2.3.4
    port: 443
`
	workerURL := "https://sub.myworker.workers.dev"
	pkg, err := EncryptSubscription([]byte(originalConfig), workerURL)
	if err != nil {
		t.Fatalf("EncryptSubscription failed: %v", err)
	}

	if pkg.SubscriptionID == "" {
		t.Errorf("empty subscription ID")
	}
	if len(pkg.SecretKeyHex) != 64 { // 32 bytes = 64 hex characters
		t.Errorf("expected 64 hex chars for 256-bit key, got %d", len(pkg.SecretKeyHex))
	}
	if !strings.HasPrefix(pkg.ZeroKnowledgeURL, workerURL+"/sub?id="+pkg.SubscriptionID+"#") {
		t.Errorf("invalid zero knowledge URL: %s", pkg.ZeroKnowledgeURL)
	}

	// Verify decryption using the secret key
	keyBytes, err := hex.DecodeString(pkg.SecretKeyHex)
	if err != nil {
		t.Fatalf("decode hex key: %v", err)
	}

	rawCipher, err := base64.StdEncoding.DecodeString(pkg.CiphertextB64)
	if err != nil {
		t.Fatalf("decode base64 ciphertext: %v", err)
	}

	block, err := aes.NewCipher(keyBytes)
	if err != nil {
		t.Fatalf("new cipher: %v", err)
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		t.Fatalf("new gcm: %v", err)
	}

	nonceSize := gcm.NonceSize()
	if len(rawCipher) < nonceSize {
		t.Fatalf("ciphertext too short")
	}

	nonce, ciphertext := rawCipher[:nonceSize], rawCipher[nonceSize:]
	decrypted, err := gcm.Open(nil, nonce, ciphertext, nil)
	if err != nil {
		t.Fatalf("gcm open failed: %v", err)
	}

	if string(decrypted) != originalConfig {
		t.Fatalf("decrypted text mismatch:\nGot: %s\nWant: %s", string(decrypted), originalConfig)
	}
}

func TestGenerateCloudflareWorkerScript(t *testing.T) {
	script := GenerateCloudflareWorkerScript()
	if !strings.Contains(script, "crypto.subtle.decrypt") {
		t.Errorf("worker script missing WebCrypto client decryption logic")
	}
	if !strings.Contains(script, "/api/publish") {
		t.Errorf("worker script missing /api/publish route")
	}
}
