//go:build integration

package httpapi

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"io/fs"
	"mime/multipart"
	"net/http"
	"path/filepath"
	"testing"
)

// These tests run lessons and documents against PostgreSQL and a real file
// store in a temporary directory, so they cover what the unit tests fake:
// the queries, the composite foreign key, the delete cascade and the files
// on disk.

// samplePDF is not a renderable PDF, but it starts the way every PDF does,
// which is all the API checks.
const samplePDF = "%PDF-1.7\n% Sveska integration test\n"

// uploadFile posts content as the "file" part of a multipart form.
func (b *browser) uploadFile(path, filename, content string) (*http.Response, map[string]any) {
	b.t.Helper()
	var buf bytes.Buffer
	mw := multipart.NewWriter(&buf)
	w, err := mw.CreateFormFile("file", filename)
	if err != nil {
		b.t.Fatal(err)
	}
	_, _ = io.WriteString(w, content)
	_ = mw.Close()

	req, err := http.NewRequest(http.MethodPost, b.base+path, &buf)
	if err != nil {
		b.t.Fatal(err)
	}
	req.Header.Set("Origin", testOrigin)
	req.Header.Set("Content-Type", mw.FormDataContentType())
	resp, err := b.client.Do(req)
	if err != nil {
		b.t.Fatal(err)
	}
	defer resp.Body.Close()
	var decoded map[string]any
	_ = json.NewDecoder(resp.Body).Decode(&decoded)
	return resp, decoded
}

// download fetches raw bytes; send would try to decode them as JSON.
func (b *browser) download(path string) (*http.Response, string) {
	b.t.Helper()
	resp, err := b.client.Get(b.base + path)
	if err != nil {
		b.t.Fatal(err)
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	return resp, string(body)
}

func (b *browser) createLesson(title string) string {
	b.t.Helper()
	resp, body := b.send(http.MethodPost, "/api/lessons", map[string]string{"title": title})
	if resp.StatusCode != http.StatusCreated {
		b.t.Fatalf("create lesson = %d %v", resp.StatusCode, body)
	}
	return body["lesson"].(map[string]any)["id"].(string)
}

func (b *browser) uploadPDF(lessonID, filename string) string {
	b.t.Helper()
	resp, body := b.uploadFile("/api/lessons/"+lessonID+"/documents", filename, samplePDF)
	if resp.StatusCode != http.StatusCreated {
		b.t.Fatalf("upload = %d %v", resp.StatusCode, body)
	}
	return body["document"].(map[string]any)["id"].(string)
}

// storedFiles counts the files in the store, ignoring directories.
func storedFiles(t *testing.T, dir string) int {
	t.Helper()
	n := 0
	err := filepath.WalkDir(dir, func(_ string, d fs.DirEntry, err error) error {
		if err == nil && !d.IsDir() {
			n++
		}
		return err
	})
	if err != nil {
		t.Fatal(err)
	}
	return n
}

func TestLessonWithDocumentsFromUploadToDelete(t *testing.T) {
	srv, _, filesDir := newStackWithFiles(t)
	ana := signedUp(t, newBrowser(t, srv), "ana@example.com")

	lessonID := ana.createLesson("Lekcija 1")
	docID := ana.uploadPDF(lessonID, "Čitanka.pdf")
	if n := storedFiles(t, filesDir); n != 1 {
		t.Fatalf("%d files stored, want 1", n)
	}

	resp, body := ana.send(http.MethodGet, "/api/lessons/"+lessonID, nil)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("get lesson = %d %v", resp.StatusCode, body)
	}
	docs := body["documents"].([]any)
	doc := docs[0].(map[string]any)
	if len(docs) != 1 || doc["title"] != "Čitanka" || doc["byteSize"] != float64(len(samplePDF)) {
		t.Errorf("documents = %v", docs)
	}
	if body["lesson"].(map[string]any)["documentCount"] != 1.0 {
		t.Errorf("lesson = %v", body["lesson"])
	}

	resp, body = ana.send(http.MethodGet, "/api/documents/"+docID, nil)
	if resp.StatusCode != http.StatusOK || body["lesson"].(map[string]any)["title"] != "Lekcija 1" {
		t.Errorf("get document = %d %v", resp.StatusCode, body)
	}

	resp, content := ana.download("/api/documents/" + docID + "/file")
	if resp.StatusCode != http.StatusOK || content != samplePDF || resp.Header.Get("Content-Type") != "application/pdf" {
		t.Errorf("download = %d %q %q", resp.StatusCode, resp.Header.Get("Content-Type"), content)
	}

	if resp, _ := ana.send(http.MethodDelete, "/api/documents/"+docID, nil); resp.StatusCode != http.StatusNoContent {
		t.Fatalf("delete document = %d", resp.StatusCode)
	}
	if n := storedFiles(t, filesDir); n != 0 {
		t.Errorf("%d files left after deleting the document", n)
	}
	if resp, _ := ana.download("/api/documents/" + docID + "/file"); resp.StatusCode != http.StatusNotFound {
		t.Errorf("download after delete = %d, want 404", resp.StatusCode)
	}
}

