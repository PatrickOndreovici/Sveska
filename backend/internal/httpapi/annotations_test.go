package httpapi

import (
	"context"
	"encoding/json"
	"net/http"
	"slices"
	"testing"

	"learning-serbian-platform/backend/internal/apperr"
	"learning-serbian-platform/backend/internal/learning"
)

const annotationID = "019a0000-0000-7000-8000-0000000000b1"

var testAnnotation = learning.Annotation{
	ID: annotationID, DocumentID: documentID, PageNumber: 2,
	Rects: []learning.Rect{{X: 0.1, Y: 0.2, Width: 0.5, Height: 0.02}},
	Quote: "Dobar dan", Note: "a greeting", CreatedAt: testTime, UpdatedAt: testTime,
}

// fakeAnnotations records what the handlers pass in and answers with
// testAnnotation, or err when set.
type fakeAnnotations struct {
	err    error
	calls  int
	lastID string
	lastIn learning.NewAnnotation
	note   string
}

func (f *fakeAnnotations) Create(_ context.Context, in learning.NewAnnotation) (learning.Annotation, error) {
	f.calls++
	f.lastIn = in
	return testAnnotation, f.err
}

func (f *fakeAnnotations) List(_ context.Context, documentID string) ([]learning.Annotation, error) {
	f.calls++
	f.lastID = documentID
	return []learning.Annotation{testAnnotation}, f.err
}

func (f *fakeAnnotations) UpdateNote(_ context.Context, id, note string) (learning.Annotation, error) {
	f.calls++
	f.lastID, f.note = id, note
	return testAnnotation, f.err
}

func (f *fakeAnnotations) Delete(_ context.Context, id string) error {
	f.calls++
	f.lastID = id
	return f.err
}

func newAnnotationsRouter(a *fakeAnnotations) http.Handler {
	return NewRouter(Dependencies{DB: fakePinger{}, Auth: &fakeAuth{}, Annotations: a, AppOrigin: testOrigin, CookieSecure: true})
}

const highlightBody = `{"pageNumber":2,"rects":[{"x":0.1,"y":0.2,"width":0.5,"height":0.02}],"quote":"Dobar dan","note":""}`

func TestAnnotationRoutesRequireSession(t *testing.T) {
	routes := []struct{ method, path, body string }{
		{http.MethodGet, "/api/documents/" + documentID + "/annotations", ""},
		{http.MethodPost, "/api/documents/" + documentID + "/annotations", highlightBody},
		{http.MethodPut, "/api/annotations/" + annotationID, `{"note":"x"}`},
		{http.MethodDelete, "/api/annotations/" + annotationID, ""},
	}
	for _, rt := range routes {
		t.Run(rt.method+" "+rt.path, func(t *testing.T) {
			a := &fakeAnnotations{}
			rec := do(t, newAnnotationsRouter(a), rt.method, rt.path, requestOpts{body: rt.body, origin: testOrigin})
			if rec.Code != http.StatusUnauthorized || a.calls != 0 {
				t.Errorf("status = %d, service calls = %d; want 401 and 0", rec.Code, a.calls)
			}
		})
	}
}

