//go:build integration

package httpapi

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/json"
	"net/http"
	"net/http/cookiejar"
	"net/http/httptest"
	"net/url"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"learning-serbian-platform/backend/internal/filestore"
	"learning-serbian-platform/backend/internal/identity"
	"learning-serbian-platform/backend/internal/learning"
	"learning-serbian-platform/backend/internal/store"
	"learning-serbian-platform/backend/internal/testdb"
)

// These tests run the real router, identity service, generated queries and
// PostgreSQL together. They cover the phase 1 exit criteria end to end.

type browser struct {
	t      *testing.T
	client *http.Client
	base   string
}

func newStack(t *testing.T) (*httptest.Server, *pgxpool.Pool) {
	t.Helper()
	srv, pool, _ := newStackWithFiles(t)
	return srv, pool
}

// newStackWithFiles also returns the directory uploaded files are kept in.
func newStackWithFiles(t *testing.T) (*httptest.Server, *pgxpool.Pool, string) {
	t.Helper()
	pool := testdb.New(t)
	queries := store.New(pool)
	filesDir := t.TempDir()
	files, err := filestore.NewLocal(filesDir)
	if err != nil {
		t.Fatal(err)
	}
	h := NewRouter(Dependencies{
		DB:           pool,
		Auth:         identity.NewService(queries, time.Hour),
		Vocabulary:   learning.NewVocabularyService(queries),
		Lessons:      learning.NewLessonService(queries, files),
		Annotations:  learning.NewAnnotationService(queries),
		AppOrigin:    testOrigin,
		CookieSecure: false, // httptest serves plain HTTP; a Secure cookie would never be sent back
	})
	srv := httptest.NewServer(h)
	t.Cleanup(srv.Close)
	return srv, pool, filesDir
}

func newBrowser(t *testing.T, srv *httptest.Server) *browser {
	jar, err := cookiejar.New(nil)
	if err != nil {
		t.Fatal(err)
	}
	return &browser{t: t, client: &http.Client{Jar: jar}, base: srv.URL}
}

func (b *browser) send(method, path string, body any) (*http.Response, map[string]any) {
	b.t.Helper()
	var reader *bytes.Reader
	if body != nil {
		raw, _ := json.Marshal(body)
		reader = bytes.NewReader(raw)
	} else {
		reader = bytes.NewReader(nil)
	}
	req, err := http.NewRequest(method, b.base+path, reader)
	if err != nil {
		b.t.Fatal(err)
	}
	req.Header.Set("Origin", testOrigin)
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	resp, err := b.client.Do(req)
	if err != nil {
		b.t.Fatal(err)
	}
	defer resp.Body.Close()
	var decoded map[string]any
	_ = json.NewDecoder(resp.Body).Decode(&decoded)
	return resp, decoded
}

func signUpPayload(email, role string) map[string]string {
	return map[string]string{"email": email, "password": "dobar-dan-123", "displayName": "Test " + role, "role": role}
}

func userField(t *testing.T, body map[string]any, field string) string {
	t.Helper()
	user, ok := body["user"].(map[string]any)
	if !ok {
		t.Fatalf("response has no user: %v", body)
	}
	value, _ := user[field].(string)
	return value
}

func TestStudentAndTeacherCanRegisterAndLogIn(t *testing.T) {
	srv, _ := newStack(t)

	for _, role := range []string{"student", "teacher"} {
		t.Run(role, func(t *testing.T) {
			email := role + "@example.com"

			signup := newBrowser(t, srv)
			resp, body := signup.send(http.MethodPost, "/api/auth/signup", signUpPayload(email, role))
			if resp.StatusCode != http.StatusCreated {
				t.Fatalf("signup status = %d, body = %v", resp.StatusCode, body)
			}
			if userField(t, body, "role") != role {
				t.Errorf("role = %q", userField(t, body, "role"))
			}
			// The sign-up response already logged this browser in.
			if resp, body := signup.send(http.MethodGet, "/api/me", nil); resp.StatusCode != http.StatusOK || userField(t, body, "email") != email {
				t.Fatalf("me after signup = %d %v", resp.StatusCode, body)
			}

			// A fresh browser logs in with the same credentials.
			login := newBrowser(t, srv)
			resp, body = login.send(http.MethodPost, "/api/auth/login", map[string]string{"email": email, "password": "dobar-dan-123"})
			if resp.StatusCode != http.StatusOK || userField(t, body, "role") != role {
				t.Fatalf("login = %d %v", resp.StatusCode, body)
			}
			if resp, _ := login.send(http.MethodGet, "/api/me", nil); resp.StatusCode != http.StatusOK {
				t.Fatalf("me after login = %d", resp.StatusCode)
			}

			// Logging out ends only that browser's session.
			if resp, _ := login.send(http.MethodPost, "/api/auth/logout", nil); resp.StatusCode != http.StatusNoContent {
				t.Fatalf("logout = %d", resp.StatusCode)
			}
			if resp, _ := login.send(http.MethodGet, "/api/me", nil); resp.StatusCode != http.StatusUnauthorized {
				t.Errorf("me after logout = %d, want 401", resp.StatusCode)
			}
			if resp, _ := signup.send(http.MethodGet, "/api/me", nil); resp.StatusCode != http.StatusOK {
				t.Errorf("other session affected by logout: %d", resp.StatusCode)
			}
		})
	}
}

