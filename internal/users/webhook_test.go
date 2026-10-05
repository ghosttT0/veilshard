package users

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"
)

func TestRevocationWebhookHandler(t *testing.T) {
	tempDir := t.TempDir()
	storeFile := filepath.Join(tempDir, "webhook_users.json")

	store, err := NewStore(storeFile)
	if err != nil {
		t.Fatalf("new store: %v", err)
	}

	_, err = store.Add("victim_user", "")
	if err != nil {
		t.Fatalf("add user: %v", err)
	}

	secret := "test-secret-token"

	// Mock server request
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		auth := r.Header.Get("Authorization")
		if auth != "Bearer "+secret {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		var req RevokeRequest
		_ = json.NewDecoder(r.Body).Decode(&req)
		_, err := store.Revoke(req.Username)
		if err != nil {
			w.WriteHeader(http.StatusNotFound)
			return
		}
		w.WriteHeader(http.StatusOK)
		_ = json.NewEncoder(w).Encode(RevokeResponse{Success: true, User: req.Username})
	})

	// 1. Unauthorized request
	reqUnauth := httptest.NewRequest("POST", "/api/user/revoke", bytes.NewReader([]byte(`{"username":"victim_user"}`)))
	wUnauth := httptest.NewRecorder()
	handler.ServeHTTP(wUnauth, reqUnauth)
	if wUnauth.Code != http.StatusUnauthorized {
		t.Errorf("expected 401 Unauthorized, got %d", wUnauth.Code)
	}

	// 2. Authorized revocation
	reqAuth := httptest.NewRequest("POST", "/api/user/revoke", bytes.NewReader([]byte(`{"username":"victim_user"}`)))
	reqAuth.Header.Set("Authorization", "Bearer "+secret)
	wAuth := httptest.NewRecorder()
	handler.ServeHTTP(wAuth, reqAuth)
	if wAuth.Code != http.StatusOK {
		t.Errorf("expected 200 OK, got %d", wAuth.Code)
	}

	u, _ := store.Get("victim_user")
	if !u.IsRevoked || u.IsActive() {
		t.Errorf("victim_user should be revoked and inactive")
	}
}
