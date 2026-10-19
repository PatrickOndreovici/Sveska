package identity

import (
	"strings"
	"testing"
)

func TestHashAndVerifyPassword(t *testing.T) {
	hash, err := hashPassword("correct horse battery")
	if err != nil {
		t.Fatalf("hashPassword: %v", err)
	}
	if !strings.HasPrefix(hash, "$argon2id$v=19$m=19456,t=2,p=1$") {
		t.Errorf("unexpected hash format: %s", hash)
	}

	ok, err := verifyPassword("correct horse battery", hash)
	if err != nil || !ok {
		t.Errorf("verify correct password = %v, %v; want true, nil", ok, err)
	}
	ok, err = verifyPassword("wrong horse battery", hash)
	if err != nil || ok {
		t.Errorf("verify wrong password = %v, %v; want false, nil", ok, err)
	}
}

func TestHashPasswordUsesUniqueSalt(t *testing.T) {
	a, _ := hashPassword("same password here")
	b, _ := hashPassword("same password here")
	if a == b {
		t.Error("two hashes of the same password must differ")
	}
}

func TestVerifyPasswordRejectsMalformedHash(t *testing.T) {
	for _, bad := range []string{"", "plain", "$bcrypt$x$y$z$w", "$argon2id$v=19$m=x$salt$hash", "$argon2id$v=19$m=1,t=1,p=1$!!$!!"} {
		if _, err := verifyPassword("whatever123", bad); err == nil {
			t.Errorf("expected error for %q", bad)
		}
	}
}
