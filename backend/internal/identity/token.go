package identity

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"fmt"
)

// Session tokens are 32 random bytes, base64url-encoded (43 characters).
// Only the SHA-256 of the token is stored, so a database leak does not
// expose usable session cookies.
const (
	tokenBytes      = 32
	encodedTokenLen = 43
)

func newSessionToken() (token string, hash []byte, err error) {
	raw := make([]byte, tokenBytes)
	if _, err := rand.Read(raw); err != nil {
		return "", nil, fmt.Errorf("generate session token: %w", err)
	}
	token = base64.RawURLEncoding.EncodeToString(raw)
	return token, hashToken(token), nil
}

func hashToken(token string) []byte {
	sum := sha256.Sum256([]byte(token))
	return sum[:]
}

// looksLikeToken rejects obviously malformed cookie values before a database lookup.
func looksLikeToken(token string) bool {
	if len(token) != encodedTokenLen {
		return false
	}
	_, err := base64.RawURLEncoding.DecodeString(token)
	return err == nil
}
