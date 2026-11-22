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

// InkColor is the colour of a stroke or of typed text.
type InkColor string

const (
	InkBlack InkColor = "black"
	InkBlue  InkColor = "blue"
	InkRed   InkColor = "red"
)

// Point is one point of a stroke in 0–1 page coordinates, like Rect. The
// JSON tags are also its stored form in ink_strokes.points.
type Point struct {
	X float64 `json:"x"`
	Y float64 `json:"y"`
}

// InkStroke is a line drawn by hand on one page of a PDF: the points of
// one movement of the pen, in order.
type InkStroke struct {
	ID         string
	DocumentID string
	PageNumber int
	Color      InkColor
	Points     []Point
	CreatedAt  time.Time
}

type NewInkStroke struct {
	DocumentID string
	PageNumber int
	Color      InkColor
	Points     []Point
}

// TextBoxFields are what can change about a text box: its text, colour,
// size and top-left corner (X, Y) in 0–1 page coordinates.
type TextBoxFields struct {
	X     float64
	Y     float64
	Text  string
	Color InkColor
	// FontSize is in CSS pixels at 100% zoom, from MinFontSize to MaxFontSize.
	FontSize int
}

// TextBox is text typed onto one page of a PDF, such as the answer to a
// fill-in exercise.
type TextBox struct {
	ID         string
	DocumentID string
	PageNumber int
	TextBoxFields
	CreatedAt time.Time
	UpdatedAt time.Time
}

type NewTextBox struct {
	DocumentID string
	PageNumber int
	TextBoxFields
}

const (
	// About a minute of slow scribbling; the viewer starts a new stroke
	// before a long one reaches it.
	maxStrokePoints = 1000
	maxTextBoxLen   = 1000
)

// The sizes text can be typed in: small enough for a crowded table, large
// enough for a heading. The database checks the same range.
const (
	MinFontSize = 8
	MaxFontSize = 48
)

// DrawingStore is the subset of generated queries this service needs.
// *store.Queries satisfies it; tests substitute a fake.
type DrawingStore interface {
	GetDocument(ctx context.Context, arg store.GetDocumentParams) (store.GetDocumentRow, error)
	CreateInkStroke(ctx context.Context, arg store.CreateInkStrokeParams) (store.InkStroke, error)
	ListDocumentInkStrokes(ctx context.Context, arg store.ListDocumentInkStrokesParams) ([]store.InkStroke, error)
	DeleteInkStroke(ctx context.Context, arg store.DeleteInkStrokeParams) (int64, error)
	CreateTextBox(ctx context.Context, arg store.CreateTextBoxParams) (store.TextBox, error)
	ListDocumentTextBoxes(ctx context.Context, arg store.ListDocumentTextBoxesParams) ([]store.TextBox, error)
	UpdateTextBox(ctx context.Context, arg store.UpdateTextBoxParams) (store.TextBox, error)
	DeleteTextBox(ctx context.Context, arg store.DeleteTextBoxParams) (int64, error)
}

// DrawingService lets a student write and draw on their own PDFs: strokes
// drawn by hand, such as circles around answers, and text typed onto the
// page, such as the answers to fill-in exercises. As with annotations, the
// owner always comes from the actor.
type DrawingService struct {
	store DrawingStore
}

func NewDrawingService(s DrawingStore) *DrawingService {
	return &DrawingService{store: s}
}

var (
	errStrokeNotFound  = apperr.New(apperr.NotFound, "stroke_not_found", "This drawing does not exist.")
	errTextBoxNotFound = apperr.New(apperr.NotFound, "text_box_not_found", "This text does not exist.")
)

// CreateStroke saves a stroke on one of the actor's documents.
func (s *DrawingService) CreateStroke(ctx context.Context, in NewInkStroke) (InkStroke, error) {
	actor, err := access.RequireStudent(ctx)
	if err != nil {
		return InkStroke{}, err
	}
	if !isUUID(in.DocumentID) {
		return InkStroke{}, errDocumentNotFound
	}
	fields := map[string]string{}
	addProblem(fields, "pageNumber", validatePageNumber(in.PageNumber))
	addProblem(fields, "color", validateInkColor(in.Color))
	addProblem(fields, "points", validatePoints(in.Points))
	if len(fields) > 0 {
		return InkStroke{}, apperr.Validation(fields)
	}
	points, err := json.Marshal(in.Points)
	if err != nil {
		return InkStroke{}, fmt.Errorf("encode points: %w", err)
	}

	row, err := s.store.CreateInkStroke(ctx, store.CreateInkStrokeParams{
		OwnerID:    actor.User.ID,
		DocumentID: in.DocumentID,
		PageNumber: int32(in.PageNumber),
		Color:      store.InkColor(in.Color),
		Points:     points,
	})
	if isForeignKeyViolation(err, "ink_strokes_document_fkey") {
		return InkStroke{}, errDocumentNotFound
	}
	if err != nil {
		return InkStroke{}, fmt.Errorf("create ink stroke: %w", err)
	}
	return strokeFromRow(row)
}

