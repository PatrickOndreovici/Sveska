package httpapi

import (
	"context"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"

	"learning-serbian-platform/backend/internal/learning"
)

// DrawingService is what the HTTP layer needs from learning.
type DrawingService interface {
	CreateStroke(ctx context.Context, in learning.NewInkStroke) (learning.InkStroke, error)
	ListStrokes(ctx context.Context, documentID string) ([]learning.InkStroke, error)
	DeleteStroke(ctx context.Context, id string) error
	CreateTextBox(ctx context.Context, in learning.NewTextBox) (learning.TextBox, error)
	ListTextBoxes(ctx context.Context, documentID string) ([]learning.TextBox, error)
	UpdateTextBox(ctx context.Context, id string, in learning.TextBoxFields) (learning.TextBox, error)
	DeleteTextBox(ctx context.Context, id string) error
}

// pointBody is one point of a stroke in 0–1 page coordinates.
type pointBody struct {
	X float64 `json:"x"`
	Y float64 `json:"y"`
}

// strokeRequest is the body of POST /api/documents/{id}/strokes.
type strokeRequest struct {
	PageNumber int         `json:"pageNumber"`
	Color      string      `json:"color"`
	Points     []pointBody `json:"points"`
}

type strokeResponse struct {
	ID         string      `json:"id"`
	DocumentID string      `json:"documentId"`
	PageNumber int         `json:"pageNumber"`
	Color      string      `json:"color"`
	Points     []pointBody `json:"points"`
	CreatedAt  time.Time   `json:"createdAt"`
}

type strokeEnvelope struct {
	Stroke strokeResponse `json:"stroke"`
}

type strokeListResponse struct {
	Strokes []strokeResponse `json:"strokes"`
}

// textBoxFieldsBody is the body of PUT /api/text-boxes/{id}.
type textBoxFieldsBody struct {
	X     float64 `json:"x"`
	Y     float64 `json:"y"`
	Text  string  `json:"text"`
	Color string  `json:"color"`
}

// newTextBoxRequest is the body of POST /api/documents/{id}/text-boxes.
type newTextBoxRequest struct {
	PageNumber int `json:"pageNumber"`
	textBoxFieldsBody
}

type textBoxResponse struct {
	ID         string `json:"id"`
	DocumentID string `json:"documentId"`
	PageNumber int    `json:"pageNumber"`
	textBoxFieldsBody
	CreatedAt time.Time `json:"createdAt"`
	UpdatedAt time.Time `json:"updatedAt"`
}

type textBoxEnvelope struct {
	TextBox textBoxResponse `json:"textBox"`
}

type textBoxListResponse struct {
	TextBoxes []textBoxResponse `json:"textBoxes"`
}

func toStrokeResponse(s learning.InkStroke) strokeResponse {
	points := make([]pointBody, len(s.Points))
	for i, p := range s.Points {
		points[i] = pointBody{X: p.X, Y: p.Y}
	}
	return strokeResponse{
		ID:         s.ID,
		DocumentID: s.DocumentID,
		PageNumber: s.PageNumber,
		Color:      string(s.Color),
		Points:     points,
		CreatedAt:  s.CreatedAt.UTC(),
	}
}

func toTextBoxResponse(b learning.TextBox) textBoxResponse {
	return textBoxResponse{
		ID:         b.ID,
		DocumentID: b.DocumentID,
		PageNumber: b.PageNumber,
		textBoxFieldsBody: textBoxFieldsBody{
			X:     b.X,
			Y:     b.Y,
			Text:  b.Text,
			Color: string(b.Color),
		},
		CreatedAt: b.CreatedAt.UTC(),
		UpdatedAt: b.UpdatedAt.UTC(),
	}
}

func (f textBoxFieldsBody) toFields() learning.TextBoxFields {
	return learning.TextBoxFields{X: f.X, Y: f.Y, Text: f.Text, Color: learning.InkColor(f.Color)}
}

func (a *api) listStrokes(w http.ResponseWriter, r *http.Request) {
	strokes, err := a.drawings.ListStrokes(r.Context(), chi.URLParam(r, "id"))
	if err != nil {
		writeServiceError(w, r, err)
		return
	}
	// make, not a nil slice, so an empty list encodes as [] rather than null.
	out := make([]strokeResponse, len(strokes))
	for i, s := range strokes {
		out[i] = toStrokeResponse(s)
	}
	writeJSON(w, http.StatusOK, strokeListResponse{Strokes: out})
}

func (a *api) createStroke(w http.ResponseWriter, r *http.Request) {
	var req strokeRequest
	if err := decodeJSON(w, r, &req); err != nil {
		writeServiceError(w, r, err)
		return
	}
	points := make([]learning.Point, len(req.Points))
	for i, p := range req.Points {
		points[i] = learning.Point{X: p.X, Y: p.Y}
	}
	stroke, err := a.drawings.CreateStroke(r.Context(), learning.NewInkStroke{
		DocumentID: chi.URLParam(r, "id"),
		PageNumber: req.PageNumber,
		Color:      learning.InkColor(req.Color),
		Points:     points,
	})
	if err != nil {
		writeServiceError(w, r, err)
		return
	}
	writeJSON(w, http.StatusCreated, strokeEnvelope{Stroke: toStrokeResponse(stroke)})
}

func (a *api) deleteStroke(w http.ResponseWriter, r *http.Request) {
	if err := a.drawings.DeleteStroke(r.Context(), chi.URLParam(r, "id")); err != nil {
		writeServiceError(w, r, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (a *api) listTextBoxes(w http.ResponseWriter, r *http.Request) {
	boxes, err := a.drawings.ListTextBoxes(r.Context(), chi.URLParam(r, "id"))
	if err != nil {
		writeServiceError(w, r, err)
		return
	}
	out := make([]textBoxResponse, len(boxes))
	for i, b := range boxes {
		out[i] = toTextBoxResponse(b)
	}
	writeJSON(w, http.StatusOK, textBoxListResponse{TextBoxes: out})
}

func (a *api) createTextBox(w http.ResponseWriter, r *http.Request) {
	var req newTextBoxRequest
	if err := decodeJSON(w, r, &req); err != nil {
		writeServiceError(w, r, err)
		return
	}
	box, err := a.drawings.CreateTextBox(r.Context(), learning.NewTextBox{
		DocumentID:    chi.URLParam(r, "id"),
		PageNumber:    req.PageNumber,
		TextBoxFields: req.toFields(),
	})
	if err != nil {
		writeServiceError(w, r, err)
		return
	}
	writeJSON(w, http.StatusCreated, textBoxEnvelope{TextBox: toTextBoxResponse(box)})
}

func (a *api) updateTextBox(w http.ResponseWriter, r *http.Request) {
	var req textBoxFieldsBody
	if err := decodeJSON(w, r, &req); err != nil {
		writeServiceError(w, r, err)
		return
	}
	box, err := a.drawings.UpdateTextBox(r.Context(), chi.URLParam(r, "id"), req.toFields())
	if err != nil {
		writeServiceError(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, textBoxEnvelope{TextBox: toTextBoxResponse(box)})
}

func (a *api) deleteTextBox(w http.ResponseWriter, r *http.Request) {
	if err := a.drawings.DeleteTextBox(r.Context(), chi.URLParam(r, "id")); err != nil {
		writeServiceError(w, r, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
