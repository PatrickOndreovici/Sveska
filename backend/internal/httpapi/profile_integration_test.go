//go:build integration

package httpapi

import (
	"bytes"
	"image"
	"image/color"
	"image/jpeg"
	"net/http"
	"strings"
	"testing"
)

// These tests cover changing the display name and the profile picture
// against PostgreSQL: the picture is stored, served back as a JPEG and
// reported on the user by every endpoint that returns one.

// photo is a w×h JPEG of one colour.
func photo(t *testing.T, w, h int) string {
	t.Helper()
	img := image.NewRGBA(image.Rect(0, 0, w, h))
	for y := range h {
		for x := range w {
			img.SetRGBA(x, y, color.RGBA{R: 200, G: 40, B: 40, A: 255})
		}
	}
	var buf bytes.Buffer
	if err := jpeg.Encode(&buf, img, nil); err != nil {
		t.Fatal(err)
	}
	return buf.String()
}

func avatarURL(t *testing.T, body map[string]any) any {
	t.Helper()
	user, ok := body["user"].(map[string]any)
	if !ok {
		t.Fatalf("response has no user: %v", body)
	}
	return user["avatarUrl"]
}

func TestChangingTheDisplayName(t *testing.T) {
	srv, _ := newStack(t)
	ana := signedUp(t, newBrowser(t, srv), "ana@example.com")

	resp, body := ana.send(http.MethodPut, "/api/me", map[string]string{"displayName": "  Ana Petrović "})
	if resp.StatusCode != http.StatusOK || userField(t, body, "displayName") != "Ana Petrović" {
		t.Fatalf("update = %d %v", resp.StatusCode, body)
	}
	if _, body := ana.send(http.MethodGet, "/api/me", nil); userField(t, body, "displayName") != "Ana Petrović" {
		t.Errorf("me = %v", body)
	}

	resp, body = ana.send(http.MethodPut, "/api/me", map[string]string{"displayName": strings.Repeat("a", 81)})
	if resp.StatusCode != http.StatusUnprocessableEntity {
		t.Errorf("long name = %d %v", resp.StatusCode, body)
	}
}

func TestProfilePictureFromUploadToDelete(t *testing.T) {
	srv, _ := newStack(t)
	ana := signedUp(t, newBrowser(t, srv), "ana@example.com")

	if _, body := ana.send(http.MethodGet, "/api/me", nil); avatarURL(t, body) != nil {
		t.Fatalf("a new user has a picture: %v", body)
	}
	if resp, _ := ana.download("/api/me/avatar"); resp.StatusCode != http.StatusNotFound {
		t.Errorf("picture before upload = %d, want 404", resp.StatusCode)
	}

	resp, body := ana.sendFile(http.MethodPut, "/api/me/avatar", "me.jpg", photo(t, 800, 600))
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("upload = %d %v", resp.StatusCode, body)
	}
	url, _ := avatarURL(t, body).(string)
	if !strings.HasPrefix(url, "/api/me/avatar?v=") {
		t.Fatalf("avatarUrl = %q", url)
	}

	// The same URL comes back from /api/me and from logging in again.
	if _, body := ana.send(http.MethodGet, "/api/me", nil); avatarURL(t, body) != url {
		t.Errorf("me avatarUrl = %v, want %s", avatarURL(t, body), url)
	}
	login := newBrowser(t, srv)
	_, body = login.send(http.MethodPost, "/api/auth/login", map[string]string{"email": "ana@example.com", "password": "dobar-dan-123"})
	if avatarURL(t, body) != url {
		t.Errorf("login avatarUrl = %v, want %s", avatarURL(t, body), url)
	}

	resp, picture := ana.download(url)
	if resp.StatusCode != http.StatusOK || resp.Header.Get("Content-Type") != "image/jpeg" {
		t.Fatalf("picture = %d %s", resp.StatusCode, resp.Header.Get("Content-Type"))
	}
	img, err := jpeg.Decode(strings.NewReader(picture))
	if err != nil {
		t.Fatal(err)
	}
	if b := img.Bounds(); b.Dx() != 256 || b.Dy() != 256 {
		t.Errorf("picture is %v, want 256×256", b.Size())
	}

	// A second upload replaces the first and gets a new URL.
	_, body = ana.sendFile(http.MethodPut, "/api/me/avatar", "me.jpg", photo(t, 100, 100))
	if next := avatarURL(t, body); next == url || next == nil {
		t.Errorf("avatarUrl after replacing = %v", next)
	}

	if resp, _ := ana.send(http.MethodDelete, "/api/me/avatar", nil); resp.StatusCode != http.StatusNoContent {
		t.Fatalf("delete = %d", resp.StatusCode)
	}
	if _, body := ana.send(http.MethodGet, "/api/me", nil); avatarURL(t, body) != nil {
		t.Errorf("me after delete = %v", body)
	}
	if resp, _ := ana.download("/api/me/avatar"); resp.StatusCode != http.StatusNotFound {
		t.Errorf("picture after delete = %d, want 404", resp.StatusCode)
	}
}

func TestProfilePictureIsYourOwn(t *testing.T) {
	srv, _ := newStack(t)
	ana := signedUp(t, newBrowser(t, srv), "ana@example.com")
	marko := signedUp(t, newBrowser(t, srv), "marko@example.com")

	if resp, body := ana.sendFile(http.MethodPut, "/api/me/avatar", "me.jpg", photo(t, 50, 50)); resp.StatusCode != http.StatusOK {
		t.Fatalf("upload = %d %v", resp.StatusCode, body)
	}
	if resp, _ := marko.download("/api/me/avatar"); resp.StatusCode != http.StatusNotFound {
		t.Errorf("Marko sees a picture: %d", resp.StatusCode)
	}
	if resp, _ := marko.send(http.MethodDelete, "/api/me/avatar", nil); resp.StatusCode != http.StatusNoContent {
		t.Fatalf("Marko's delete = %d", resp.StatusCode)
	}
	if resp, _ := ana.download("/api/me/avatar"); resp.StatusCode != http.StatusOK {
		t.Errorf("Ana's picture after Marko deleted his = %d", resp.StatusCode)
	}
}

func TestProfilePictureMustBeAPicture(t *testing.T) {
	srv, _ := newStack(t)
	ana := signedUp(t, newBrowser(t, srv), "ana@example.com")

	resp, body := ana.sendFile(http.MethodPut, "/api/me/avatar", "lesson.pdf", samplePDF)
	if resp.StatusCode != http.StatusUnprocessableEntity {
		t.Fatalf("PDF as a picture = %d %v", resp.StatusCode, body)
	}
	if _, body := ana.send(http.MethodGet, "/api/me", nil); avatarURL(t, body) != nil {
		t.Errorf("a picture was saved: %v", body)
	}
}
