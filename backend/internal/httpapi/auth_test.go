package httpapi

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"learning-serbian-platform/backend/internal/apperr"
	"learning-serbian-platform/backend/internal/identity"
)

const (
	testOrigin = "http://localhost:5180"
	validToken = "valid-token-valid-token-valid-token-valid-t"
)

var testUser = identity.User{
	ID: "019a0000-0000-7000-8000-000000000001", Email: "ana@example.com", DisplayName: "Ana",
	Role: identity.RoleStudent, CreatedAt: time.Date(2026, 10, 19, 12, 0, 0, 0, time.UTC),
}

// fakeAuth accepts validToken as the only live session.
type fakeAuth struct {
	signUpErr  error
	loggedOut  []string
	authErr    error
	lastSignUp identity.SignUpInput
}

func (f *fakeAuth) SignUp(_ context.Context, in identity.SignUpInput) (identity.User, identity.Session, error) {
	f.lastSignUp = in
	if f.signUpErr != nil {
		return identity.User{}, identity.Session{}, f.signUpErr
	}
	return testUser, identity.Session{Token: validToken, ExpiresAt: time.Now().Add(time.Hour)}, nil
}

func (f *fakeAuth) LogIn(_ context.Context, in identity.LogInInput) (identity.User, identity.Session, error) {
	if in.Password != "dobar-dan-123" {
		return identity.User{}, identity.Session{}, apperr.New(apperr.Unauthenticated, "invalid_credentials", "Email or password is incorrect.")
	}
	return testUser, identity.Session{Token: validToken, ExpiresAt: time.Now().Add(time.Hour)}, nil
}

func (f *fakeAuth) LogOut(_ context.Context, token string) error {
	f.loggedOut = append(f.loggedOut, token)
	return nil
}

func (f *fakeAuth) Authenticate(_ context.Context, token string) (identity.User, error) {
	if f.authErr != nil {
		return identity.User{}, f.authErr
	}
	if token != validToken {
		return identity.User{}, apperr.New(apperr.Unauthenticated, "unauthenticated", "Log in to continue.")
	}
	return testUser, nil
}

func newTestRouter(auth *fakeAuth) http.Handler {
	return NewRouter(Dependencies{DB: fakePinger{}, Auth: auth, AppOrigin: testOrigin, CookieSecure: true})
}

type requestOpts struct {
	body   string
	cookie string
	origin string
	noJSON bool
}

func do(t *testing.T, h http.Handler, method, path string, o requestOpts) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest(method, path, strings.NewReader(o.body))
	if o.body != "" && !o.noJSON {
		req.Header.Set("Content-Type", "application/json")
	}
	if o.cookie != "" {
		req.AddCookie(&http.Cookie{Name: sessionCookieName, Value: o.cookie})
	}
	if o.origin != "" {
		req.Header.Set("Origin", o.origin)
	}
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	return rec
}

func sessionCookie(t *testing.T, rec *httptest.ResponseRecorder) *http.Cookie {
	t.Helper()
	for _, c := range rec.Result().Cookies() {
		if c.Name == sessionCookieName {
			return c
		}
	}
	return nil
}

func decodeError(t *testing.T, rec *httptest.ResponseRecorder) errorDetail {
	t.Helper()
	var body errorBody
	if err := json.NewDecoder(rec.Body).Decode(&body); err != nil {
		t.Fatalf("decode error body: %v", err)
	}
	return body.Error
}

const signUpBody = `{"email":"ana@example.com","password":"dobar-dan-123","displayName":"Ana","role":"student"}`

func TestSignUpSetsHardenedSessionCookie(t *testing.T) {
	auth := &fakeAuth{}
	rec := do(t, newTestRouter(auth), http.MethodPost, "/api/auth/signup", requestOpts{body: signUpBody, origin: testOrigin})

	if rec.Code != http.StatusCreated {
		t.Fatalf("status = %d, body = %s", rec.Code, rec.Body)
	}
	c := sessionCookie(t, rec)
	if c == nil {
		t.Fatal("no session cookie set")
	}
	if c.Value != validToken || !c.HttpOnly || !c.Secure || c.SameSite != http.SameSiteLaxMode || c.Path != "/" || c.MaxAge <= 0 {
		t.Errorf("cookie = %+v", c)
	}
	if auth.lastSignUp.Role != identity.RoleStudent || auth.lastSignUp.DisplayName != "Ana" {
		t.Errorf("service received %+v", auth.lastSignUp)
	}
	var body userEnvelope
	if err := json.NewDecoder(rec.Body).Decode(&body); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if body.User.ID != testUser.ID || body.User.Role != "student" || body.User.DisplayName != "Ana" {
		t.Errorf("body = %+v", body)
	}
}

