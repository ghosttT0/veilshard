package users

import (
	"path/filepath"
	"testing"
	"time"
)

func TestUserStore(t *testing.T) {
	tempDir := t.TempDir()
	storeFile := filepath.Join(tempDir, "users.json")

	store, err := NewStore(storeFile)
	if err != nil {
		t.Fatalf("failed to create store: %v", err)
	}

	u1, err := store.Add("alice", "")
	if err != nil {
		t.Fatalf("failed to add alice: %v", err)
	}
	if u1.Name != "alice" || !u1.Enabled || len(u1.UUID) != 36 {
		t.Errorf("unexpected user properties: %+v", u1)
	}

	_, err = store.Add("alice", "")
	if err != ErrUserAlreadyExists {
		t.Errorf("expected ErrUserAlreadyExists, got %v", err)
	}

	u2, err := store.Add("bob", "")
	if err != nil {
		t.Fatalf("failed to add bob: %v", err)
	}

	if len(store.List()) != 2 {
		t.Errorf("expected 2 users, got %d", len(store.List()))
	}

	if _, err := store.SetEnabled("bob", false); err != nil {
		t.Fatalf("failed to disable bob: %v", err)
	}

	if len(store.ListEnabled()) != 1 {
		t.Errorf("expected 1 enabled user, got %d", len(store.ListEnabled()))
	}

	if err := store.Remove("alice"); err != nil {
		t.Fatalf("failed to remove alice: %v", err)
	}

	if len(store.List()) != 1 {
		t.Errorf("expected 1 user left, got %d", len(store.List()))
	}
	_ = u2
}

func TestEphemeralUsersAndRevocation(t *testing.T) {
	tempDir := t.TempDir()
	storeFile := filepath.Join(tempDir, "ephemeral_users.json")

	store, err := NewStore(storeFile)
	if err != nil {
		t.Fatalf("failed to create store: %v", err)
	}

	// 1. Add ephemeral user with 100ms expiry
	u, err := store.AddWithExpiry("guest_test", "", 100*time.Millisecond)
	if err != nil {
		t.Fatalf("add ephemeral user failed: %v", err)
	}
	if !u.IsActive() {
		t.Errorf("new ephemeral user should be active initially")
	}

	time.Sleep(150 * time.Millisecond)
	if u.IsActive() {
		t.Errorf("ephemeral user should have expired after 100ms")
	}
	if len(store.ListEnabled()) != 0 {
		t.Errorf("expired user should not appear in ListEnabled")
	}

	// 2. Add normal user and revoke
	u2, err := store.Add("permanent_user", "")
	if err != nil {
		t.Fatalf("add permanent user failed: %v", err)
	}
	if !u2.IsActive() {
		t.Errorf("permanent user should be active")
	}

	revoked, err := store.Revoke("permanent_user")
	if err != nil {
		t.Fatalf("revoke failed: %v", err)
	}
	if !revoked.IsRevoked || revoked.IsActive() {
		t.Errorf("user should be revoked and inactive")
	}
	if len(store.ListEnabled()) != 0 {
		t.Errorf("revoked user should not appear in ListEnabled")
	}
}

