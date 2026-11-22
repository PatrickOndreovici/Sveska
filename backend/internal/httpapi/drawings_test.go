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

const (
	strokeID  = "019a0000-0000-7000-8000-0000000000c1"
	textBoxID = "019a0000-0000-7000-8000-0000000000c2"
)

var (
	testStroke = learning.InkStroke{
		ID: strokeID, DocumentID: documentID, PageNumber: 2, Color: learning.InkRed,
		Points: []learning.Point{{X: 0.2, Y: 0.3}, {X: 0.25, Y: 0.28}}, CreatedAt: testTime,
	}
	testTextBox = learning.TextBox{
		ID: textBoxID, DocumentID: documentID, PageNumber: 1,
		TextBoxFields: learning.TextBoxFields{X: 0.4, Y: 0.5, Text: "kuća", Color: learning.InkBlue, FontSize: 14},
		CreatedAt:     testTime, UpdatedAt: testTime,
	}
)

// fakeDrawings records what the handlers pass in and answers with
// testStroke and testTextBox, or err when set.
type fakeDrawings struct {
	err        error
	calls      int
	lastID     string
	lastStroke learning.NewInkStroke
	lastBox    learning.NewTextBox
	lastFields learning.TextBoxFields
}

func (f *fakeDrawings) CreateStroke(_ context.Context, in learning.NewInkStroke) (learning.InkStroke, error) {
	f.calls++
	f.lastStroke = in
	return testStroke, f.err
}

func (f *fakeDrawings) ListStrokes(_ context.Context, documentID string) ([]learning.InkStroke, error) {
	f.calls++
	f.lastID = documentID
	return []learning.InkStroke{testStroke}, f.err
}

func (f *fakeDrawings) DeleteStroke(_ context.Context, id string) error {
	f.calls++
	f.lastID = id
	return f.err
}

func (f *fakeDrawings) CreateTextBox(_ context.Context, in learning.NewTextBox) (learning.TextBox, error) {
	f.calls++
	f.lastBox = in
	return testTextBox, f.err
}

func (f *fakeDrawings) ListTextBoxes(_ context.Context, documentID string) ([]learning.TextBox, error) {
	f.calls++
	f.lastID = documentID
	return []learning.TextBox{testTextBox}, f.err
}

func (f *fakeDrawings) UpdateTextBox(_ context.Context, id string, in learning.TextBoxFields) (learning.TextBox, error) {
	f.calls++
	f.lastID, f.lastFields = id, in
	return testTextBox, f.err
}

func (f *fakeDrawings) DeleteTextBox(_ context.Context, id string) error {
	f.calls++
	f.lastID = id
	return f.err
}

func newDrawingsRouter(d *fakeDrawings) http.Handler {
	return NewRouter(Dependencies{DB: fakePinger{}, Auth: &fakeAuth{}, Drawings: d, AppOrigin: testOrigin, CookieSecure: true})
}

const (
	strokeBody  = `{"pageNumber":2,"color":"red","points":[{"x":0.2,"y":0.3},{"x":0.25,"y":0.28}]}`
	textBoxBody = `{"pageNumber":1,"x":0.4,"y":0.5,"text":"kuća","color":"blue","fontSize":14}`
)

func TestDrawingRoutesRequireSession(t *testing.T) {
	routes := []struct{ method, path, body string }{
		{http.MethodGet, "/api/documents/" + documentID + "/strokes", ""},
		{http.MethodPost, "/api/documents/" + documentID + "/strokes", strokeBody},
		{http.MethodDelete, "/api/strokes/" + strokeID, ""},
		{http.MethodGet, "/api/documents/" + documentID + "/text-boxes", ""},
		{http.MethodPost, "/api/documents/" + documentID + "/text-boxes", textBoxBody},
		{http.MethodPut, "/api/text-boxes/" + textBoxID, `{"x":0.4,"y":0.5,"text":"kuća","color":"blue","fontSize":14}`},
		{http.MethodDelete, "/api/text-boxes/" + textBoxID, ""},
	}
	for _, rt := range routes {
		t.Run(rt.method+" "+rt.path, func(t *testing.T) {
			d := &fakeDrawings{}
			rec := do(t, newDrawingsRouter(d), rt.method, rt.path, requestOpts{body: rt.body, origin: testOrigin})
			if rec.Code != http.StatusUnauthorized || d.calls != 0 {
				t.Errorf("status = %d, service calls = %d; want 401 and 0", rec.Code, d.calls)
			}
		})
	}
}