func TestDeletingALessonDeletesItsDocumentsAndFiles(t *testing.T) {
	srv, pool, filesDir := newStackWithFiles(t)
	ana := signedUp(t, newBrowser(t, srv), "ana@example.com")
	doomed := ana.createLesson("Lekcija 1")
	ana.uploadPDF(doomed, "a.pdf")
	ana.uploadPDF(doomed, "b.pdf")
	kept := ana.createLesson("Lekcija 2")
	keptDoc := ana.uploadPDF(kept, "c.pdf")

	if resp, _ := ana.send(http.MethodDelete, "/api/lessons/"+doomed, nil); resp.StatusCode != http.StatusNoContent {
		t.Fatalf("delete lesson = %d", resp.StatusCode)
	}
	if n := storedFiles(t, filesDir); n != 1 {
		t.Errorf("%d files stored, want only the other lesson's 1", n)
	}
	var ids []string
	rows, err := pool.Query(context.Background(), `SELECT id::text FROM documents`)
	if err != nil {
		t.Fatal(err)
	}
	for rows.Next() {
		var id string
		_ = rows.Scan(&id)
		ids = append(ids, id)
	}
	if len(ids) != 1 || ids[0] != keptDoc {
		t.Errorf("documents left = %v, want only %s", ids, keptDoc)
	}
	if resp, _ := ana.send(http.MethodDelete, "/api/lessons/"+doomed, nil); resp.StatusCode != http.StatusNotFound {
		t.Errorf("second delete = %d, want 404", resp.StatusCode)
	}
}

func TestLessonsAndDocumentsArePrivate(t *testing.T) {
	srv, _, filesDir := newStackWithFiles(t)
	ana := signedUp(t, newBrowser(t, srv), "ana@example.com")
	marko := signedUp(t, newBrowser(t, srv), "marko@example.com")
	lessonID := ana.createLesson("Ana's")
	docID := ana.uploadPDF(lessonID, "a.pdf")

	if _, body := marko.send(http.MethodGet, "/api/lessons", nil); len(body["lessons"].([]any)) != 0 {
		t.Errorf("Marko's list = %v", body)
	}
	checks := []struct {
		name string
		do   func() *http.Response
	}{
		{"get lesson", func() *http.Response { r, _ := marko.send(http.MethodGet, "/api/lessons/"+lessonID, nil); return r }},
		{"update lesson", func() *http.Response {
			r, _ := marko.send(http.MethodPut, "/api/lessons/"+lessonID, map[string]string{"title": "mine"})
			return r
		}},
		{"delete lesson", func() *http.Response { r, _ := marko.send(http.MethodDelete, "/api/lessons/"+lessonID, nil); return r }},
		{"upload into lesson", func() *http.Response {
			r, _ := marko.uploadFile("/api/lessons/"+lessonID+"/documents", "x.pdf", samplePDF)
			return r
		}},
		{"get document", func() *http.Response { r, _ := marko.send(http.MethodGet, "/api/documents/"+docID, nil); return r }},
		{"download document", func() *http.Response { r, _ := marko.download("/api/documents/" + docID + "/file"); return r }},
		{"delete document", func() *http.Response { r, _ := marko.send(http.MethodDelete, "/api/documents/"+docID, nil); return r }},
	}
	for _, c := range checks {
		t.Run(c.name, func(t *testing.T) {
			if resp := c.do(); resp.StatusCode != http.StatusNotFound {
				t.Errorf("status = %d, want 404", resp.StatusCode)
			}
		})
	}
	if resp, content := ana.download("/api/documents/" + docID + "/file"); resp.StatusCode != http.StatusOK || content != samplePDF {
		t.Errorf("Ana's download after Marko's attempts = %d", resp.StatusCode)
	}
	if n := storedFiles(t, filesDir); n != 1 {
		t.Errorf("%d files stored, want Ana's 1 (Marko's upload must not store one)", n)
	}
}

func TestUploadRejectsFilesThatAreNotPDFs(t *testing.T) {
	srv, _, filesDir := newStackWithFiles(t)
	ana := signedUp(t, newBrowser(t, srv), "ana@example.com")
	lessonID := ana.createLesson("Lekcija 1")

	resp, body := ana.uploadFile("/api/lessons/"+lessonID+"/documents", "notes.pdf", "<html>not really</html>")
	if resp.StatusCode != http.StatusUnprocessableEntity {
		t.Fatalf("status = %d, body = %v", resp.StatusCode, body)
	}
	if fields, _ := body["error"].(map[string]any)["fields"].(map[string]any); fields["file"] != "Choose a PDF file." {
		t.Errorf("body = %v", body)
	}
	if n := storedFiles(t, filesDir); n != 0 {
		t.Errorf("%d files stored, want 0", n)
	}
}

func TestLessonListIsNewestFirstWithCounts(t *testing.T) {
	srv, _ := newStack(t)
	ana := signedUp(t, newBrowser(t, srv), "ana@example.com")
	first := ana.createLesson("Prva")
	ana.uploadPDF(first, "a.pdf")
	ana.createLesson("Druga")

	_, body := ana.send(http.MethodGet, "/api/lessons", nil)
	lessons := body["lessons"].([]any)
	if len(lessons) != 2 {
		t.Fatalf("lessons = %v", lessons)
	}
	newest, oldest := lessons[0].(map[string]any), lessons[1].(map[string]any)
	if newest["title"] != "Druga" || newest["documentCount"] != 0.0 || oldest["title"] != "Prva" || oldest["documentCount"] != 1.0 {
		t.Errorf("lessons = %v", lessons)
	}
}

func TestDocumentOwnerMustOwnTheLesson(t *testing.T) {
	srv, pool := newStack(t)
	ana := signedUp(t, newBrowser(t, srv), "ana@example.com")
	signedUp(t, newBrowser(t, srv), "marko@example.com")
	lessonID := ana.createLesson("Ana's")

	// Bypass the API: the database itself must refuse a document whose
	// owner is not the lesson's owner.
	_, err := pool.Exec(context.Background(), `
		INSERT INTO documents (owner_id, lesson_id, title, storage_key, byte_size, sha256)
		SELECT u.id, $1, 'x', 'k', 1, sha256('x')
		FROM users u WHERE u.email = 'marko@example.com'`, lessonID)
	if err == nil {
		t.Error("insert succeeded, want documents_lesson_fkey to refuse it")
	}
}
