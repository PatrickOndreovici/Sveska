package learning

import (
	"cmp"
	"context"
	"fmt"
	"math"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"

	"learning-serbian-platform/backend/internal/apperr"
	"learning-serbian-platform/backend/internal/store"
)

// fakeDrawingStore keeps documents (only their owners), strokes and text
// boxes in memory. It mirrors the owner filter and the composite foreign
// keys to documents(id, owner_id).
type fakeDrawingStore struct {
	documentOwners map[string]string // document id -> owner id
	strokes        map[string]store.InkStroke
	textBoxes      map[string]store.TextBox
	nextID         int
	now            time.Time
	calls          int
}

func newDrawingTest() (*DrawingService, *fakeDrawingStore) {
	fs := &fakeDrawingStore{
		documentOwners: map[string]string{anasDocID: anaID, markosDocID: markoID},
		strokes:        map[string]store.InkStroke{},
		textBoxes:      map[string]store.TextBox{},
		now:            time.Date(2026, 10, 23, 12, 0, 0, 0, time.UTC),
	}
	return NewDrawingService(fs), fs
}

// newID counts up, so ids sort in creation order like UUIDv7s.
func (f *fakeDrawingStore) newID() string {
	f.nextID++
	return fmt.Sprintf("00000000-0000-7000-8000-%012d", f.nextID)
}

func (f *fakeDrawingStore) GetDocument(_ context.Context, arg store.GetDocumentParams) (store.GetDocumentRow, error) {
	f.calls++
	if f.documentOwners[arg.ID] != arg.OwnerID {
		return store.GetDocumentRow{}, pgx.ErrNoRows
	}
	return store.GetDocumentRow{ID: arg.ID, OwnerID: arg.OwnerID}, nil
}

func (f *fakeDrawingStore) CreateInkStroke(_ context.Context, arg store.CreateInkStrokeParams) (store.InkStroke, error) {
	f.calls++
	if f.documentOwners[arg.DocumentID] != arg.OwnerID {
		return store.InkStroke{}, &pgconn.PgError{Code: "23503", ConstraintName: "ink_strokes_document_fkey"}
	}
	row := store.InkStroke{
		ID: f.newID(), OwnerID: arg.OwnerID, DocumentID: arg.DocumentID, PageNumber: arg.PageNumber,
		Color: arg.Color, Points: arg.Points, CreatedAt: f.now,
	}
	f.strokes[row.ID] = row
	return row, nil
}

func (f *fakeDrawingStore) ListDocumentInkStrokes(_ context.Context, arg store.ListDocumentInkStrokesParams) ([]store.InkStroke, error) {
	f.calls++
	var rows []store.InkStroke
	for _, s := range f.strokes {
		if s.DocumentID == arg.DocumentID && s.OwnerID == arg.OwnerID {
			rows = append(rows, s)
		}
	}
	slices.SortFunc(rows, func(a, b store.InkStroke) int {
		return cmp.Or(cmp.Compare(a.PageNumber, b.PageNumber), cmp.Compare(a.ID, b.ID))
	})
	return rows, nil
}

func (f *fakeDrawingStore) DeleteInkStroke(_ context.Context, arg store.DeleteInkStrokeParams) (int64, error) {
	f.calls++
	if s, ok := f.strokes[arg.ID]; !ok || s.OwnerID != arg.OwnerID {
		return 0, nil
	}
	delete(f.strokes, arg.ID)
	return 1, nil
}

func (f *fakeDrawingStore) CreateTextBox(_ context.Context, arg store.CreateTextBoxParams) (store.TextBox, error) {
	f.calls++
	if f.documentOwners[arg.DocumentID] != arg.OwnerID {
		return store.TextBox{}, &pgconn.PgError{Code: "23503", ConstraintName: "text_boxes_document_fkey"}
	}
	row := store.TextBox{
		ID: f.newID(), OwnerID: arg.OwnerID, DocumentID: arg.DocumentID, PageNumber: arg.PageNumber,
		X: arg.X, Y: arg.Y, Body: arg.Body, Color: arg.Color, CreatedAt: f.now, UpdatedAt: f.now,
	}
	f.textBoxes[row.ID] = row
	return row, nil
}

