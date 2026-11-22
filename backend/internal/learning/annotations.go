package learning

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"

	"learning-serbian-platform/backend/internal/access"
	"learning-serbian-platform/backend/internal/apperr"
	"learning-serbian-platform/backend/internal/store"
)

// Rect is one rectangle of a highlight in 0–1 page coordinates: X and Y
// from the page's top-left corner and the size, all as fractions of the
// page's width and height, so a highlight fits the page at any zoom. The
// JSON tags are also its stored form in annotations.rects.
type Rect struct {
	X      float64 `json:"x"`
	Y      float64 `json:"y"`
	Width  float64 `json:"width"`
	Height float64 `json:"height"`
}

// Annotation is a highlight on one page of a PDF with an optional note.
type Annotation struct {
	ID         string
	DocumentID string
	PageNumber int
	Rects      []Rect
	Quote      string // the highlighted text
	Note       string
	CreatedAt  time.Time
	UpdatedAt  time.Time
}

// NewAnnotation is a highlight as the viewer reports a text selection.
type NewAnnotation struct {
	DocumentID string
	PageNumber int
	Rects      []Rect
	Quote      string
	Note       string
}

const (
	maxRects      = 200 // a selection spanning a whole dense page stays well under this
	maxQuoteLen   = 2000
	maxNoteLen    = 5000
	maxPageNumber = 100_000 // keeps page_number well inside int32
	// rectSlack absorbs floating-point error when a rectangle ends exactly at the page edge.
	rectSlack = 1e-9
)

// AnnotationStore is the subset of generated queries this service needs.
// *store.Queries satisfies it; tests substitute a fake.
type AnnotationStore interface {
	GetDocument(ctx context.Context, arg store.GetDocumentParams) (store.GetDocumentRow, error)
	CreateAnnotation(ctx context.Context, arg store.CreateAnnotationParams) (store.Annotation, error)
	ListDocumentAnnotations(ctx context.Context, arg store.ListDocumentAnnotationsParams) ([]store.Annotation, error)
	UpdateAnnotationNote(ctx context.Context, arg store.UpdateAnnotationNoteParams) (store.Annotation, error)
	DeleteAnnotation(ctx context.Context, arg store.DeleteAnnotationParams) (int64, error)
}

// AnnotationService lets a student highlight text in their own PDFs and
// write notes on the highlights. As with lessons, the owner always comes
// from the actor, so someone else's document or annotation is not found.
type AnnotationService struct {
	store AnnotationStore
}

func NewAnnotationService(s AnnotationStore) *AnnotationService {
	return &AnnotationService{store: s}
}

var errAnnotationNotFound = apperr.New(apperr.NotFound, "annotation_not_found", "This highlight does not exist.")

// Create saves a highlight on one of the actor's documents.
func (s *AnnotationService) Create(ctx context.Context, in NewAnnotation) (Annotation, error) {
	actor, err := access.RequireStudent(ctx)
	if err != nil {
		return Annotation{}, err
	}
	if !isUUID(in.DocumentID) {
		return Annotation{}, errDocumentNotFound
	}
	in.Quote = strings.TrimSpace(in.Quote)
	in.Note = strings.TrimSpace(in.Note)
	if fields := validateAnnotation(in); len(fields) > 0 {
		return Annotation{}, apperr.Validation(fields)
	}
	rects, err := json.Marshal(in.Rects)
	if err != nil {
		return Annotation{}, fmt.Errorf("encode rects: %w", err)
	}

	row, err := s.store.CreateAnnotation(ctx, store.CreateAnnotationParams{
		OwnerID:    actor.User.ID,
		DocumentID: in.DocumentID,
		PageNumber: int32(in.PageNumber),
		Rects:      rects,
		Quote:      in.Quote,
		Note:       in.Note,
	})
	if isForeignKeyViolation(err, "annotations_document_fkey") {
		return Annotation{}, errDocumentNotFound
	}
	if err != nil {
		return Annotation{}, fmt.Errorf("create annotation: %w", err)
	}
	return annotationFromRow(row)
}

// List returns a document's annotations in reading order: by page, then
// oldest first. A missing document is document_not_found rather than an
// empty list.
func (s *AnnotationService) List(ctx context.Context, documentID string) ([]Annotation, error) {
	actor, err := access.RequireStudent(ctx)
	if err != nil {
		return nil, err
	}
	if err := requireOwnDocument(ctx, s.store, actor.User.ID, documentID); err != nil {
		return nil, err
	}

	rows, err := s.store.ListDocumentAnnotations(ctx, store.ListDocumentAnnotationsParams{
		DocumentID: documentID,
		OwnerID:    actor.User.ID,
	})
	if err != nil {
		return nil, fmt.Errorf("list annotations of %s: %w", documentID, err)
	}
	annotations := make([]Annotation, 0, len(rows))
	for _, row := range rows {
		a, err := annotationFromRow(row)
		if err != nil {
			return nil, err
		}
		annotations = append(annotations, a)
	}
	return annotations, nil
}