func TestSignUpReplacesExistingSession(t *testing.T) {
	auth := &fakeAuth{}
	old := "old-token-old-token-old-token-old-token-old"
	do(t, newTestRouter(auth), http.MethodPost, "/api/auth/signup", requestOpts{body: signUpBody, cookie: old})
	if len(auth.loggedOut) != 1 || auth.loggedOut[0] != old {
		t.Errorf("logged out = %v, want the old token", auth.loggedOut)
	}
}

func TestServiceErrorsMapToStatusCodes(t *testing.T) {
	tests := []struct {
		name   string
		err    error
		status int
		code   string
	}{
		{"validation", apperr.Validation(map[string]string{"email": "Enter a valid email address."}), http.StatusUnprocessableEntity, "validation_failed"},
		{"conflict", apperr.New(apperr.Conflict, "email_taken", "taken"), http.StatusConflict, "email_taken"},
		{"not found", apperr.New(apperr.NotFound, "not_found", "missing"), http.StatusNotFound, "not_found"},
		{"unexpected", errors.New("db exploded"), http.StatusInternalServerError, "internal_error"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rec := do(t, newTestRouter(&fakeAuth{signUpErr: tt.err}), http.MethodPost, "/api/auth/signup", requestOpts{body: signUpBody})
			if rec.Code != tt.status {
				t.Fatalf("status = %d, want %d", rec.Code, tt.status)
			}
			detail := decodeError(t, rec)
			if detail.Code != tt.code {
				t.Errorf("code = %q, want %q", detail.Code, tt.code)
			}
			if strings.Contains(detail.Message, "exploded") {
				t.Error("internal error details leaked to the client")
			}
			if sessionCookie(t, rec) != nil {
				t.Error("failed sign-up must not set a session cookie")
			}
		})
	}
}

func TestValidationErrorIncludesFields(t *testing.T) {
	err := apperr.Validation(map[string]string{"email": "Enter a valid email address."})
	rec := do(t, newTestRouter(&fakeAuth{signUpErr: err}), http.MethodPost, "/api/auth/signup", requestOpts{body: signUpBody})
	if detail := decodeError(t, rec); detail.Fields["email"] != "Enter a valid email address." {
		t.Errorf("fields = %v", detail.Fields)
	}
}

func TestRequestBodyIsStrict(t *testing.T) {
	tests := []struct {
		name string
		opts requestOpts
	}{
		{"unknown field", requestOpts{body: `{"email":"a@b.rs","password":"x","isAdmin":true}`}},
		{"malformed json", requestOpts{body: `{"email":`}},
		{"two objects", requestOpts{body: `{"email":"a@b.rs","password":"x"}{}`}},
		{"wrong content type", requestOpts{body: `{"email":"a@b.rs","password":"x"}`, noJSON: true}},
		{"oversized body", requestOpts{body: `{"email":"` + strings.Repeat("a", 70<<10) + `"}`}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rec := do(t, newTestRouter(&fakeAuth{}), http.MethodPost, "/api/auth/login", tt.opts)
			if rec.Code != http.StatusUnprocessableEntity {
				t.Errorf("status = %d, want 422", rec.Code)
			}
		})
	}
}

func TestLogInWithWrongPasswordReturns401(t *testing.T) {
	rec := do(t, newTestRouter(&fakeAuth{}), http.MethodPost, "/api/auth/login", requestOpts{body: `{"email":"ana@example.com","password":"nope-nope-nope"}`})
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d", rec.Code)
	}
	if decodeError(t, rec).Code != "invalid_credentials" {
		t.Error("wrong error code")
	}
}