// ListStrokes returns a document's strokes page by page, in the order they
// were drawn.
func (s *DrawingService) ListStrokes(ctx context.Context, documentID string) ([]InkStroke, error) {
	actor, err := access.RequireStudent(ctx)
	if err != nil {
		return nil, err
	}
	if err := requireOwnDocument(ctx, s.store, actor.User.ID, documentID); err != nil {
		return nil, err
	}
	rows, err := s.store.ListDocumentInkStrokes(ctx, store.ListDocumentInkStrokesParams{
		DocumentID: documentID,
		OwnerID:    actor.User.ID,
	})
	if err != nil {
		return nil, fmt.Errorf("list ink strokes of %s: %w", documentID, err)
	}
	strokes := make([]InkStroke, 0, len(rows))
	for _, row := range rows {
		stroke, err := strokeFromRow(row)
		if err != nil {
			return nil, err
		}
		strokes = append(strokes, stroke)
	}
	return strokes, nil
}

// DeleteStroke erases a stroke. A stroke never changes otherwise.
func (s *DrawingService) DeleteStroke(ctx context.Context, id string) error {
	actor, err := access.RequireStudent(ctx)
	if err != nil {
		return err
	}
	if !isUUID(id) {
		return errStrokeNotFound
	}
	n, err := s.store.DeleteInkStroke(ctx, store.DeleteInkStrokeParams{ID: id, OwnerID: actor.User.ID})
	if err != nil {
		return fmt.Errorf("delete ink stroke %s: %w", id, err)
	}
	if n == 0 {
		return errStrokeNotFound
	}
	return nil
}

// CreateTextBox saves text typed onto one of the actor's documents.
func (s *DrawingService) CreateTextBox(ctx context.Context, in NewTextBox) (TextBox, error) {
	actor, err := access.RequireStudent(ctx)
	if err != nil {
		return TextBox{}, err
	}
	if !isUUID(in.DocumentID) {
		return TextBox{}, errDocumentNotFound
	}
	in.Text = strings.TrimSpace(in.Text)
	fields := validateTextBox(in.TextBoxFields)
	addProblem(fields, "pageNumber", validatePageNumber(in.PageNumber))
	if len(fields) > 0 {
		return TextBox{}, apperr.Validation(fields)
	}

	row, err := s.store.CreateTextBox(ctx, store.CreateTextBoxParams{
		OwnerID:    actor.User.ID,
		DocumentID: in.DocumentID,
		PageNumber: int32(in.PageNumber),
		X:          in.X,
		Y:          in.Y,
		Body:       in.Text,
		Color:      store.InkColor(in.Color),
		FontSize:   int32(in.FontSize),
	})
	if isForeignKeyViolation(err, "text_boxes_document_fkey") {
		return TextBox{}, errDocumentNotFound
	}
	if err != nil {
		return TextBox{}, fmt.Errorf("create text box: %w", err)
	}
	return textBoxFromRow(row), nil
}

// ListTextBoxes returns a document's text boxes page by page, in the order
// they were added.
func (s *DrawingService) ListTextBoxes(ctx context.Context, documentID string) ([]TextBox, error) {
	actor, err := access.RequireStudent(ctx)
	if err != nil {
		return nil, err
	}
	if err := requireOwnDocument(ctx, s.store, actor.User.ID, documentID); err != nil {
		return nil, err
	}
	rows, err := s.store.ListDocumentTextBoxes(ctx, store.ListDocumentTextBoxesParams{
		DocumentID: documentID,
		OwnerID:    actor.User.ID,
	})
	if err != nil {
		return nil, fmt.Errorf("list text boxes of %s: %w", documentID, err)
	}
	boxes := make([]TextBox, 0, len(rows))
	for _, row := range rows {
		boxes = append(boxes, textBoxFromRow(row))
	}
	return boxes, nil
}

