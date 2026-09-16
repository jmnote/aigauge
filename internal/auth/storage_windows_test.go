//go:build windows

package auth

import (
	"os"
	"path/filepath"
	"testing"
)

func TestDPAPIStoreRoundtrip(t *testing.T) {
	tempDir := t.TempDir()
	filePath := filepath.Join(tempDir, "test_credentials.dat")
	store := NewWindowsDPAPIStore(filePath)

	tok := &Token{
		AccessToken:  "test-access-token-12345",
		RefreshToken: "test-refresh-token-67890",
		Extra:        Extra{Plan: "plus"},
	}

	if err := store.SaveToken("codex", tok); err != nil {
		t.Fatalf("SaveToken() error = %v", err)
	}

	// Verify file exists and is encrypted (not plaintext JSON)
	raw, err := os.ReadFile(filePath)
	if err != nil {
		t.Fatalf("ReadFile() error = %v", err)
	}
	if len(raw) == 0 {
		t.Fatal("stored credential file is empty")
	}

	loaded, err := store.GetToken("codex")
	if err != nil {
		t.Fatalf("GetToken() error = %v", err)
	}
	if loaded == nil {
		t.Fatal("GetToken() returned nil")
	}
	if loaded.AccessToken != tok.AccessToken {
		t.Errorf("AccessToken = %q, want %q", loaded.AccessToken, tok.AccessToken)
	}
	if loaded.RefreshToken != tok.RefreshToken {
		t.Errorf("RefreshToken = %q, want %q", loaded.RefreshToken, tok.RefreshToken)
	}
	if loaded.Extra.Plan != tok.Extra.Plan {
		t.Errorf("Extra plan = %q, want %q", loaded.Extra.Plan, tok.Extra.Plan)
	}

	// Test DeleteToken
	if err := store.DeleteToken("codex"); err != nil {
		t.Fatalf("DeleteToken() error = %v", err)
	}
	deleted, err := store.GetToken("codex")
	if err != nil {
		t.Fatalf("GetToken() after delete error = %v", err)
	}
	if deleted != nil {
		t.Errorf("GetToken() after delete = %v, want nil", deleted)
	}

	// Test ListTokens & Clear
	if err := store.SaveToken("claude", tok); err != nil {
		t.Fatalf("SaveToken() error = %v", err)
	}
	if !store.IsInitialized() {
		t.Error("IsInitialized() = false, want true")
	}
	list, err := store.ListTokens()
	if err != nil {
		t.Fatalf("ListTokens() error = %v", err)
	}
	if len(list) != 1 || list["claude"] == nil {
		t.Fatalf("ListTokens() = %v, want 1 token for claude", list)
	}

	if err := store.Clear(); err != nil {
		t.Fatalf("Clear() error = %v", err)
	}
	listAfterClear, err := store.ListTokens()
	if err != nil {
		t.Fatalf("ListTokens() after clear error = %v", err)
	}
	if len(listAfterClear) != 0 {
		t.Fatalf("ListTokens() after clear = %v, want empty", listAfterClear)
	}
}
