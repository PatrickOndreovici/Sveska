package httpapi

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"learning-serbian-platform/backend/internal/apperr"
	"learning-serbian-platform/backend/internal/learning"
)

const (
	lessonID   = "019a0000-0000-7000-8000-0000000000a1"
	documentID = "019a0000-0000-7000-8000-0000000000d1"
	pdfBytes   = "%PDF-1.7 fake"
)

var (
	testTime   = time.Date(2026, 10, 23, 12, 0, 0, 0, time.UTC)
	testLesson = learning.Lesson{ID: lessonID, Title: "Lekcija 1", Description: "Padeži", DocumentCount: 1, CreatedAt: testTime, UpdatedAt: testTime}
	testDoc    = learning.Document{ID: documentID, LessonID: lessonID, Title: "Čitanka", ByteSize: int64(len(pdfBytes)), CreatedAt: testTime}
)

// fakeLessons records what the handlers pass in and answers with the test
// lesson and document, or err when set.
type fakeLessons struct {
	err          error
	calls        int
	lastFields   learning.LessonFields
	lastID       string
	lastUpload   learning.UploadInput
	uploadedBody []byte
}

func (f *fakeLessons) CreateLesson(_ context.Context, in learning.LessonFields) (learning.Lesson, error) {
	f.calls++
	f.lastFields = in
	return testLesson, f.err
}

func (f *fakeLessons) ListLessons(context.Context) ([]learning.Lesson, error) {
	f.calls++
	return []learning.Lesson{testLesson}, f.err
}

func (f *fakeLessons) GetLesson(_ context.Context, id string) (learning.Lesson, []learning.Document, error) {
	f.calls++
	f.lastID = id
	return testLesson, []learning.Document{testDoc}, f.err
}

func (f *fakeLessons) UpdateLesson(_ context.Context, id string, in learning.LessonFields) (learning.Lesson, error) {
	f.calls++
	f.lastID, f.lastFields = id, in
	return testLesson, f.err
}

func (f *fakeLessons) DeleteLesson(_ context.Context, id string) error {
	f.calls++
	f.lastID = id
	return f.err
}

// UploadDocument reads the whole body, like the real service streaming it
// to storage, so the request size limit is exercised.
func (f *fakeLessons) UploadDocument(_ context.Context, in learning.UploadInput) (learning.Document, error) {
	f.calls++
	f.lastUpload = in
	body, err := io.ReadAll(in.Body)
	f.uploadedBody = body
	if err != nil {
		return learning.Document{}, err
	}
	return testDoc, f.err
}

func (f *fakeLessons) GetDocument(_ context.Context, id string) (learning.DocumentInLesson, error) {
	f.calls++
	f.lastID = id
	return learning.DocumentInLesson{Document: testDoc, LessonTitle: testLesson.Title}, f.err
}

func (f *fakeLessons) OpenDocument(_ context.Context, id string) (learning.Document, io.ReadSeekCloser, error) {
	f.calls++
	f.lastID = id
	if f.err != nil {
		return learning.Document{}, nil, f.err
	}
	return testDoc, nopReadSeekCloser{strings.NewReader(pdfBytes)}, nil
}

func (f *fakeLessons) DeleteDocument(_ context.Context, id string) error {
	f.calls++
	f.lastID = id
	return f.err
}

type nopReadSeekCloser struct{ io.ReadSeeker }

func (nopReadSeekCloser) Close() error { return nil }

func newLessonsRouter(l *fakeLessons) http.Handler {
	return NewRouter(Dependencies{DB: fakePinger{}, Auth: &fakeAuth{}, Lessons: l, AppOrigin: testOrigin, CookieSecure: true})
}

// multipartBody builds a form with one part per field; "file" is a file part.
func multipartBody(t *testing.T, fields map[string]string) (string, *bytes.Buffer) {
	t.Helper()
	var buf bytes.Buffer
	mw := multipart.NewWriter(&buf)
	for name, value := range fields {
		var w io.Writer
		var err error
		if name == "file" {
			w, err = mw.CreateFormFile("file", "Čitanka.pdf")
		} else {
			w, err = mw.CreateFormField(name)
		}
		if err != nil {
			t.Fatal(err)
		}
		_, _ = io.WriteString(w, value)
	}
	if err := mw.Close(); err != nil {
		t.Fatal(err)
	}
	return mw.FormDataContentType(), &buf
}

func upload(t *testing.T, h http.Handler, contentType string, body io.Reader) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest(http.MethodPost, "/api/lessons/"+lessonID+"/documents", body)
	req.Header.Set("Content-Type", contentType)
	req.Header.Set("Origin", testOrigin)
	req.AddCookie(&http.Cookie{Name: sessionCookieName, Value: validToken})
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	return rec
}

