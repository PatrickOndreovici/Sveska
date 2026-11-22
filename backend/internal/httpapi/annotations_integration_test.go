//go:build integration

package httpapi

import (
	"context"
	"net/http"
	"testing"
)

// These tests run annotations and words saved from them against
// PostgreSQL, covering what the unit tests fake: the jsonb round trip, the
// composite foreign keys, ON DELETE SET NULL on a word's source and the
// cascades from documents and lessons.

func highlight(page int, quote string) map[string]any {
	return map[string]any{
		"pageNumber": page,
		"rects":      []map[string]float64{{"x": 0.125, "y": 0.25, "width": 0.5, "height": 0.03125}},
		"quote":      quote,
	}
}

func (b *browser) annotate(docID string, body map[string]any) string {
	b.t.Helper()
	resp, out := b.send(http.MethodPost, "/api/documents/"+docID+"/annotations", body)
	if resp.StatusCode != http.StatusCreated {
		b.t.Fatalf("create annotation = %d %v", resp.StatusCode, out)
	}
	return out["annotation"].(map[string]any)["id"].(string)
}

// studentWithPDF signs Ana up and gives her a lesson with one PDF.
func studentWithPDF(t *testing.T) (*browser, string) {
	t.Helper()
	srv, _, _ := newStackWithFiles(t)
	ana := signedUp(t, newBrowser(t, srv), "ana@example.com")
	docID := ana.uploadPDF(ana.createLesson("Lekcija 1"), "Čitanka.pdf")
	return ana, docID
}

func TestAnnotationsFromHighlightToNote(t *testing.T) {
	ana, docID := studentWithPDF(t)
	later := ana.annotate(docID, highlight(2, "Dobar dan"))
	first := ana.annotate(docID, highlight(1, "Zdravo"))

	resp, body := ana.send(http.MethodGet, "/api/documents/"+docID+"/annotations", nil)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("list = %d %v", resp.StatusCode, body)
	}
	list := body["annotations"].([]any)
	if len(list) != 2 || list[0].(map[string]any)["id"] != first || list[1].(map[string]any)["id"] != later {
		t.Fatalf("annotations = %v, want page 1 then page 2", list)
	}
	rect := list[1].(map[string]any)["rects"].([]any)[0].(map[string]any)
	if rect["x"] != 0.125 || rect["y"] != 0.25 || rect["width"] != 0.5 || rect["height"] != 0.03125 {
		t.Errorf("rect after the jsonb round trip = %v", rect)
	}

	resp, body = ana.send(http.MethodPut, "/api/annotations/"+later, map[string]string{"note": " a greeting "})
	if resp.StatusCode != http.StatusOK || body["annotation"].(map[string]any)["note"] != "a greeting" {
		t.Errorf("update note = %d %v", resp.StatusCode, body)
	}

	if resp, _ := ana.send(http.MethodDelete, "/api/annotations/"+first, nil); resp.StatusCode != http.StatusNoContent {
		t.Fatalf("delete = %d", resp.StatusCode)
	}
	_, body = ana.send(http.MethodGet, "/api/documents/"+docID+"/annotations", nil)
	if n := len(body["annotations"].([]any)); n != 1 {
		t.Errorf("%d annotations left, want 1", n)
	}
}

func TestAnnotationValidationIsA422NotA500(t *testing.T) {
	ana, docID := studentWithPDF(t)
	bad := highlight(1, "x")
	bad["rects"] = []map[string]float64{{"x": 0.9, "y": 0, "width": 0.5, "height": 0.1}}

	resp, body := ana.send(http.MethodPost, "/api/documents/"+docID+"/annotations", bad)
	if resp.StatusCode != http.StatusUnprocessableEntity || errorCode(body) != "validation_failed" {
		t.Errorf("= %d %v, want 422 validation_failed", resp.StatusCode, body)
	}
}