func (f *fakeDrawingStore) ListDocumentTextBoxes(_ context.Context, arg store.ListDocumentTextBoxesParams) ([]store.TextBox, error) {
	f.calls++
	var rows []store.TextBox
	for _, b := range f.textBoxes {
		if b.DocumentID == arg.DocumentID && b.OwnerID == arg.OwnerID {
			rows = append(rows, b)
		}
	}
	slices.SortFunc(rows, func(a, b store.TextBox) int {
		return cmp.Or(cmp.Compare(a.PageNumber, b.PageNumber), cmp.Compare(a.ID, b.ID))
	})
	return rows, nil
}

func (f *fakeDrawingStore) UpdateTextBox(_ context.Context, arg store.UpdateTextBoxParams) (store.TextBox, error) {
	f.calls++
	row, ok := f.textBoxes[arg.ID]
	if !ok || row.OwnerID != arg.OwnerID {
		return store.TextBox{}, pgx.ErrNoRows
	}
	row.X, row.Y, row.Body, row.Color = arg.X, arg.Y, arg.Body, arg.Color
	row.UpdatedAt = f.now.Add(time.Hour)
	f.textBoxes[arg.ID] = row
	return row, nil
}

func (f *fakeDrawingStore) DeleteTextBox(_ context.Context, arg store.DeleteTextBoxParams) (int64, error) {
	f.calls++
	if b, ok := f.textBoxes[arg.ID]; !ok || b.OwnerID != arg.OwnerID {
		return 0, nil
	}
	delete(f.textBoxes, arg.ID)
	return 1, nil
}

// circle is a valid red stroke on the given page of Ana's document.
func circle(page int) NewInkStroke {
	return NewInkStroke{
		DocumentID: anasDocID,
		PageNumber: page,
		Color:      InkRed,
		Points:     []Point{{X: 0.2, Y: 0.3}, {X: 0.25, Y: 0.28}, {X: 0.3, Y: 0.3}},
	}
}

// answer is valid blue text on page 1 of Ana's document.
func answer(text string) NewTextBox {
	return NewTextBox{
		DocumentID:    anasDocID,
		PageNumber:    1,
		TextBoxFields: TextBoxFields{X: 0.4, Y: 0.5, Text: text, Color: InkBlue},
	}
}

func mustDraw(t *testing.T, svc *DrawingService, in NewInkStroke) InkStroke {
	t.Helper()
	s, err := svc.CreateStroke(actorCtx(anaID), in)
	if err != nil {
		t.Fatalf("CreateStroke: %v", err)
	}
	return s
}

func mustWrite(t *testing.T, svc *DrawingService, in NewTextBox) TextBox {
	t.Helper()
	b, err := svc.CreateTextBox(actorCtx(anaID), in)
	if err != nil {
		t.Fatalf("CreateTextBox: %v", err)
	}
	return b
}

func TestCreateStrokeSavesThePoints(t *testing.T) {
	svc, fs := newDrawingTest()
	in := circle(2)

	s := mustDraw(t, svc, in)

	if s.DocumentID != anasDocID || s.PageNumber != 2 || s.Color != InkRed || !slices.Equal(s.Points, in.Points) {
		t.Errorf("stroke = %+v", s)
	}
	saved := fs.strokes[s.ID]
	if saved.OwnerID != anaID {
		t.Errorf("owner = %q, want %q", saved.OwnerID, anaID)
	}
	if want := `[{"x":0.2,"y":0.3},{"x":0.25,"y":0.28},{"x":0.3,"y":0.3}]`; string(saved.Points) != want {
		t.Errorf("stored points = %s, want %s", saved.Points, want)
	}
}