// UpdateTextBox replaces a text box's text, colour and place on its page.
func (s *DrawingService) UpdateTextBox(ctx context.Context, id string, in TextBoxFields) (TextBox, error) {
	actor, err := access.RequireStudent(ctx)
	if err != nil {
		return TextBox{}, err
	}
	if !isUUID(id) {
		return TextBox{}, errTextBoxNotFound
	}
	in.Text = strings.TrimSpace(in.Text)
	if fields := validateTextBox(in); len(fields) > 0 {
		return TextBox{}, apperr.Validation(fields)
	}

	row, err := s.store.UpdateTextBox(ctx, store.UpdateTextBoxParams{
		ID:       id,
		OwnerID:  actor.User.ID,
		X:        in.X,
		Y:        in.Y,
		Body:     in.Text,
		Color:    store.InkColor(in.Color),
		FontSize: int32(in.FontSize),
	})
	if errors.Is(err, pgx.ErrNoRows) {
		return TextBox{}, errTextBoxNotFound
	}
	if err != nil {
		return TextBox{}, fmt.Errorf("update text box %s: %w", id, err)
	}
	return textBoxFromRow(row), nil
}

// DeleteTextBox removes a text box.
func (s *DrawingService) DeleteTextBox(ctx context.Context, id string) error {
	actor, err := access.RequireStudent(ctx)
	if err != nil {
		return err
	}
	if !isUUID(id) {
		return errTextBoxNotFound
	}
	n, err := s.store.DeleteTextBox(ctx, store.DeleteTextBoxParams{ID: id, OwnerID: actor.User.ID})
	if err != nil {
		return fmt.Errorf("delete text box %s: %w", id, err)
	}
	if n == 0 {
		return errTextBoxNotFound
	}
	return nil
}

func validateInkColor(c InkColor) string {
	switch c {
	case InkBlack, InkBlue, InkRed:
		return ""
	}
	return "Choose black, blue or red."
}

func validatePoints(points []Point) string {
	if len(points) == 0 || len(points) > maxStrokePoints {
		return fmt.Sprintf("Send between 1 and %d points.", maxStrokePoints)
	}
	for _, p := range points {
		// Written so that NaN, which fails every comparison, is rejected too.
		if !(p.X >= 0 && p.X <= 1 && p.Y >= 0 && p.Y <= 1) {
			return "Each point must lie within the page (0 to 1)."
		}
	}
	return ""
}

func validateTextBox(in TextBoxFields) map[string]string {
	fields := map[string]string{}
	// The corner must be on the page; the text may run on past it.
	if !(in.X >= 0 && in.X < 1) {
		fields["x"] = "Place the text on the page (0 to 1)."
	}
	if !(in.Y >= 0 && in.Y < 1) {
		fields["y"] = "Place the text on the page (0 to 1)."
	}
	if in.Text == "" {
		fields["text"] = "Type some text."
	} else {
		addProblem(fields, "text", validateLength(in.Text, maxTextBoxLen))
	}
	addProblem(fields, "color", validateInkColor(in.Color))
	if in.FontSize < MinFontSize || in.FontSize > MaxFontSize {
		fields["fontSize"] = fmt.Sprintf("Choose a size from %d to %d pixels.", MinFontSize, MaxFontSize)
	}
	return fields
}

func strokeFromRow(row store.InkStroke) (InkStroke, error) {
	var points []Point
	if err := json.Unmarshal(row.Points, &points); err != nil {
		return InkStroke{}, fmt.Errorf("decode points of ink stroke %s: %w", row.ID, err)
	}
	return InkStroke{
		ID:         row.ID,
		DocumentID: row.DocumentID,
		PageNumber: int(row.PageNumber),
		Color:      InkColor(row.Color),
		Points:     points,
		CreatedAt:  row.CreatedAt,
	}, nil
}

func textBoxFromRow(row store.TextBox) TextBox {
	return TextBox{
		ID:         row.ID,
		DocumentID: row.DocumentID,
		PageNumber: int(row.PageNumber),
		TextBoxFields: TextBoxFields{
			X:        row.X,
			Y:        row.Y,
			Text:     row.Body,
			Color:    InkColor(row.Color),
			FontSize: int(row.FontSize),
		},
		CreatedAt: row.CreatedAt,
		UpdatedAt: row.UpdatedAt,
	}
}