func TestCreateAnnotation(t *testing.T) {
	a := &fakeAnnotations{}
	rec := do(t, newAnnotationsRouter(a), http.MethodPost, "/api/documents/"+documentID+"/annotations", loggedIn(highlightBody))

	if rec.Code != http.StatusCreated {
		t.Fatalf("status = %d, body = %s", rec.Code, rec.Body)
	}
	want := learning.NewAnnotation{
		DocumentID: documentID, PageNumber: 2, Quote: "Dobar dan",
		Rects: []learning.Rect{{X: 0.1, Y: 0.2, Width: 0.5, Height: 0.02}},
	}
	if a.lastIn.DocumentID != want.DocumentID || a.lastIn.PageNumber != 2 || a.lastIn.Quote != want.Quote || !slices.Equal(a.lastIn.Rects, want.Rects) {
		t.Errorf("service received %+v, want %+v", a.lastIn, want)
	}

	// The JSON keys are the API contract the frontend reads.
	var body map[string]map[string]any
	if err := json.NewDecoder(rec.Body).Decode(&body); err != nil {
		t.Fatal(err)
	}
	got := body["annotation"]
	for key, value := range map[string]any{
		"id": annotationID, "documentId": documentID, "pageNumber": 2.0, "quote": "Dobar dan",
		"note": "a greeting", "createdAt": "2026-10-23T12:00:00Z",
	} {
		if got[key] != value {
			t.Errorf("annotation[%q] = %v, want %v", key, got[key], value)
		}
	}
	rects, _ := got["rects"].([]any)
	if len(rects) != 1 || rects[0].(map[string]any)["width"] != 0.5 {
		t.Errorf("rects = %v", got["rects"])
	}
}

func TestCreateAnnotationRejectsUnknownFields(t *testing.T) {
	a := &fakeAnnotations{}
	rec := do(t, newAnnotationsRouter(a), http.MethodPost, "/api/documents/"+documentID+"/annotations",
		loggedIn(`{"pageNumber":2,"rects":[],"quote":"x","color":"red"}`))
	if rec.Code != http.StatusUnprocessableEntity || decodeError(t, rec).Code != "invalid_json" || a.calls != 0 {
		t.Errorf("status = %d, calls = %d; want 422 invalid_json and 0", rec.Code, a.calls)
	}
}

func TestListAnnotations(t *testing.T) {
	a := &fakeAnnotations{}
	rec := do(t, newAnnotationsRouter(a), http.MethodGet, "/api/documents/"+documentID+"/annotations", loggedIn(""))

	if rec.Code != http.StatusOK || a.lastID != documentID {
		t.Fatalf("status = %d, document = %q", rec.Code, a.lastID)
	}
	var body struct {
		Annotations []map[string]any `json:"annotations"`
	}
	if err := json.NewDecoder(rec.Body).Decode(&body); err != nil {
		t.Fatal(err)
	}
	if len(body.Annotations) != 1 || body.Annotations[0]["id"] != annotationID {
		t.Errorf("annotations = %v", body.Annotations)
	}
}

func TestUpdateAndDeleteAnnotation(t *testing.T) {
	a := &fakeAnnotations{}
	h := newAnnotationsRouter(a)

	rec := do(t, h, http.MethodPut, "/api/annotations/"+annotationID, loggedIn(`{"note":"hello"}`))
	if rec.Code != http.StatusOK || a.lastID != annotationID || a.note != "hello" {
		t.Errorf("update = %d, service got %q %q", rec.Code, a.lastID, a.note)
	}
	rec = do(t, h, http.MethodDelete, "/api/annotations/"+annotationID, loggedIn(""))
	if rec.Code != http.StatusNoContent || a.lastID != annotationID {
		t.Errorf("delete = %d, service got %q", rec.Code, a.lastID)
	}
}

func TestAnnotationErrorsReachTheClient(t *testing.T) {
	cases := []struct {
		err    error
		status int
	}{
		{apperr.New(apperr.NotFound, "document_not_found", "This PDF does not exist."), http.StatusNotFound},
		{apperr.New(apperr.Forbidden, "students_only", "This is only available to student accounts."), http.StatusForbidden},
		{apperr.Validation(map[string]string{"rects": "Each rectangle must lie within the page (0 to 1)."}), http.StatusUnprocessableEntity},
	}
	for _, c := range cases {
		a := &fakeAnnotations{err: c.err}
		rec := do(t, newAnnotationsRouter(a), http.MethodPost, "/api/documents/"+documentID+"/annotations", loggedIn(highlightBody))
		if rec.Code != c.status {
			t.Errorf("%v: status = %d, want %d", c.err, rec.Code, c.status)
		}
	}
}
