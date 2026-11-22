package learning

import (
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

// fakeAnnotationStore keeps documents (only their owners) and annotations
// in memory. It mirrors the owner filter and the composite foreign key
// annotations(document_id, owner_id) -> documents(id, owner_id).
type fakeAnnotationStore struct {
	documentOwners map[string]string // document id -> owner id
	annotations    map[string]store.Annotation
	nextID         int
	now            time.Time
	calls          int
}

const (
	anasDocID   = "00000000-0000-7000-8000-0000000000d1"
	markosDocID = "00000000-0000-7000-8000-0000000000d2"
)

func newAnnotationTest() (*AnnotationService, *fakeAnnotationStore) {
	fs := &fakeAnnotationStore{
		documentOwners: map[string]string{anasDocID: anaID, markosDocID: markoID},
		annotations:    map[string]store.Annotation{},
		now:            time.Date(2026, 10, 23, 12, 0, 0, 0, time.UTC),
	}
	return NewAnnotationService(fs), fs
}

func (f *fakeAnnotationStore) GetDocument(_ context.Context, arg store.GetDocumentParams) (store.GetDocumentRow, error) {
	f.calls++
	if f.documentOwners[arg.ID] != arg.OwnerID {
		return store.GetDocumentRow{}, pgx.ErrNoRows
	}
	return store.GetDocumentRow{ID: arg.ID, OwnerID: arg.OwnerID}, nil
}

func (f *fakeAnnotationStore) CreateAnnotation(_ context.Context, arg store.CreateAnnotationParams) (store.Annotation, error) {
	f.calls++
	if f.documentOwners[arg.DocumentID] != arg.OwnerID {
		return store.Annotation{}, &pgconn.PgError{Code: "23503", ConstraintName: "annotations_document_fkey"}
	}
	f.nextID++
	row := store.Annotation{
		ID: fmt.Sprintf("00000000-0000-7000-8000-%012d", f.nextID), OwnerID: arg.OwnerID, DocumentID: arg.DocumentID,
		PageNumber: arg.PageNumber, Rects: arg.Rects, Quote: arg.Quote, Note: arg.Note,
		CreatedAt: f.now.Add(time.Duration(f.nextID) * time.Second), UpdatedAt: f.now,
	}
	f.annotations[row.ID] = row
	return row, nil
}

func (f *fakeAnnotationStore) ListDocumentAnnotations(_ context.Context, arg store.ListDocumentAnnotationsParams) ([]store.Annotation, error) {
	f.calls++
	var rows []store.Annotation
	for _, a := range f.annotations {
		if a.DocumentID == arg.DocumentID && a.OwnerID == arg.OwnerID {
			rows = append(rows, a)
		}
	}
	slices.SortFunc(rows, func(a, b store.Annotation) int {
		if a.PageNumber != b.PageNumber {
			return int(a.PageNumber - b.PageNumber)
		}
		return a.CreatedAt.Compare(b.CreatedAt)
	})
	return rows, nil
}

func (f *fakeAnnotationStore) UpdateAnnotationNote(_ context.Context, arg store.UpdateAnnotationNoteParams) (store.Annotation, error) {
	f.calls++
	row, ok := f.annotations[arg.ID]
	if !ok || row.OwnerID != arg.OwnerID {
		return store.Annotation{}, pgx.ErrNoRows
	}
	row.Note = arg.Note
	row.UpdatedAt = f.now.Add(time.Hour)
	f.annotations[arg.ID] = row
	return row, nil
}

func (f *fakeAnnotationStore) DeleteAnnotation(_ context.Context, arg store.DeleteAnnotationParams) (int64, error) {
	f.calls++
	row, ok := f.annotations[arg.ID]
	if !ok || row.OwnerID != arg.OwnerID {
		return 0, nil
	}
	delete(f.annotations, arg.ID)
	return 1, nil
}

// line is a valid one-line highlight on page 2 of Ana's document.
func line(quote string) NewAnnotation {
	return NewAnnotation{
		DocumentID: anasDocID,
		PageNumber: 2,
		Rects:      []Rect{{X: 0.1, Y: 0.2, Width: 0.5, Height: 0.02}},
		Quote:      quote,
	}
}

func mustAnnotate(t *testing.T, svc *AnnotationService, in NewAnnotation) Annotation {
	t.Helper()
	a, err := svc.Create(actorCtx(anaID), in)
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	return a
}

func TestCreateAnnotationSavesTheHighlight(t *testing.T) {
	svc, fs := newAnnotationTest()
	in := line("  Dobar dan  ")
	in.Note = " greeting\n"
	in.Rects = append(in.Rects, Rect{X: 0, Y: 0.22, Width: 1, Height: 0.02})

	a := mustAnnotate(t, svc, in)

	if a.Quote != "Dobar dan" || a.Note != "greeting" {
		t.Errorf("quote, note = %q, %q, want trimmed", a.Quote, a.Note)
	}
	if a.DocumentID != anasDocID || a.PageNumber != 2 || !slices.Equal(a.Rects, in.Rects) {
		t.Errorf("annotation = %+v", a)
	}
	saved := fs.annotations[a.ID]
	if saved.OwnerID != anaID {
		t.Errorf("owner = %q, want %q", saved.OwnerID, anaID)
	}
	if want := `[{"x":0.1,"y":0.2,"width":0.5,"height":0.02},{"x":0,"y":0.22,"width":1,"height":0.02}]`; string(saved.Rects) != want {
		t.Errorf("stored rects = %s, want %s", saved.Rects, want)
	}
}

func TestCreateAnnotationOnlyOnTheUsersOwnDocument(t *testing.T) {
	for name, docID := range map[string]string{"someone else's": markosDocID, "missing": missingID} {
		t.Run(name, func(t *testing.T) {
			svc, fs := newAnnotationTest()
			in := line("x")
			in.DocumentID = docID
			_, err := svc.Create(actorCtx(anaID), in)
			requireAppErr(t, err, apperr.NotFound, "document_not_found")
			if len(fs.annotations) != 0 {
				t.Errorf("%d annotations saved, want 0", len(fs.annotations))
			}
		})
	}
	t.Run("malformed", func(t *testing.T) {
		svc, fs := newAnnotationTest()
		in := line("x")
		in.DocumentID = "1; DROP TABLE"
		_, err := svc.Create(actorCtx(anaID), in)
		requireAppErr(t, err, apperr.NotFound, "document_not_found")
		if fs.calls != 0 {
			t.Errorf("store called %d times, want 0", fs.calls)
		}
	})
}

func TestCreateAnnotationRejectsInvalidInput(t *testing.T) {
	tooMany := make([]Rect, maxRects+1)
	for i := range tooMany {
		tooMany[i] = Rect{X: 0, Y: 0, Width: 0.1, Height: 0.001}
	}
	cases := []struct {
		name  string
		edit  func(*NewAnnotation)
		field string
	}{
		{"page zero", func(in *NewAnnotation) { in.PageNumber = 0 }, "pageNumber"},
		{"page past the limit", func(in *NewAnnotation) { in.PageNumber = maxPageNumber + 1 }, "pageNumber"},
		{"no rectangles", func(in *NewAnnotation) { in.Rects = nil }, "rects"},
		{"too many rectangles", func(in *NewAnnotation) { in.Rects = tooMany }, "rects"},
		{"negative x", func(in *NewAnnotation) { in.Rects[0].X = -0.1 }, "rects"},
		{"past the right edge", func(in *NewAnnotation) { in.Rects[0].X = 0.6 }, "rects"},
		{"past the bottom edge", func(in *NewAnnotation) { in.Rects[0].Y = 0.99 }, "rects"},
		{"zero width", func(in *NewAnnotation) { in.Rects[0].Width = 0 }, "rects"},
		{"NaN", func(in *NewAnnotation) { in.Rects[0].Height = math.NaN() }, "rects"},
		{"blank quote", func(in *NewAnnotation) { in.Quote = "   " }, "quote"},
		{"long quote", func(in *NewAnnotation) { in.Quote = strings.Repeat("ć", maxQuoteLen+1) }, "quote"},
		{"long note", func(in *NewAnnotation) { in.Note = strings.Repeat("ž", maxNoteLen+1) }, "note"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			svc, fs := newAnnotationTest()
			in := line("Dobar dan")
			c.edit(&in)
			_, err := svc.Create(actorCtx(anaID), in)
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

func TestCreateAnnotationAcceptsTheEdgesOfThePage(t *testing.T) {
	svc, _ := newAnnotationTest()
	in := line(strings.Repeat("ć", maxQuoteLen))
	in.Rects = []Rect{{X: 0, Y: 0, Width: 1, Height: 1}, {X: 0.7, Y: 0.9, Width: 0.3, Height: 0.1}}
	in.Note = strings.Repeat("ž", maxNoteLen)
	mustAnnotate(t, svc, in)
}

func TestListAnnotationsInReadingOrder(t *testing.T) {
	svc, _ := newAnnotationTest()
	later := line("page two, later")
	first := line("page one")
	first.PageNumber = 1
	earlier := line("page two, earlier")
	a := mustAnnotate(t, svc, earlier)
	b := mustAnnotate(t, svc, later)
	c := mustAnnotate(t, svc, first)

	got, err := svc.List(actorCtx(anaID), anasDocID)
	if err != nil {
		t.Fatal(err)
	}
	ids := []string{}
	for _, x := range got {
		ids = append(ids, x.ID)
	}
	if want := []string{c.ID, a.ID, b.ID}; !slices.Equal(ids, want) {
		t.Errorf("order = %v, want %v", ids, want)
	}
}

func TestListAnnotationsOfADocumentWithoutAnyIsEmptyNotNil(t *testing.T) {
	svc, _ := newAnnotationTest()
	got, err := svc.List(actorCtx(anaID), anasDocID)
	if err != nil || got == nil || len(got) != 0 {
		t.Errorf("List = %v, %v; want an empty, non-nil slice", got, err)
	}
}

func TestListAnnotationsOfSomeoneElsesDocumentIsNotFound(t *testing.T) {
	svc, _ := newAnnotationTest()
	for _, id := range []string{markosDocID, missingID, "nope"} {
		_, err := svc.List(actorCtx(anaID), id)
		requireAppErr(t, err, apperr.NotFound, "document_not_found")
	}
}

func TestUpdateAnnotationNote(t *testing.T) {
	svc, _ := newAnnotationTest()
	a := mustAnnotate(t, svc, line("Dobar dan"))

	updated, err := svc.UpdateNote(actorCtx(anaID), a.ID, "  a greeting ")
	if err != nil {
		t.Fatal(err)
	}
	if updated.Note != "a greeting" || updated.Quote != "Dobar dan" {
		t.Errorf("updated = %+v", updated)
	}

	_, err = svc.UpdateNote(actorCtx(anaID), a.ID, strings.Repeat("ž", maxNoteLen+1))
	appErr := requireAppErr(t, err, apperr.Invalid, "validation_failed")
	if appErr.Fields["note"] == "" {
		t.Errorf("fields = %v, want note", appErr.Fields)
	}
}

func TestDeleteAnnotation(t *testing.T) {
	svc, fs := newAnnotationTest()
	a := mustAnnotate(t, svc, line("Dobar dan"))

	if err := svc.Delete(actorCtx(anaID), a.ID); err != nil {
		t.Fatal(err)
	}
	if len(fs.annotations) != 0 {
		t.Errorf("%d annotations left, want 0", len(fs.annotations))
	}
	requireAppErr(t, svc.Delete(actorCtx(anaID), a.ID), apperr.NotFound, "annotation_not_found")
}

func TestAnnotationsOfOthersAreNotFound(t *testing.T) {
	svc, fs := newAnnotationTest()
	a := mustAnnotate(t, svc, line("Ana's"))
	marko := actorCtx(markoID)

	_, err := svc.UpdateNote(marko, a.ID, "mine")
	requireAppErr(t, err, apperr.NotFound, "annotation_not_found")
	requireAppErr(t, svc.Delete(marko, a.ID), apperr.NotFound, "annotation_not_found")
	_, err = svc.UpdateNote(actorCtx(anaID), "nope", "x")
	requireAppErr(t, err, apperr.NotFound, "annotation_not_found")
	requireAppErr(t, svc.Delete(actorCtx(anaID), "nope"), apperr.NotFound, "annotation_not_found")
	if fs.annotations[a.ID].Note != "" {
		t.Error("Marko changed Ana's note")
	}
}

func TestAnnotationsRequireAStudent(t *testing.T) {
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
			svc, fs := newAnnotationTest()
			checks := map[string]error{}
			_, checks["create"] = svc.Create(c.ctx, line("x"))
			_, checks["list"] = svc.List(c.ctx, anasDocID)
			_, checks["update"] = svc.UpdateNote(c.ctx, missingID, "x")
			checks["delete"] = svc.Delete(c.ctx, missingID)
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