func TestAWordSavedFromAHighlightOutlivesIt(t *testing.T) {
	ana, docID := studentWithPDF(t)
	annotationID := ana.annotate(docID, highlight(3, "kuća"))

	resp, body := ana.send(http.MethodPost, "/api/vocabulary", map[string]string{
		"term": "kuća", "translation": "house", "sourceAnnotationId": annotationID,
	})
	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("create entry = %d %v", resp.StatusCode, body)
	}
	source := body["entry"].(map[string]any)["source"].(map[string]any)
	if source["documentId"] != docID || source["documentTitle"] != "Čitanka" || source["pageNumber"] != 3.0 {
		t.Errorf("source = %v", source)
	}
	_, body = ana.send(http.MethodGet, "/api/vocabulary", nil)
	listed := body["entries"].([]any)[0].(map[string]any)
	if listed["source"].(map[string]any)["annotationId"] != annotationID {
		t.Errorf("listed entry = %v", listed)
	}

	if resp, _ := ana.send(http.MethodDelete, "/api/annotations/"+annotationID, nil); resp.StatusCode != http.StatusNoContent {
		t.Fatalf("delete annotation = %d", resp.StatusCode)
	}
	_, body = ana.send(http.MethodGet, "/api/vocabulary", nil)
	entries := body["entries"].([]any)
	if len(entries) != 1 || entries[0].(map[string]any)["source"] != nil {
		t.Errorf("entries after deleting the highlight = %v, want the word with a null source", entries)
	}
}

func TestAnnotationsArePrivate(t *testing.T) {
	srv, _, _ := newStackWithFiles(t)
	ana := signedUp(t, newBrowser(t, srv), "ana@example.com")
	marko := signedUp(t, newBrowser(t, srv), "marko@example.com")
	docID := ana.uploadPDF(ana.createLesson("Ana's"), "a.pdf")
	annotationID := ana.annotate(docID, highlight(1, "Ana's highlight"))

	checks := []struct {
		name string
		do   func() (*http.Response, map[string]any)
	}{
		{"list", func() (*http.Response, map[string]any) {
			return marko.send(http.MethodGet, "/api/documents/"+docID+"/annotations", nil)
		}},
		{"create", func() (*http.Response, map[string]any) {
			return marko.send(http.MethodPost, "/api/documents/"+docID+"/annotations", highlight(1, "mine"))
		}},
		{"update", func() (*http.Response, map[string]any) {
			return marko.send(http.MethodPut, "/api/annotations/"+annotationID, map[string]string{"note": "mine"})
		}},
		{"delete", func() (*http.Response, map[string]any) {
			return marko.send(http.MethodDelete, "/api/annotations/"+annotationID, nil)
		}},
		{"cite in vocabulary", func() (*http.Response, map[string]any) {
			return marko.send(http.MethodPost, "/api/vocabulary", map[string]string{
				"term": "x", "translation": "y", "sourceAnnotationId": annotationID,
			})
		}},
	}
	for _, c := range checks {
		t.Run(c.name, func(t *testing.T) {
			if resp, body := c.do(); resp.StatusCode != http.StatusNotFound {
				t.Errorf("= %d %v, want 404", resp.StatusCode, body)
			}
		})
	}
	_, body := ana.send(http.MethodGet, "/api/documents/"+docID+"/annotations", nil)
	got := body["annotations"].([]any)
	if len(got) != 1 || got[0].(map[string]any)["note"] != "" {
		t.Errorf("Ana's annotations after Marko's attempts = %v", got)
	}
}

func TestDeletingADocumentOrLessonDeletesItsAnnotations(t *testing.T) {
	srv, pool, _ := newStackWithFiles(t)
	ana := signedUp(t, newBrowser(t, srv), "ana@example.com")
	lessonID := ana.createLesson("Lekcija 1")
	doc1 := ana.uploadPDF(lessonID, "a.pdf")
	doc2 := ana.uploadPDF(lessonID, "b.pdf")
	ana.annotate(doc1, highlight(1, "one"))
	ana.annotate(doc2, highlight(1, "two"))

	count := func() int {
		var n int
		if err := pool.QueryRow(context.Background(), `SELECT count(*) FROM annotations`).Scan(&n); err != nil {
			t.Fatal(err)
		}
		return n
	}
	if resp, _ := ana.send(http.MethodDelete, "/api/documents/"+doc1, nil); resp.StatusCode != http.StatusNoContent {
		t.Fatalf("delete document = %d", resp.StatusCode)
	}
	if n := count(); n != 1 {
		t.Errorf("%d annotations after deleting a document, want 1", n)
	}
	if resp, _ := ana.send(http.MethodDelete, "/api/lessons/"+lessonID, nil); resp.StatusCode != http.StatusNoContent {
		t.Fatalf("delete lesson = %d", resp.StatusCode)
	}
	if n := count(); n != 0 {
		t.Errorf("%d annotations after deleting the lesson, want 0", n)
	}
}