func TestLessonRoutesRequireSession(t *testing.T) {
	routes := []struct{ method, path string }{
		{http.MethodGet, "/api/lessons"},
		{http.MethodPost, "/api/lessons"},
		{http.MethodGet, "/api/lessons/" + lessonID},
		{http.MethodPut, "/api/lessons/" + lessonID},
		{http.MethodDelete, "/api/lessons/" + lessonID},
		{http.MethodPost, "/api/lessons/" + lessonID + "/documents"},
		{http.MethodGet, "/api/documents/" + documentID},
		{http.MethodGet, "/api/documents/" + documentID + "/file"},
		{http.MethodDelete, "/api/documents/" + documentID},
	}
	for _, rt := range routes {
		t.Run(rt.method+" "+rt.path, func(t *testing.T) {
			l := &fakeLessons{}
			rec := do(t, newLessonsRouter(l), rt.method, rt.path, requestOpts{origin: testOrigin})
			if rec.Code != http.StatusUnauthorized {
				t.Errorf("status = %d, want 401", rec.Code)
			}
			if l.calls != 0 {
				t.Errorf("service called %d times, want 0", l.calls)
			}
		})
	}
}

func TestCreateAndUpdateLesson(t *testing.T) {
	l := &fakeLessons{}
	h := newLessonsRouter(l)

	rec := do(t, h, http.MethodPost, "/api/lessons", loggedIn(`{"title":"Lekcija 1","description":"Padeži"}`))
	if rec.Code != http.StatusCreated {
		t.Fatalf("create status = %d, body = %s", rec.Code, rec.Body)
	}
	if l.lastFields != (learning.LessonFields{Title: "Lekcija 1", Description: "Padeži"}) {
		t.Errorf("service received %+v", l.lastFields)
	}
	var body map[string]map[string]any
	if err := json.NewDecoder(rec.Body).Decode(&body); err != nil {
		t.Fatal(err)
	}
	if body["lesson"]["id"] != lessonID || body["lesson"]["documentCount"] != 1.0 || body["lesson"]["createdAt"] != "2026-10-23T12:00:00Z" {
		t.Errorf("lesson = %v", body["lesson"])
	}

	rec = do(t, h, http.MethodPut, "/api/lessons/"+lessonID, loggedIn(`{"title":"Lekcija 2"}`))
	if rec.Code != http.StatusOK || l.lastID != lessonID || l.lastFields.Title != "Lekcija 2" {
		t.Errorf("update = %d, service got %q %+v", rec.Code, l.lastID, l.lastFields)
	}
}

func TestListAndGetLessons(t *testing.T) {
	h := newLessonsRouter(&fakeLessons{})

	rec := do(t, h, http.MethodGet, "/api/lessons", loggedIn(""))
	var list lessonListResponse
	if err := json.NewDecoder(rec.Body).Decode(&list); err != nil || rec.Code != http.StatusOK {
		t.Fatalf("list = %d, %v", rec.Code, err)
	}
	if len(list.Lessons) != 1 || list.Lessons[0].Title != "Lekcija 1" {
		t.Errorf("lessons = %+v", list.Lessons)
	}

	rec = do(t, h, http.MethodGet, "/api/lessons/"+lessonID, loggedIn(""))
	var detail lessonDetailResponse
	if err := json.NewDecoder(rec.Body).Decode(&detail); err != nil || rec.Code != http.StatusOK {
		t.Fatalf("get = %d, %v", rec.Code, err)
	}
	if detail.Lesson.ID != lessonID || len(detail.Documents) != 1 || detail.Documents[0].LessonID != lessonID {
		t.Errorf("detail = %+v", detail)
	}
}

func TestUploadDocumentStreamsTheFilePart(t *testing.T) {
	l := &fakeLessons{}
	// A field before the file must be skipped, not mistaken for the file.
	contentType, body := multipartBody(t, map[string]string{"note": "ignored", "file": pdfBytes})

	rec := upload(t, newLessonsRouter(l), contentType, body)

	if rec.Code != http.StatusCreated {
		t.Fatalf("status = %d, body = %s", rec.Code, rec.Body)
	}
	if l.lastUpload.LessonID != lessonID || l.lastUpload.FileName != "Čitanka.pdf" || string(l.uploadedBody) != pdfBytes {
		t.Errorf("service got lesson %q, name %q, body %q", l.lastUpload.LessonID, l.lastUpload.FileName, l.uploadedBody)
	}
	var env documentEnvelope
	if err := json.NewDecoder(rec.Body).Decode(&env); err != nil {
		t.Fatal(err)
	}
	if env.Document.ID != documentID || env.Document.ByteSize != int64(len(pdfBytes)) {
		t.Errorf("document = %+v", env.Document)
	}
}