// UpdateNote replaces an annotation's note; an empty note leaves a plain
// highlight. The highlighted text itself cannot change.
func (s *AnnotationService) UpdateNote(ctx context.Context, id, note string) (Annotation, error) {
	actor, err := access.RequireStudent(ctx)
	if err != nil {
		return Annotation{}, err
	}
	if !isUUID(id) {
		return Annotation{}, errAnnotationNotFound
	}
	note = strings.TrimSpace(note)
	if problem := validateLength(note, maxNoteLen); problem != "" {
		return Annotation{}, apperr.Validation(map[string]string{"note": problem})
	}

	row, err := s.store.UpdateAnnotationNote(ctx, store.UpdateAnnotationNoteParams{ID: id, OwnerID: actor.User.ID, Note: note})
	if errors.Is(err, pgx.ErrNoRows) {
		return Annotation{}, errAnnotationNotFound
	}
	if err != nil {
		return Annotation{}, fmt.Errorf("update annotation %s: %w", id, err)
	}
	return annotationFromRow(row)
}

// Delete removes an annotation. Words saved from it stay in the
// vocabulary; the database clears their link to it.
func (s *AnnotationService) Delete(ctx context.Context, id string) error {
	actor, err := access.RequireStudent(ctx)
	if err != nil {
		return err
	}
	if !isUUID(id) {
		return errAnnotationNotFound
	}
	n, err := s.store.DeleteAnnotation(ctx, store.DeleteAnnotationParams{ID: id, OwnerID: actor.User.ID})
	if err != nil {
		return fmt.Errorf("delete annotation %s: %w", id, err)
	}
	if n == 0 {
		return errAnnotationNotFound
	}
	return nil
}

// documentGetter finds one of a user's documents.
type documentGetter interface {
	GetDocument(ctx context.Context, arg store.GetDocumentParams) (store.GetDocumentRow, error)
}

// requireOwnDocument is document_not_found unless the document exists and
// belongs to ownerID, so listing what is on a missing document is an error
// rather than an empty list.
func requireOwnDocument(ctx context.Context, s documentGetter, ownerID, documentID string) error {
	if !isUUID(documentID) {
		return errDocumentNotFound
	}
	_, err := s.GetDocument(ctx, store.GetDocumentParams{ID: documentID, OwnerID: ownerID})
	if errors.Is(err, pgx.ErrNoRows) {
		return errDocumentNotFound
	}
	if err != nil {
		return fmt.Errorf("get document %s: %w", documentID, err)
	}
	return nil
}

func validateAnnotation(in NewAnnotation) map[string]string {
	fields := map[string]string{}
	addProblem(fields, "pageNumber", validatePageNumber(in.PageNumber))
	addProblem(fields, "rects", validateRects(in.Rects))
	if in.Quote == "" {
		fields["quote"] = "Select some text to highlight."
	} else {
		addProblem(fields, "quote", validateLength(in.Quote, maxQuoteLen))
	}
	addProblem(fields, "note", validateLength(in.Note, maxNoteLen))
	return fields
}

func validateRects(rects []Rect) string {
	if len(rects) == 0 || len(rects) > maxRects {
		return fmt.Sprintf("Send between 1 and %d rectangles.", maxRects)
	}
	for _, r := range rects {
		// Written so that NaN, which fails every comparison, is rejected too.
		inside := r.X >= 0 && r.Y >= 0 && r.Width > 0 && r.Height > 0 &&
			r.X+r.Width <= 1+rectSlack && r.Y+r.Height <= 1+rectSlack
		if !inside {
			return "Each rectangle must lie within the page (0 to 1)."
		}
	}
	return ""
}

func annotationFromRow(row store.Annotation) (Annotation, error) {
	var rects []Rect
	if err := json.Unmarshal(row.Rects, &rects); err != nil {
		return Annotation{}, fmt.Errorf("decode rects of annotation %s: %w", row.ID, err)
	}
	return Annotation{
		ID:         row.ID,
		DocumentID: row.DocumentID,
		PageNumber: int(row.PageNumber),
		Rects:      rects,
		Quote:      row.Quote,
		Note:       row.Note,
		CreatedAt:  row.CreatedAt,
		UpdatedAt:  row.UpdatedAt,
	}, nil
}
