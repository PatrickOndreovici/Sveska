package httpapi

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
)

type fakePinger struct{ err error }

func (f fakePinger) Ping(context.Context) error { return f.err }

func get(t *testing.T, h http.Handler, path string) *httptest.ResponseRecorder {
	t.Helper()
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, path, nil))
	return rec
}

func TestHealthReportsOKWhenDatabaseReachable(t *testing.T) {
	rec := get(t, NewRouter(Dependencies{DB: fakePinger{}, Auth: &fakeAuth{}, AppOrigin: testOrigin}), "/api/health")

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusOK)
	}
	if ct := rec.Header().Get("Content-Type"); ct != "application/json" {
		t.Errorf("Content-Type = %q", ct)
	}
	var body healthResponse
	if err := json.NewDecoder(rec.Body).Decode(&body); err != nil {
		t.Fatalf("decode body: %v", err)
	}
	if body != (healthResponse{Status: "ok", Database: "ok"}) {
		t.Errorf("body = %+v", body)
	}
}

func TestHealthReportsUnavailableWhenDatabaseDown(t *testing.T) {
	rec := get(t, NewRouter(Dependencies{DB: fakePinger{err: errors.New("connection refused")}, Auth: &fakeAuth{}, AppOrigin: testOrigin}), "/api/health")

	if rec.Code != http.StatusServiceUnavailable {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusServiceUnavailable)
	}
	var body healthResponse
	if err := json.NewDecoder(rec.Body).Decode(&body); err != nil {
		t.Fatalf("decode body: %v", err)
	}
	if body != (healthResponse{Status: "unavailable", Database: "unreachable"}) {
		t.Errorf("body = %+v", body)
	}
}

func TestUnknownRouteReturnsJSONError(t *testing.T) {
	rec := get(t, NewRouter(Dependencies{DB: fakePinger{}, Auth: &fakeAuth{}, AppOrigin: testOrigin}), "/api/does-not-exist")

	if rec.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusNotFound)
	}
	var body errorBody
	if err := json.NewDecoder(rec.Body).Decode(&body); err != nil {
		t.Fatalf("decode body: %v", err)
	}
	if body.Error.Code != "not_found" {
		t.Errorf("error code = %q, want %q", body.Error.Code, "not_found")
	}
}

func TestWrongMethodReturnsJSONError(t *testing.T) {
	rec := httptest.NewRecorder()
	NewRouter(Dependencies{DB: fakePinger{}, Auth: &fakeAuth{}, AppOrigin: testOrigin}).ServeHTTP(rec, httptest.NewRequest(http.MethodPost, "/api/health", nil))

	if rec.Code != http.StatusMethodNotAllowed {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusMethodNotAllowed)
	}
}
