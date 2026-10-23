package httpapi

import (
	"context"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"

	"learning-serbian-platform/backend/internal/learning"
)

// AnnotationService is what the HTTP layer needs from learning.
type AnnotationService interface {
	Create(ctx context.Context, in learning.NewAnnotation) (learning.Annotation, error)
	List(ctx context.Context, documentID string) ([]learning.Annotation, error)
	UpdateNote(ctx context.Context, id, note string) (learning.Annotation, error)
	Delete(ctx context.Context, id string) error
}

// rectBody is one rectangle of a highlight in 0–1 page coordinates.
type rectBody struct {
	X      float64 `json:"x"`
	Y      float64 `json:"y"`
	Width  float64 `json:"width"`
	Height float64 `json:"height"`
}

// annotationRequest is the body of POST /api/documents/{id}/annotations.
type annotationRequest struct {
	PageNumber int        `json:"pageNumber"`
	Rects      []rectBody `json:"rects"`
	Quote      string     `json:"quote"`
	Note       string     `json:"note"`
}

// noteRequest is the body of PUT /api/annotations/{id}: only the note changes.
type noteRequest struct {
	Note string `json:"note"`
}

type annotationResponse struct {
	ID         string     `json:"id"`
	DocumentID string     `json:"documentId"`
	PageNumber int        `json:"pageNumber"`
	Rects      []rectBody `json:"rects"`
	Quote      string     `json:"quote"`
	Note       string     `json:"note"`
	CreatedAt  time.Time  `json:"createdAt"`
	UpdatedAt  time.Time  `json:"updatedAt"`
}

type annotationEnvelope struct {
	Annotation annotationResponse `json:"annotation"`
}

type annotationListResponse struct {
	Annotations []annotationResponse `json:"annotations"`
}

func toAnnotationResponse(a learning.Annotation) annotationResponse {
	rects := make([]rectBody, len(a.Rects))
	for i, r := range a.Rects {
		rects[i] = rectBody{X: r.X, Y: r.Y, Width: r.Width, Height: r.Height}
	}
	return annotationResponse{
		ID:         a.ID,
		DocumentID: a.DocumentID,
		PageNumber: a.PageNumber,
		Rects:      rects,
		Quote:      a.Quote,
		Note:       a.Note,
		CreatedAt:  a.CreatedAt.UTC(),
		UpdatedAt:  a.UpdatedAt.UTC(),
	}
}

func (a *api) listAnnotations(w http.ResponseWriter, r *http.Request) {
	annotations, err := a.annotations.List(r.Context(), chi.URLParam(r, "id"))
	if err != nil {
		writeServiceError(w, r, err)
		return
	}
	// make, not a nil slice, so an empty list encodes as [] rather than null.
	out := make([]annotationResponse, len(annotations))
	for i, an := range annotations {
		out[i] = toAnnotationResponse(an)
	}
	writeJSON(w, http.StatusOK, annotationListResponse{Annotations: out})
}

func (a *api) createAnnotation(w http.ResponseWriter, r *http.Request) {
	var req annotationRequest
	if err := decodeJSON(w, r, &req); err != nil {
		writeServiceError(w, r, err)
		return
	}
	rects := make([]learning.Rect, len(req.Rects))
	for i, rc := range req.Rects {
		rects[i] = learning.Rect{X: rc.X, Y: rc.Y, Width: rc.Width, Height: rc.Height}
	}
	annotation, err := a.annotations.Create(r.Context(), learning.NewAnnotation{
		DocumentID: chi.URLParam(r, "id"),
		PageNumber: req.PageNumber,
		Rects:      rects,
		Quote:      req.Quote,
		Note:       req.Note,
	})
	if err != nil {
		writeServiceError(w, r, err)
		return
	}
	writeJSON(w, http.StatusCreated, annotationEnvelope{Annotation: toAnnotationResponse(annotation)})
}

func (a *api) updateAnnotation(w http.ResponseWriter, r *http.Request) {
	var req noteRequest
	if err := decodeJSON(w, r, &req); err != nil {
		writeServiceError(w, r, err)
		return
	}
	annotation, err := a.annotations.UpdateNote(r.Context(), chi.URLParam(r, "id"), req.Note)
	if err != nil {
		writeServiceError(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, annotationEnvelope{Annotation: toAnnotationResponse(annotation)})
}

func (a *api) deleteAnnotation(w http.ResponseWriter, r *http.Request) {
	if err := a.annotations.Delete(r.Context(), chi.URLParam(r, "id")); err != nil {
		writeServiceError(w, r, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
