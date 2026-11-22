//go:build integration

package httpapi

import (
	"context"
	"net/http"
	"testing"
)

// These tests run strokes and text boxes against PostgreSQL, covering what
// the unit tests fake: the jsonb round trip, the ink_color enum, the
// composite foreign keys and the cascade from documents.

func stroke(page int, color string) map[string]any {
	return map[string]any{
		"pageNumber": page,
		"color":      color,
		"points":     []map[string]float64{{"x": 0.125, "y": 0.25}, {"x": 0.5, "y": 0.375}, {"x": 1, "y": 1}},
	}
}

func textBox(page int, text string) map[string]any {
	return map[string]any{"pageNumber": page, "x": 0.25, "y": 0.5, "text": text, "color": "blue", "fontSize": 14}
}

func (b *browser) create(path, envelope string, body map[string]any) string {
	b.t.Helper()
	resp, out := b.send(http.MethodPost, path, body)
	if resp.StatusCode != http.StatusCreated {
		b.t.Fatalf("POST %s = %d %v", path, resp.StatusCode, out)
	}
	return out[envelope].(map[string]any)["id"].(string)
}

func TestStrokesFromDrawingToErasing(t *testing.T) {
	ana, docID := studentWithPDF(t)
	path := "/api/documents/" + docID + "/strokes"
	later := ana.create(path, "stroke", stroke(2, "red"))
	first := ana.create(path, "stroke", stroke(1, "black"))

	resp, body := ana.send(http.MethodGet, path, nil)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("list = %d %v", resp.StatusCode, body)
	}
	list := body["strokes"].([]any)
	if len(list) != 2 || list[0].(map[string]any)["id"] != first || list[1].(map[string]any)["id"] != later {
		t.Fatalf("strokes = %v, want page 1 then page 2", list)
	}
	red := list[1].(map[string]any)
	points := red["points"].([]any)
	if red["color"] != "red" || len(points) != 3 || points[1].(map[string]any)["y"] != 0.375 {
		t.Errorf("stroke after the jsonb round trip = %v", red)
	}

	if resp, _ := ana.send(http.MethodDelete, "/api/strokes/"+first, nil); resp.StatusCode != http.StatusNoContent {
		t.Fatalf("delete = %d", resp.StatusCode)
	}
	_, body = ana.send(http.MethodGet, path, nil)
	if n := len(body["strokes"].([]any)); n != 1 {
		t.Errorf("%d strokes left, want 1", n)
	}
}

func TestTextBoxesFromTypingToDeleting(t *testing.T) {
	ana, docID := studentWithPDF(t)
	path := "/api/documents/" + docID + "/text-boxes"
	id := ana.create(path, "textBox", textBox(1, " kuća\nkuće "))

	resp, body := ana.send(http.MethodPut, "/api/text-boxes/"+id, map[string]any{
		"x": 0.0625, "y": 0.75, "text": "kuća", "color": "red", "fontSize": 9,
	})
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("update = %d %v", resp.StatusCode, body)
	}

	_, body = ana.send(http.MethodGet, path, nil)
	list := body["textBoxes"].([]any)
	if len(list) != 1 {
		t.Fatalf("text boxes = %v", list)
	}
	got := list[0].(map[string]any)
	if got["x"] != 0.0625 || got["y"] != 0.75 || got["text"] != "kuća" || got["color"] != "red" || got["fontSize"] != 9.0 || got["pageNumber"] != 1.0 {
		t.Errorf("text box = %v", got)
	}

	if resp, _ := ana.send(http.MethodDelete, "/api/text-boxes/"+id, nil); resp.StatusCode != http.StatusNoContent {
		t.Fatalf("delete = %d", resp.StatusCode)
	}
	if resp, _ := ana.send(http.MethodDelete, "/api/text-boxes/"+id, nil); resp.StatusCode != http.StatusNotFound {
		t.Errorf("second delete = %d, want 404", resp.StatusCode)
	}
}