func TestCreateStroke(t *testing.T) {
	d := &fakeDrawings{}
	rec := do(t, newDrawingsRouter(d), http.MethodPost, "/api/documents/"+documentID+"/strokes", loggedIn(strokeBody))

	if rec.Code != http.StatusCreated {
		t.Fatalf("status = %d, body = %s", rec.Code, rec.Body)
	}
	in := d.lastStroke
	if in.DocumentID != documentID || in.PageNumber != 2 || in.Color != learning.InkRed || !slices.Equal(in.Points, testStroke.Points) {
		t.Errorf("service received %+v", in)
	}

	// The JSON keys are the API contract the frontend reads.
	var body map[string]map[string]any
	if err := json.NewDecoder(rec.Body).Decode(&body); err != nil {
		t.Fatal(err)
	}
	got := body["stroke"]
	for key, value := range map[string]any{
		"id": strokeID, "documentId": documentID, "pageNumber": 2.0, "color": "red", "createdAt": "2026-10-23T12:00:00Z",
	} {
		if got[key] != value {
			t.Errorf("stroke[%q] = %v, want %v", key, got[key], value)
		}
	}
	points, _ := got["points"].([]any)
	if len(points) != 2 || points[1].(map[string]any)["x"] != 0.25 {
		t.Errorf("points = %v", got["points"])
	}
}

func TestListAndDeleteStrokes(t *testing.T) {
	d := &fakeDrawings{}
	h := newDrawingsRouter(d)

	rec := do(t, h, http.MethodGet, "/api/documents/"+documentID+"/strokes", loggedIn(""))
	var body struct {
		Strokes []map[string]any `json:"strokes"`
	}
	if err := json.NewDecoder(rec.Body).Decode(&body); err != nil {
		t.Fatal(err)
	}
	if rec.Code != http.StatusOK || d.lastID != documentID || len(body.Strokes) != 1 || body.Strokes[0]["id"] != strokeID {
		t.Errorf("list = %d %v, document %q", rec.Code, body.Strokes, d.lastID)
	}

	rec = do(t, h, http.MethodDelete, "/api/strokes/"+strokeID, loggedIn(""))
	if rec.Code != http.StatusNoContent || d.lastID != strokeID {
		t.Errorf("delete = %d, service got %q", rec.Code, d.lastID)
	}
}

func TestCreateTextBox(t *testing.T) {
	d := &fakeDrawings{}
	rec := do(t, newDrawingsRouter(d), http.MethodPost, "/api/documents/"+documentID+"/text-boxes", loggedIn(textBoxBody))

	if rec.Code != http.StatusCreated {
		t.Fatalf("status = %d, body = %s", rec.Code, rec.Body)
	}
	want := learning.NewTextBox{DocumentID: documentID, PageNumber: 1, TextBoxFields: testTextBox.TextBoxFields}
	if d.lastBox != want {
		t.Errorf("service received %+v, want %+v", d.lastBox, want)
	}

	var body map[string]map[string]any
	if err := json.NewDecoder(rec.Body).Decode(&body); err != nil {
		t.Fatal(err)
	}
	got := body["textBox"]
	for key, value := range map[string]any{
		"id": textBoxID, "documentId": documentID, "pageNumber": 1.0, "x": 0.4, "y": 0.5,
		"text": "kuća", "color": "blue", "fontSize": 14.0, "createdAt": "2026-10-23T12:00:00Z", "updatedAt": "2026-10-23T12:00:00Z",
	} {
		if got[key] != value {
			t.Errorf("textBox[%q] = %v, want %v", key, got[key], value)
		}
	}
}