func TestProtectedEndpointWithoutSessionIs401(t *testing.T) {
	srv, _ := newStack(t)
	resp, body := newBrowser(t, srv).send(http.MethodGet, "/api/me", nil)
	if resp.StatusCode != http.StatusUnauthorized {
		t.Fatalf("status = %d", resp.StatusCode)
	}
	if errBody, _ := body["error"].(map[string]any); errBody["code"] != "unauthenticated" {
		t.Errorf("body = %v", body)
	}
}

func TestDuplicateEmailIsRejectedCaseInsensitively(t *testing.T) {
	srv, _ := newStack(t)
	b := newBrowser(t, srv)
	if resp, _ := b.send(http.MethodPost, "/api/auth/signup", signUpPayload("ana@example.com", "student")); resp.StatusCode != http.StatusCreated {
		t.Fatalf("first signup = %d", resp.StatusCode)
	}
	resp, body := newBrowser(t, srv).send(http.MethodPost, "/api/auth/signup", signUpPayload("ANA@Example.com", "teacher"))
	if resp.StatusCode != http.StatusConflict {
		t.Fatalf("status = %d, body = %v", resp.StatusCode, body)
	}
}

func TestWrongPasswordAndUnknownEmailLookTheSame(t *testing.T) {
	srv, _ := newStack(t)
	newBrowser(t, srv).send(http.MethodPost, "/api/auth/signup", signUpPayload("ana@example.com", "student"))

	r1, b1 := newBrowser(t, srv).send(http.MethodPost, "/api/auth/login", map[string]string{"email": "ana@example.com", "password": "wrong-password"})
	r2, b2 := newBrowser(t, srv).send(http.MethodPost, "/api/auth/login", map[string]string{"email": "nobody@example.com", "password": "wrong-password"})
	if r1.StatusCode != http.StatusUnauthorized || r2.StatusCode != http.StatusUnauthorized {
		t.Fatalf("statuses = %d, %d", r1.StatusCode, r2.StatusCode)
	}
	if b1["error"].(map[string]any)["code"] != b2["error"].(map[string]any)["code"] {
		t.Errorf("responses differ: %v vs %v", b1, b2)
	}
}

func TestExpiredSessionIsRejected(t *testing.T) {
	srv, pool := newStack(t)
	b := newBrowser(t, srv)
	b.send(http.MethodPost, "/api/auth/signup", signUpPayload("ana@example.com", "student"))

	if _, err := pool.Exec(context.Background(), `UPDATE sessions SET created_at = now() - interval '2 hours', expires_at = now() - interval '1 second'`); err != nil {
		t.Fatal(err)
	}
	if resp, _ := b.send(http.MethodGet, "/api/me", nil); resp.StatusCode != http.StatusUnauthorized {
		t.Errorf("status = %d, want 401", resp.StatusCode)
	}
}

func TestDatabaseStoresOnlyHashes(t *testing.T) {
	srv, pool := newStack(t)
	b := newBrowser(t, srv)
	b.send(http.MethodPost, "/api/auth/signup", signUpPayload("ana@example.com", "student"))

	var passwordHash string
	var tokenHash []byte
	err := pool.QueryRow(context.Background(), `SELECT u.password_hash, s.token_hash FROM users u JOIN sessions s ON s.user_id = u.id`).Scan(&passwordHash, &tokenHash)
	if err != nil {
		t.Fatal(err)
	}
	if passwordHash == "dobar-dan-123" || len(passwordHash) < 50 {
		t.Errorf("password not hashed: %q", passwordHash)
	}

	var cookieValue string
	for _, c := range b.client.Jar.Cookies(mustURL(t, srv.URL)) {
		if c.Name == sessionCookieName {
			cookieValue = c.Value
		}
	}
	sum := sha256.Sum256([]byte(cookieValue))
	if cookieValue == "" || !bytes.Equal(tokenHash, sum[:]) {
		t.Error("stored token hash does not match SHA-256 of the cookie")
	}
	if bytes.Equal(tokenHash, []byte(cookieValue)) {
		t.Error("raw token stored in the database")
	}
}

func TestDeletingUserDeletesSessions(t *testing.T) {
	srv, pool := newStack(t)
	b := newBrowser(t, srv)
	b.send(http.MethodPost, "/api/auth/signup", signUpPayload("ana@example.com", "student"))

	if _, err := pool.Exec(context.Background(), `DELETE FROM users`); err != nil {
		t.Fatal(err)
	}
	var count int
	if err := pool.QueryRow(context.Background(), `SELECT count(*) FROM sessions`).Scan(&count); err != nil {
		t.Fatal(err)
	}
	if count != 0 {
		t.Errorf("sessions left after user deletion: %d", count)
	}
	if resp, _ := b.send(http.MethodGet, "/api/me", nil); resp.StatusCode != http.StatusUnauthorized {
		t.Errorf("me = %d, want 401", resp.StatusCode)
	}
}

func mustURL(t *testing.T, raw string) *url.URL {
	t.Helper()
	u, err := url.Parse(raw)
	if err != nil {
		t.Fatal(err)
	}
	return u
}