func TestDrawingValidationIsA422NotA500(t *testing.T) {
	ana, docID := studentWithPDF(t)
	badColor := stroke(1, "pink")
	offPage := textBox(1, "x")
	offPage["x"] = 1.5
	cases := map[string]struct {
		path string
		body map[string]any
	}{
		"unknown colour": {"/api/documents/" + docID + "/strokes", badColor},
		"off the page":   {"/api/documents/" + docID + "/text-boxes", offPage},
	}
	for name, c := range cases {
		t.Run(name, func(t *testing.T) {
			resp, body := ana.send(http.MethodPost, c.path, c.body)
			if resp.StatusCode != http.StatusUnprocessableEntity || errorCode(body) != "validation_failed" {
				t.Errorf("= %d %v, want 422 validation_failed", resp.StatusCode, body)
			}
		})
	}
}

func TestDrawingsArePrivate(t *testing.T) {
	srv, _, _ := newStackWithFiles(t)
	ana := signedUp(t, newBrowser(t, srv), "ana@example.com")
	marko := signedUp(t, newBrowser(t, srv), "marko@example.com")
	docID := ana.uploadPDF(ana.createLesson("Ana's"), "a.pdf")
	strokeID := ana.create("/api/documents/"+docID+"/strokes", "stroke", stroke(1, "red"))
	textBoxID := ana.create("/api/documents/"+docID+"/text-boxes", "textBox", textBox(1, "Ana's"))

	checks := []struct{ method, path string }{
		{http.MethodGet, "/api/documents/" + docID + "/strokes"},
		{http.MethodPost, "/api/documents/" + docID + "/strokes"},
		{http.MethodDelete, "/api/strokes/" + strokeID},
		{http.MethodGet, "/api/documents/" + docID + "/text-boxes"},
		{http.MethodPost, "/api/documents/" + docID + "/text-boxes"},
		{http.MethodPut, "/api/text-boxes/" + textBoxID},
		{http.MethodDelete, "/api/text-boxes/" + textBoxID},
	}
	bodies := map[string]map[string]any{
		"/api/documents/" + docID + "/strokes":    stroke(1, "red"),
		"/api/documents/" + docID + "/text-boxes": textBox(1, "mine"),
		"/api/text-boxes/" + textBoxID:            {"x": 0, "y": 0, "text": "mine", "color": "red", "fontSize": 14},
	}
	for _, c := range checks {
		t.Run(c.method+" "+c.path, func(t *testing.T) {
			var body any
			if c.method != http.MethodGet && c.method != http.MethodDelete {
				body = bodies[c.path]
			}
			if resp, out := marko.send(c.method, c.path, body); resp.StatusCode != http.StatusNotFound {
				t.Errorf("= %d %v, want 404", resp.StatusCode, out)
			}
		})
	}

	_, body := ana.send(http.MethodGet, "/api/documents/"+docID+"/strokes", nil)
	if n := len(body["strokes"].([]any)); n != 1 {
		t.Errorf("Ana has %d strokes after Marko's attempts, want 1", n)
	}
	_, body = ana.send(http.MethodGet, "/api/documents/"+docID+"/text-boxes", nil)
	boxes := body["textBoxes"].([]any)
	if len(boxes) != 1 || boxes[0].(map[string]any)["text"] != "Ana's" {
		t.Errorf("Ana's text boxes after Marko's attempts = %v", boxes)
	}
}

func TestDeletingADocumentDeletesItsDrawings(t *testing.T) {
	srv, pool, _ := newStackWithFiles(t)
	ana := signedUp(t, newBrowser(t, srv), "ana@example.com")
	lessonID := ana.createLesson("Lekcija 1")
	doc1 := ana.uploadPDF(lessonID, "a.pdf")
	doc2 := ana.uploadPDF(lessonID, "b.pdf")
	for _, doc := range []string{doc1, doc2} {
		ana.create("/api/documents/"+doc+"/strokes", "stroke", stroke(1, "red"))
		ana.create("/api/documents/"+doc+"/text-boxes", "textBox", textBox(1, "x"))
	}

	count := func(table string) int {
		var n int
		if err := pool.QueryRow(context.Background(), `SELECT count(*) FROM `+table).Scan(&n); err != nil {
			t.Fatal(err)
		}
		return n
	}
	if resp, _ := ana.send(http.MethodDelete, "/api/documents/"+doc1, nil); resp.StatusCode != http.StatusNoContent {
		t.Fatalf("delete document = %d", resp.StatusCode)
	}
	if s, b := count("ink_strokes"), count("text_boxes"); s != 1 || b != 1 {
		t.Errorf("%d strokes and %d text boxes after deleting a document, want 1 and 1", s, b)
	}
}
