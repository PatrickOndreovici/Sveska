package httpapi

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"learning-serbian-platform/backend/internal/apperr"
	"learning-serbian-platform/backend/internal/identity"
)

var avatarTime = time.Date(2026, 10, 23, 9, 30, 0, 123_000_000, time.UTC)

// fakeProfile records what the handlers pass in. It answers with testUser,
// renamed or given a picture, or with err when set.
type fakeProfile struct {
	err       error
	calls     int
	lastID    string
	lastName  string
	lastUser  identity.User
	lastImage string
}

func (f *fakeProfile) UpdateDisplayName(_ context.Context, userID, displayName string) (identity.User, error) {
	f.calls++
	f.lastID, f.lastName = userID, displayName
	user := testUser
	user.DisplayName = displayName
	return user, f.err
}

func (f *fakeProfile) SaveAvatar(_ context.Context, user identity.User, body io.Reader) (identity.User, error) {
	f.calls++
	f.lastUser = user
	data, err := io.ReadAll(body)
	if err != nil {
		return identity.User{}, err
	}
	f.lastImage = string(data)
	user.AvatarUpdatedAt = &avatarTime
	return user, f.err
}

func (f *fakeProfile) Avatar(_ context.Context, userID string) (identity.Avatar, error) {
	f.calls++
	f.lastID = userID
	return identity.Avatar{Image: []byte("jpeg bytes"), UpdatedAt: avatarTime}, f.err
}

func (f *fakeProfile) DeleteAvatar(_ context.Context, userID string) error {
	f.calls++
	f.lastID = userID
	return f.err
}

func newProfileRouter(p *fakeProfile) http.Handler {
	return NewRouter(Dependencies{DB: fakePinger{}, Auth: &fakeAuth{}, Profile: p, AppOrigin: testOrigin, CookieSecure: true})
}

func uploadAvatar(t *testing.T, h http.Handler, contentType string, body io.Reader) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest(http.MethodPut, "/api/me/avatar", body)
	req.Header.Set("Content-Type", contentType)
	req.Header.Set("Origin", testOrigin)
	req.AddCookie(&http.Cookie{Name: sessionCookieName, Value: validToken})
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	return rec
}

func decodeUser(t *testing.T, rec *httptest.ResponseRecorder) userResponse {
	t.Helper()
	var env userEnvelope
	if err := json.NewDecoder(rec.Body).Decode(&env); err != nil {
		t.Fatal(err)
	}
	return env.User
}

func TestProfileRoutesRequireSession(t *testing.T) {
	routes := []struct{ method, path, body string }{
		{http.MethodPut, "/api/me", `{"displayName":"Ana"}`},
		{http.MethodGet, "/api/me/avatar", ""},
		{http.MethodPut, "/api/me/avatar", ""},
		{http.MethodDelete, "/api/me/avatar", ""},
	}
	for _, rt := range routes {
		t.Run(rt.method+" "+rt.path, func(t *testing.T) {
			p := &fakeProfile{}
			rec := do(t, newProfileRouter(p), rt.method, rt.path, requestOpts{body: rt.body, origin: testOrigin})
			if rec.Code != http.StatusUnauthorized || p.calls != 0 {
				t.Errorf("status = %d, service calls = %d; want 401 and 0", rec.Code, p.calls)
			}
		})
	}
}

func TestUpdateMeChangesTheLoggedInUsersName(t *testing.T) {
	p := &fakeProfile{}
	rec := do(t, newProfileRouter(p), http.MethodPut, "/api/me", loggedIn(`{"displayName":"Ana Petrović"}`))

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", rec.Code, rec.Body)
	}
	if p.lastID != testUser.ID || p.lastName != "Ana Petrović" {
		t.Errorf("service got %q, %q", p.lastID, p.lastName)
	}
	if user := decodeUser(t, rec); user.DisplayName != "Ana Petrović" || user.AvatarURL != nil {
		t.Errorf("user = %+v", user)
	}
}

func TestUpdateMeRejectsOtherFields(t *testing.T) {
	p := &fakeProfile{}
	rec := do(t, newProfileRouter(p), http.MethodPut, "/api/me", loggedIn(`{"displayName":"Ana","role":"teacher"}`))
	if rec.Code != http.StatusUnprocessableEntity || p.calls != 0 {
		t.Errorf("status = %d, service calls = %d; want 422 and 0", rec.Code, p.calls)
	}
}

func TestUpdateMeReportsTheNamesProblem(t *testing.T) {
	p := &fakeProfile{err: apperr.Validation(map[string]string{"displayName": "Enter your name."})}
	rec := do(t, newProfileRouter(p), http.MethodPut, "/api/me", loggedIn(`{"displayName":" "}`))
	if rec.Code != http.StatusUnprocessableEntity || decodeError(t, rec).Fields["displayName"] != "Enter your name." {
		t.Errorf("status = %d, body = %s", rec.Code, rec.Body)
	}
}