func TestUploadDocumentRejectsBadRequests(t *testing.T) {
	noFileType, noFileBody := multipartBody(t, map[string]string{"title": "x"})
	tests := []struct {
		name        string
		contentType string
		body        io.Reader
		code        string
		field       string
	}{
		{"JSON instead of a form", "application/json", strings.NewReader(`{}`), "unsupported_content_type", ""},
		{"form without a file", noFileType, noFileBody, "validation_failed", "file"},
		{"truncated form", "multipart/form-data; boundary=xyz", strings.NewReader("--xyz\r\nContent-Disposition: form-data; name=\"file\"; filename=\"a.pdf\"\r\n\r\n%PDF-"), "", ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rec := upload(t, newLessonsRouter(&fakeLessons{}), tt.contentType, tt.body)
			if rec.Code != http.StatusUnprocessableEntity {
				t.Fatalf("status = %d, want 422 (%s)", rec.Code, rec.Body)
			}
			e := decodeError(t, rec)
			if tt.code != "" && e.Code != tt.code {
				t.Errorf("code = %q, want %q", e.Code, tt.code)
			}
			if tt.field != "" && e.Fields[tt.field] == "" {
				t.Errorf("fields = %v, want a problem for %q", e.Fields, tt.field)
			}
		})
	}
}

func TestUploadDocumentOverTheRequestLimitIsAFieldError(t *testing.T) {
	// Bigger than the file limit plus the multipart allowance, so the
	// handler's MaxBytesReader stops it whatever the service does.
	contentType, body := multipartBody(t, map[string]string{"file": "%PDF-" + strings.Repeat("0", learning.MaxDocumentBytes+maxMultipartOverhead)})

	rec := upload(t, newLessonsRouter(&fakeLessons{}), contentType, body)

	if rec.Code != http.StatusUnprocessableEntity {
		t.Fatalf("status = %d, want 422", rec.Code)
	}
	if got := decodeError(t, rec).Fields["file"]; got != "Choose a PDF of at most 20 MB." {
		t.Errorf("file problem = %q", got)
	}
}

func TestDownloadDocumentIsAnAttachmentThatCannotRun(t *testing.T) {
	l := &fakeLessons{}
	rec := do(t, newLessonsRouter(l), http.MethodGet, "/api/documents/"+documentID+"/file", loggedIn(""))

	if rec.Code != http.StatusOK || rec.Body.String() != pdfBytes {
		t.Fatalf("status = %d, body = %q", rec.Code, rec.Body)
	}
	h := rec.Header()
	if h.Get("Content-Type") != "application/pdf" {
		t.Errorf("Content-Type = %q", h.Get("Content-Type"))
	}
	// Non-ASCII names are encoded, so "Čitanka" survives the header.
	if got := h.Get("Content-Disposition"); got != "attachment; filename*=utf-8''%C4%8Citanka.pdf" {
		t.Errorf("Content-Disposition = %q", got)
	}
	if !strings.Contains(h.Get("Content-Security-Policy"), "sandbox") {
		t.Errorf("Content-Security-Policy = %q, want sandbox", h.Get("Content-Security-Policy"))
	}
	if h.Get("X-Content-Type-Options") != "nosniff" || h.Get("Accept-Ranges") != "bytes" {
		t.Errorf("headers = %v", h)
	}
	if l.lastID != documentID {
		t.Errorf("service got id %q", l.lastID)
	}
}

func TestDownloadDocumentAnswersRangeRequests(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/api/documents/"+documentID+"/file", nil)
	req.AddCookie(&http.Cookie{Name: sessionCookieName, Value: validToken})
	req.Header.Set("Range", "bytes=0-4")
	rec := httptest.NewRecorder()
	newLessonsRouter(&fakeLessons{}).ServeHTTP(rec, req)

	if rec.Code != http.StatusPartialContent || rec.Body.String() != "%PDF-" {
		t.Errorf("status = %d, body = %q; want 206 %%PDF-", rec.Code, rec.Body)
	}
}

func TestGetDocumentIncludesItsLesson(t *testing.T) {
	rec := do(t, newLessonsRouter(&fakeLessons{}), http.MethodGet, "/api/documents/"+documentID, loggedIn(""))
	var body documentDetailResponse
	if err := json.NewDecoder(rec.Body).Decode(&body); err != nil || rec.Code != http.StatusOK {
		t.Fatalf("status = %d, %v", rec.Code, err)
	}
	if body.Document.Title != "Čitanka" || body.Lesson != (lessonRef{ID: lessonID, Title: "Lekcija 1"}) {
		t.Errorf("body = %+v", body)
	}
}

func TestDeleteLessonAndDocument(t *testing.T) {
	for _, path := range []string{"/api/lessons/" + lessonID, "/api/documents/" + documentID} {
		l := &fakeLessons{}
		rec := do(t, newLessonsRouter(l), http.MethodDelete, path, loggedIn(""))
		if rec.Code != http.StatusNoContent || l.calls != 1 {
			t.Errorf("DELETE %s = %d, service calls %d", path, rec.Code, l.calls)
		}
	}
}

func TestLessonNotFoundReachesTheClient(t *testing.T) {
	l := &fakeLessons{err: apperr.New(apperr.NotFound, "lesson_not_found", "This lesson does not exist.")}
	rec := do(t, newLessonsRouter(l), http.MethodGet, "/api/lessons/"+lessonID, loggedIn(""))
	if rec.Code != http.StatusNotFound || decodeError(t, rec).Code != "lesson_not_found" {
		t.Errorf("status = %d, body = %s", rec.Code, rec.Body)
	}
}