func TestCreateStrokeAcceptsADotAndTheEdgesOfThePage(t *testing.T) {
	svc, _ := newDrawingTest()
	dot := circle(1)
	dot.Points = []Point{{X: 1, Y: 1}}
	mustDraw(t, svc, dot)

	long := circle(1)
	long.Points = make([]Point, maxStrokePoints)
	mustDraw(t, svc, long)
}

func TestCreateStrokeRejectsInvalidInput(t *testing.T) {
	cases := []struct {
		name  string
		edit  func(*NewInkStroke)
		field string
	}{
		{"page zero", func(in *NewInkStroke) { in.PageNumber = 0 }, "pageNumber"},
		{"page past the limit", func(in *NewInkStroke) { in.PageNumber = maxPageNumber + 1 }, "pageNumber"},
		{"no colour", func(in *NewInkStroke) { in.Color = "" }, "color"},
		{"unknown colour", func(in *NewInkStroke) { in.Color = "pink" }, "color"},
		{"no points", func(in *NewInkStroke) { in.Points = nil }, "points"},
		{"too many points", func(in *NewInkStroke) { in.Points = make([]Point, maxStrokePoints+1) }, "points"},
		{"left of the page", func(in *NewInkStroke) { in.Points[1].X = -0.01 }, "points"},
		{"below the page", func(in *NewInkStroke) { in.Points[2].Y = 1.01 }, "points"},
		{"NaN", func(in *NewInkStroke) { in.Points[0].X = math.NaN() }, "points"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			svc, fs := newDrawingTest()
			in := circle(1)
			c.edit(&in)
			_, err := svc.CreateStroke(actorCtx(anaID), in)
			appErr := requireAppErr(t, err, apperr.Invalid, "validation_failed")
			if appErr.Fields[c.field] == "" || len(appErr.Fields) != 1 {
				t.Errorf("fields = %v, want only %s", appErr.Fields, c.field)
			}
			if fs.calls != 0 {
				t.Errorf("store called %d times, want 0", fs.calls)
			}
		})
	}
}

func TestDrawingOnlyOnTheUsersOwnDocument(t *testing.T) {
	for name, docID := range map[string]string{"someone else's": markosDocID, "missing": missingID, "malformed": "1; DROP"} {
		t.Run(name, func(t *testing.T) {
			svc, fs := newDrawingTest()
			stroke := circle(1)
			stroke.DocumentID = docID
			_, err := svc.CreateStroke(actorCtx(anaID), stroke)
			requireAppErr(t, err, apperr.NotFound, "document_not_found")

			text := answer("kuća")
			text.DocumentID = docID
			_, err = svc.CreateTextBox(actorCtx(anaID), text)
			requireAppErr(t, err, apperr.NotFound, "document_not_found")

			_, err = svc.ListStrokes(actorCtx(anaID), docID)
			requireAppErr(t, err, apperr.NotFound, "document_not_found")
			_, err = svc.ListTextBoxes(actorCtx(anaID), docID)
			requireAppErr(t, err, apperr.NotFound, "document_not_found")

			if len(fs.strokes)+len(fs.textBoxes) != 0 {
				t.Error("something was saved on a document that is not Ana's")
			}
		})
	}
}

func TestListStrokesPageByPageInDrawingOrder(t *testing.T) {
	svc, _ := newDrawingTest()
	a := mustDraw(t, svc, circle(2))
	b := mustDraw(t, svc, circle(1))
	c := mustDraw(t, svc, circle(2))

	got, err := svc.ListStrokes(actorCtx(anaID), anasDocID)
	if err != nil {
		t.Fatal(err)
	}
	ids := []string{}
	for _, s := range got {
		ids = append(ids, s.ID)
	}
	if want := []string{b.ID, a.ID, c.ID}; !slices.Equal(ids, want) {
		t.Errorf("order = %v, want %v", ids, want)
	}
}

