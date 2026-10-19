package identity

import (
	"bytes"
	"testing"
)

func TestNewSessionToken(t *testing.T) {
	token, hash, err := newSessionToken()
	if err != nil {
		t.Fatalf("newSessionToken: %v", err)
	}
	if !looksLikeToken(token) {
		t.Errorf("token %q does not pass its own format check", token)
	}
	if !bytes.Equal(hash, hashToken(token)) {
		t.Error("returned hash does not match hashToken(token)")
	}
	if len(hash) != 32 {
		t.Errorf("hash length = %d, want 32", len(hash))
	}
	other, _, _ := newSessionToken()
	if other == token {
		t.Error("tokens must be unique")
	}
}

func TestLooksLikeTokenRejectsJunk(t *testing.T) {
	for _, bad := range []string{"", "short", "!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!", "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"} {
		if looksLikeToken(bad) {
			t.Errorf("looksLikeToken(%q) = true", bad)
		}
	}
}