func TestMe(t *testing.T) {
	h := newTestRouter(&fakeAuth{})

	t.Run("without a cookie is 401", func(t *testing.T) {
		rec := do(t, h, http.MethodGet, "/api/me", requestOpts{})
		if rec.Code != http.StatusUnauthorized || decodeError(t, rec).Code != "unauthenticated" {
			t.Fatalf("status = %d", rec.Code)
		}
	})
	t.Run("with a stale cookie is 401 and clears the cookie", func(t *testing.T) {
		rec := do(t, h, http.MethodGet, "/api/me", requestOpts{cookie: "stale-stale-stale-stale-stale-stale-stale-s"})
		if rec.Code != http.StatusUnauthorized {
			t.Fatalf("status = %d", rec.Code)
		}
		if c := sessionCookie(t, rec); c == nil || c.MaxAge >= 0 {
			t.Errorf("stale cookie not cleared: %+v", c)
		}
	})
	t.Run("with a valid session returns the user", func(t *testing.T) {
		rec := do(t, h, http.MethodGet, "/api/me", requestOpts{cookie: validToken})
		if rec.Code != http.StatusOK {
			t.Fatalf("status = %d", rec.Code)
		}
		var body userEnvelope
		_ = json.NewDecoder(rec.Body).Decode(&body)
		if body.User.Email != testUser.Email {
			t.Errorf("body = %+v", body)
		}
	})
}

func TestSessionLookupFailureIs500(t *testing.T) {
	rec := do(t, newTestRouter(&fakeAuth{authErr: errors.New("db down")}), http.MethodGet, "/api/me", requestOpts{cookie: validToken})
	if rec.Code != http.StatusInternalServerError {
		t.Errorf("status = %d, want 500", rec.Code)
	}
}

func TestLogOutEndsSessionAndClearsCookie(t *testing.T) {
	auth := &fakeAuth{}
	rec := do(t, newTestRouter(auth), http.MethodPost, "/api/auth/logout", requestOpts{cookie: validToken, origin: testOrigin})
	if rec.Code != http.StatusNoContent {
		t.Fatalf("status = %d", rec.Code)
	}
	if len(auth.loggedOut) != 1 || auth.loggedOut[0] != validToken {
		t.Errorf("logged out = %v", auth.loggedOut)
	}
	if c := sessionCookie(t, rec); c == nil || c.MaxAge >= 0 {
		t.Errorf("cookie not cleared: %+v", c)
	}
}

func TestLogOutWithoutSessionStillSucceeds(t *testing.T) {
	rec := do(t, newTestRouter(&fakeAuth{}), http.MethodPost, "/api/auth/logout", requestOpts{})
	if rec.Code != http.StatusNoContent {
		t.Errorf("status = %d", rec.Code)
	}
}

func TestCrossOriginWritesAreRejected(t *testing.T) {
	h := newTestRouter(&fakeAuth{})
	for _, origin := range []string{"https://evil.example", "null", "http://localhost:5181"} {
		rec := do(t, h, http.MethodPost, "/api/auth/login", requestOpts{body: `{"email":"ana@example.com","password":"dobar-dan-123"}`, origin: origin})
		if rec.Code != http.StatusForbidden || decodeError(t, rec).Code != "forbidden_origin" {
			t.Errorf("origin %q: status = %d, want 403", origin, rec.Code)
		}
	}
	// Reads are not blocked by origin; the session cookie decides.
	if rec := do(t, h, http.MethodGet, "/api/me", requestOpts{cookie: validToken, origin: "https://evil.example"}); rec.Code != http.StatusOK {
		t.Errorf("GET with foreign origin: status = %d", rec.Code)
	}
}

func TestSecurityHeadersOnEveryResponse(t *testing.T) {
	h := newTestRouter(&fakeAuth{})
	for _, path := range []string{"/api/health", "/api/me", "/api/nope"} {
		rec := do(t, h, http.MethodGet, path, requestOpts{})
		for header, want := range map[string]string{
			"X-Content-Type-Options": "nosniff",
			"X-Frame-Options":        "DENY",
			"Cache-Control":          "no-store",
			"Referrer-Policy":        "no-referrer",
		} {
			if got := rec.Header().Get(header); got != want {
				t.Errorf("%s %s = %q, want %q", path, header, got, want)
			}
		}
	}
}