func TestListsOfADocumentWithoutDrawingsAreEmptyNotNil(t *testing.T) {
	svc, _ := newDrawingTest()
	strokes, err := svc.ListStrokes(actorCtx(anaID), anasDocID)
	if err != nil || strokes == nil || len(strokes) != 0 {
		t.Errorf("ListStrokes = %v, %v; want an empty, non-nil slice", strokes, err)
	}
	boxes, err := svc.ListTextBoxes(actorCtx(anaID), anasDocID)
	if err != nil || boxes == nil || len(boxes) != 0 {
		t.Errorf("ListTextBoxes = %v, %v; want an empty, non-nil slice", boxes, err)
	}
}

func TestDeleteStroke(t *testing.T) {
	svc, fs := newDrawingTest()
	s := mustDraw(t, svc, circle(1))

	requireAppErr(t, svc.DeleteStroke(actorCtx(markoID), s.ID), apperr.NotFound, "stroke_not_found")
	if err := svc.DeleteStroke(actorCtx(anaID), s.ID); err != nil {
		t.Fatal(err)
	}
	if len(fs.strokes) != 0 {
		t.Errorf("%d strokes left, want 0", len(fs.strokes))
	}
	requireAppErr(t, svc.DeleteStroke(actorCtx(anaID), s.ID), apperr.NotFound, "stroke_not_found")
	requireAppErr(t, svc.DeleteStroke(actorCtx(anaID), "nope"), apperr.NotFound, "stroke_not_found")
}

func TestCreateTextBoxTrimsTheText(t *testing.T) {
	svc, fs := newDrawingTest()
	b := mustWrite(t, svc, answer("  kuća\nkuće \n"))

	if b.Text != "kuća\nkuće" || b.X != 0.4 || b.Y != 0.5 || b.Color != InkBlue || b.PageNumber != 1 {
		t.Errorf("text box = %+v", b)
	}
	if saved := fs.textBoxes[b.ID]; saved.OwnerID != anaID || saved.Body != "kuća\nkuće" {
		t.Errorf("saved = %+v", saved)
	}
}

func TestTextBoxesRejectInvalidInput(t *testing.T) {
	cases := []struct {
		name  string
		edit  func(*TextBoxFields)
		field string
	}{
		{"left of the page", func(in *TextBoxFields) { in.X = -0.01 }, "x"},
		{"at the right edge", func(in *TextBoxFields) { in.X = 1 }, "x"},
		{"above the page", func(in *TextBoxFields) { in.Y = -0.01 }, "y"},
		{"at the bottom edge", func(in *TextBoxFields) { in.Y = 1 }, "y"},
		{"NaN", func(in *TextBoxFields) { in.Y = math.NaN() }, "y"},
		{"blank text", func(in *TextBoxFields) { in.Text = " \n " }, "text"},
		{"long text", func(in *TextBoxFields) { in.Text = strings.Repeat("ć", maxTextBoxLen+1) }, "text"},
		{"unknown colour", func(in *TextBoxFields) { in.Color = "pink" }, "color"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			svc, fs := newDrawingTest()
			existing := mustWrite(t, svc, answer("kuća"))
			calls := fs.calls

			in := answer("kuća")
			c.edit(&in.TextBoxFields)
			_, err := svc.CreateTextBox(actorCtx(anaID), in)
			appErr := requireAppErr(t, err, apperr.Invalid, "validation_failed")
			if appErr.Fields[c.field] == "" || len(appErr.Fields) != 1 {
				t.Errorf("create: fields = %v, want only %s", appErr.Fields, c.field)
			}

			_, err = svc.UpdateTextBox(actorCtx(anaID), existing.ID, in.TextBoxFields)
			appErr = requireAppErr(t, err, apperr.Invalid, "validation_failed")
			if appErr.Fields[c.field] == "" || len(appErr.Fields) != 1 {
				t.Errorf("update: fields = %v, want only %s", appErr.Fields, c.field)
			}
			if fs.calls != calls {
				t.Errorf("store called %d more times, want 0", fs.calls-calls)
			}
		})
	}
	t.Run("page zero", func(t *testing.T) {
		svc, _ := newDrawingTest()
		in := answer("kuća")
		in.PageNumber = 0
		_, err := svc.CreateTextBox(actorCtx(anaID), in)
		appErr := requireAppErr(t, err, apperr.Invalid, "validation_failed")
		if appErr.Fields["pageNumber"] == "" {
			t.Errorf("fields = %v, want pageNumber", appErr.Fields)
		}
	})
}

