package sub

import (
	"encoding/base64"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/veilshard/veilshard/internal/exporter"
)

func TestSubscriptionGenerators(t *testing.T) {
	ctx := &exporter.ExportContext{
		ServerIP:   "198.51.100.1",
		Port:       443,
		UUID:       "9f3f4c6e-1111-2222-3333-444455556666",
		UserName:   "alice",
		PublicKey:  "test-public-key-abcdef",
		ShortID:    "1234abcd",
		ServerName: "gateway.icloud.com",
		NodeName:   "vpnctl-alice",
	}

	// 1. Test Universal Base64 Subscription
	b64 := GenerateBase64Subscription(ctx)
	if b64 == "" {
		t.Fatalf("empty base64 subscription")
	}

	decodedBytes, err := base64.StdEncoding.DecodeString(b64)
	if err != nil {
		t.Fatalf("decode base64 failed: %v", err)
	}

	decodedStr := string(decodedBytes)
	if !strings.HasPrefix(decodedStr, "vless://") {
		t.Fatalf("expected vless:// in decoded content, got: %s", decodedStr)
	}
	if !strings.Contains(decodedStr, "pbk=test-public-key-abcdef") {
		t.Fatalf("missing public key in vless uri")
	}

	// 2. Test Full Clash Profile
	profile := GenerateFullClashProfile(ctx)
	if !strings.Contains(profile, "proxies:") || !strings.Contains(profile, "proxy-groups:") || !strings.Contains(profile, "rules:") {
		t.Fatalf("clash profile missing essential sections")
	}
	if !strings.Contains(profile, "gateway.icloud.com") {
		t.Fatalf("clash profile missing SNI")
	}
	if !strings.Contains(profile, "flow: xtls-rprx-vision") {
		t.Fatalf("clash profile missing flow: xtls-rprx-vision")
	}

	// 3. Test HTTP Subscription handler
	reqClash := httptest.NewRequest("GET", "/sub/token", nil)
	reqClash.Header.Set("User-Agent", "ClashMeta/v1.18.0")
	recClash := httptest.NewRecorder()

	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ua := strings.ToLower(r.UserAgent())
		if strings.Contains(ua, "clash") {
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(GenerateFullClashProfile(ctx)))
		} else {
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(GenerateBase64Subscription(ctx)))
		}
	})

	handler.ServeHTTP(recClash, reqClash)
	if !strings.Contains(recClash.Body.String(), "proxies:") {
		t.Fatalf("expected Clash YAML response for Clash User-Agent")
	}

	reqRocket := httptest.NewRequest("GET", "/sub/token", nil)
	reqRocket.Header.Set("User-Agent", "Shadowrocket/2.2.0")
	recRocket := httptest.NewRecorder()

	handler.ServeHTTP(recRocket, reqRocket)
	if strings.Contains(recRocket.Body.String(), "proxies:") {
		t.Fatalf("expected Base64 response for Shadowrocket User-Agent")
	}
}