func TestUploadAvatarReturnsTheUserWithAVersionedURL(t *testing.T) {
	p := &fakeProfile{}
	contentType, body := multipartBody(t, map[string]string{"note": "ignored", "file": "picture bytes"})

	rec := uploadAvatar(t, newProfileRouter(p), contentType, body)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", rec.Code, rec.Body)
	}
	if p.lastUser.ID != testUser.ID || p.lastImage != "picture bytes" {
		t.Errorf("service got user %q, image %q", p.lastUser.ID, p.lastImage)
	}
	user := decodeUser(t, rec)
	if user.AvatarURL == nil || *user.AvatarURL != "/api/me/avatar?v=1792747800123" {
		t.Errorf("avatarUrl = %v", user.AvatarURL)
	}
}

func TestUploadAvatarRejectsBadRequests(t *testing.T) {
	noFileType, noFileBody := multipartBody(t, map[string]string{"title": "x"})
	tests := []struct {
		name        string
		contentType string
		body        io.Reader
		code        string
		problem     string
	}{
		{"JSON instead of a form", "application/json", strings.NewReader(`{}`), "unsupported_content_type", ""},
		{"form without a file", noFileType, noFileBody, "validation_failed", "Choose a picture to upload."},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			p := &fakeProfile{}
			rec := uploadAvatar(t, newProfileRouter(p), tt.contentType, tt.body)
			if rec.Code != http.StatusUnprocessableEntity || p.calls != 0 {
				t.Fatalf("status = %d, service calls = %d; want 422 and 0", rec.Code, p.calls)
			}
			e := decodeError(t, rec)
			if e.Code != tt.code || e.Fields["file"] != tt.problem {
				t.Errorf("error = %+v", e)
			}
		})
	}
}

func TestUploadAvatarOverTheRequestLimitIsAFieldError(t *testing.T) {
	contentType, body := multipartBody(t, map[string]string{"file": strings.Repeat("0", identity.MaxAvatarBytes+maxMultipartOverhead)})

	rec := uploadAvatar(t, newProfileRouter(&fakeProfile{}), contentType, body)

	if rec.Code != http.StatusUnprocessableEntity {
		t.Fatalf("status = %d, want 422", rec.Code)
	}
	if got := decodeError(t, rec).Fields["file"]; got != "Choose a picture of at most 10 MB." {
		t.Errorf("file problem = %q", got)
	}
}

func TestGetAvatarServesTheLoggedInUsersPicture(t *testing.T) {
	p := &fakeProfile{}
	rec := do(t, newProfileRouter(p), http.MethodGet, "/api/me/avatar?v=1", loggedIn(""))

	if rec.Code != http.StatusOK || rec.Body.String() != "jpeg bytes" {
		t.Fatalf("status = %d, body = %q", rec.Code, rec.Body)
	}
	if p.lastID != testUser.ID {
		t.Errorf("service got %q", p.lastID)
	}
	h := rec.Header()
	if h.Get("Content-Type") != "image/jpeg" || h.Get("X-Content-Type-Options") != "nosniff" {
		t.Errorf("Content-Type = %q, nosniff = %q", h.Get("Content-Type"), h.Get("X-Content-Type-Options"))
	}
	if got := h.Get("Cache-Control"); got != "private, max-age=31536000, immutable" {
		t.Errorf("Cache-Control = %q", got)
	}
}

func TestGetAvatarWithoutOneIs404(t *testing.T) {
	p := &fakeProfile{err: apperr.New(apperr.NotFound, "avatar_not_found", "There is no profile picture.")}
	rec := do(t, newProfileRouter(p), http.MethodGet, "/api/me/avatar", loggedIn(""))
	if rec.Code != http.StatusNotFound || rec.Header().Get("Cache-Control") != "no-store" {
		t.Errorf("status = %d, Cache-Control = %q", rec.Code, rec.Header().Get("Cache-Control"))
	}
}

func TestDeleteAvatar(t *testing.T) {
	p := &fakeProfile{}
	rec := do(t, newProfileRouter(p), http.MethodDelete, "/api/me/avatar", loggedIn(""))
	if rec.Code != http.StatusNoContent || p.lastID != testUser.ID {
		t.Errorf("status = %d, service got %q", rec.Code, p.lastID)
	}
}

func TestMeIncludesTheAvatarURL(t *testing.T) {
	user := testUser
	user.AvatarUpdatedAt = &avatarTime
	if got := toUserEnvelope(user).User.AvatarURL; got == nil || *got != "/api/me/avatar?v=1792747800123" {
		t.Errorf("avatarUrl = %v", got)
	}
	if got := toUserEnvelope(testUser).User.AvatarURL; got != nil {
		t.Errorf("without a picture: avatarUrl = %q", *got)
	}
}