func TestTextBoxesAcceptTheEdgesOfThePage(t *testing.T) {
	svc, _ := newDrawingTest()
	in := answer(strings.Repeat("ć", maxTextBoxLen))
	in.X, in.Y = 0, 0.9999
	mustWrite(t, svc, in)
}

func TestUpdateTextBoxMovesAndRewritesIt(t *testing.T) {
	svc, _ := newDrawingTest()
	b := mustWrite(t, svc, answer("kuca"))

	updated, err := svc.UpdateTextBox(actorCtx(anaID), b.ID, TextBoxFields{X: 0.1, Y: 0.2, Text: " kuća ", Color: InkBlack})
	if err != nil {
		t.Fatal(err)
	}
	want := TextBoxFields{X: 0.1, Y: 0.2, Text: "kuća", Color: InkBlack}
	if updated.TextBoxFields != want || updated.PageNumber != 1 || updated.ID != b.ID {
		t.Errorf("updated = %+v", updated)
	}
}

func TestTextBoxesOfOthersAreNotFound(t *testing.T) {
	svc, fs := newDrawingTest()
	b := mustWrite(t, svc, answer("Ana's"))
	marko := actorCtx(markoID)

	_, err := svc.UpdateTextBox(marko, b.ID, answer("mine").TextBoxFields)
	requireAppErr(t, err, apperr.NotFound, "text_box_not_found")
	requireAppErr(t, svc.DeleteTextBox(marko, b.ID), apperr.NotFound, "text_box_not_found")
	_, err = svc.UpdateTextBox(actorCtx(anaID), "nope", answer("x").TextBoxFields)
	requireAppErr(t, err, apperr.NotFound, "text_box_not_found")
	if fs.textBoxes[b.ID].Body != "Ana's" {
		t.Error("Marko changed Ana's text")
	}

	if err := svc.DeleteTextBox(actorCtx(anaID), b.ID); err != nil {
		t.Fatal(err)
	}
	requireAppErr(t, svc.DeleteTextBox(actorCtx(anaID), b.ID), apperr.NotFound, "text_box_not_found")
}

func TestDrawingsRequireAStudent(t *testing.T) {
	callers := []struct {
		name string
		ctx  context.Context
		kind apperr.Kind
		code string
	}{
		{"anonymous", context.Background(), apperr.Unauthenticated, "unauthenticated"},
		{"teacher", teacherCtx(), apperr.Forbidden, "students_only"},
	}
	for _, c := range callers {
		t.Run(c.name, func(t *testing.T) {
			svc, fs := newDrawingTest()
			checks := map[string]error{}
			_, checks["create stroke"] = svc.CreateStroke(c.ctx, circle(1))
			_, checks["list strokes"] = svc.ListStrokes(c.ctx, anasDocID)
			checks["delete stroke"] = svc.DeleteStroke(c.ctx, missingID)
			_, checks["create text box"] = svc.CreateTextBox(c.ctx, answer("x"))
			_, checks["list text boxes"] = svc.ListTextBoxes(c.ctx, anasDocID)
			_, checks["update text box"] = svc.UpdateTextBox(c.ctx, missingID, answer("x").TextBoxFields)
			checks["delete text box"] = svc.DeleteTextBox(c.ctx, missingID)
			for name, err := range checks {
				t.Run(name, func(t *testing.T) {
					requireAppErr(t, err, c.kind, c.code)
				})
			}
			if fs.calls != 0 {
				t.Errorf("store called %d times, want 0", fs.calls)
			}
		})
	}
}