func TestUpdateListAndDeleteTextBoxes(t *testing.T) {
	d := &fakeDrawings{}
	h := newDrawingsRouter(d)

	rec := do(t, h, http.MethodPut, "/api/text-boxes/"+textBoxID, loggedIn(`{"x":0.1,"y":0.2,"text":"kuće","color":"black","fontSize":20}`))
	want := learning.TextBoxFields{X: 0.1, Y: 0.2, Text: "kuće", Color: learning.InkBlack, FontSize: 20}
	if rec.Code != http.StatusOK || d.lastID != textBoxID || d.lastFields != want {
		t.Errorf("update = %d, service got %q %+v", rec.Code, d.lastID, d.lastFields)
	}

	rec = do(t, h, http.MethodGet, "/api/documents/"+documentID+"/text-boxes", loggedIn(""))
	var body struct {
		TextBoxes []map[string]any `json:"textBoxes"`
	}
	if err := json.NewDecoder(rec.Body).Decode(&body); err != nil {
		t.Fatal(err)
	}
	if rec.Code != http.StatusOK || len(body.TextBoxes) != 1 || body.TextBoxes[0]["text"] != "kuća" {
		t.Errorf("list = %d %v", rec.Code, body.TextBoxes)
	}

	rec = do(t, h, http.MethodDelete, "/api/text-boxes/"+textBoxID, loggedIn(""))
	if rec.Code != http.StatusNoContent || d.lastID != textBoxID {
		t.Errorf("delete = %d, service got %q", rec.Code, d.lastID)
	}
}

func TestDrawingBodiesRejectUnknownFields(t *testing.T) {
	cases := []struct{ method, path, body string }{
		{http.MethodPost, "/api/documents/" + documentID + "/strokes", `{"pageNumber":2,"color":"red","points":[],"width":3}`},
		{http.MethodPost, "/api/documents/" + documentID + "/text-boxes", `{"pageNumber":1,"x":0,"y":0,"text":"x","color":"red","size":12}`},
		// The page of a text box cannot change.
		{http.MethodPut, "/api/text-boxes/" + textBoxID, `{"pageNumber":2,"x":0,"y":0,"text":"x","color":"red"}`},
	}
	for _, c := range cases {
		t.Run(c.method+" "+c.path, func(t *testing.T) {
			d := &fakeDrawings{}
			rec := do(t, newDrawingsRouter(d), c.method, c.path, loggedIn(c.body))
			if rec.Code != http.StatusUnprocessableEntity || decodeError(t, rec).Code != "invalid_json" || d.calls != 0 {
				t.Errorf("status = %d, calls = %d; want 422 invalid_json and 0", rec.Code, d.calls)
			}
		})
	}
}

func TestDrawingErrorsReachTheClient(t *testing.T) {
	cases := []struct {
		err    error
		status int
	}{
		{apperr.New(apperr.NotFound, "stroke_not_found", "This drawing does not exist."), http.StatusNotFound},
		{apperr.New(apperr.Forbidden, "students_only", "This is only available to student accounts."), http.StatusForbidden},
		{apperr.Validation(map[string]string{"points": "Each point must lie within the page (0 to 1)."}), http.StatusUnprocessableEntity},
	}
	for _, c := range cases {
		d := &fakeDrawings{err: c.err}
		rec := do(t, newDrawingsRouter(d), http.MethodPost, "/api/documents/"+documentID+"/strokes", loggedIn(strokeBody))
		if rec.Code != c.status {
			t.Errorf("%v: status = %d, want %d", c.err, rec.Code, c.status)
		}
	}
}
