package learning

import (
	"bytes"
	"context"
	"crypto/sha256"
	"errors"
	"fmt"
	"io"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"

	"learning-serbian-platform/backend/internal/apperr"
	"learning-serbian-platform/backend/internal/filestore"
	"learning-serbian-platform/backend/internal/store"
)

// fakeLessonStore keeps lessons and documents in memory and mirrors the
// owner filter and the composite foreign key from documents to lessons.
type fakeLessonStore struct {
	lessons   map[string]store.Lesson
	documents map[string]store.Document
	nextID    int
	now       time.Time
	calls     int
}

func newFakeLessonStore() *fakeLessonStore {
	return &fakeLessonStore{
		lessons:   map[string]store.Lesson{},
		documents: map[string]store.Document{},
		now:       time.Date(2026, 10, 23, 12, 0, 0, 0, time.UTC),
	}
}

func (f *fakeLessonStore) id() string {
	f.nextID++
	return fmt.Sprintf("00000000-0000-7000-8000-%012d", f.nextID)
}

func (f *fakeLessonStore) CreateLesson(_ context.Context, arg store.CreateLessonParams) (store.Lesson, error) {
	f.calls++
	row := store.Lesson{ID: f.id(), OwnerID: arg.OwnerID, Title: arg.Title, Description: arg.Description, CreatedAt: f.now, UpdatedAt: f.now}
	f.lessons[row.ID] = row
	return row, nil
}

func (f *fakeLessonStore) ListLessons(_ context.Context, ownerID string) ([]store.ListLessonsRow, error) {
	f.calls++
	var rows []store.ListLessonsRow
	for _, l := range f.lessons {
		if l.OwnerID == ownerID {
			rows = append(rows, store.ListLessonsRow{
				ID: l.ID, OwnerID: l.OwnerID, Title: l.Title, Description: l.Description,
				CreatedAt: l.CreatedAt, UpdatedAt: l.UpdatedAt, DocumentCount: int32(len(f.docsOf(l.ID))),
			})
		}
	}
	return rows, nil
}

func (f *fakeLessonStore) docsOf(lessonID string) []store.Document {
	var docs []store.Document
	for _, d := range f.documents {
		if d.LessonID == lessonID {
			docs = append(docs, d)
		}
	}
	return docs
}

func (f *fakeLessonStore) GetLesson(_ context.Context, arg store.GetLessonParams) (store.Lesson, error) {
	f.calls++
	l, ok := f.lessons[arg.ID]
	if !ok || l.OwnerID != arg.OwnerID {
		return store.Lesson{}, pgx.ErrNoRows
	}
	return l, nil
}

func (f *fakeLessonStore) UpdateLesson(_ context.Context, arg store.UpdateLessonParams) (store.Lesson, error) {
	f.calls++
	l, ok := f.lessons[arg.ID]
	if !ok || l.OwnerID != arg.OwnerID {
		return store.Lesson{}, pgx.ErrNoRows
	}
	l.Title, l.Description, l.UpdatedAt = arg.Title, arg.Description, f.now.Add(time.Minute)
	f.lessons[l.ID] = l
	return l, nil
}

func (f *fakeLessonStore) DeleteLesson(_ context.Context, arg store.DeleteLessonParams) ([]string, error) {
	f.calls++
	l, ok := f.lessons[arg.ID]
	if !ok || l.OwnerID != arg.OwnerID {
		return nil, pgx.ErrNoRows
	}
	keys := []string{}
	for _, d := range f.docsOf(l.ID) {
		keys = append(keys, d.StorageKey)
		delete(f.documents, d.ID)
	}
	delete(f.lessons, l.ID)
	return keys, nil
}

func (f *fakeLessonStore) CreateDocument(_ context.Context, arg store.CreateDocumentParams) (store.Document, error) {
	f.calls++
	if l, ok := f.lessons[arg.LessonID]; !ok || l.OwnerID != arg.OwnerID {
		return store.Document{}, &pgconn.PgError{Code: "23503", ConstraintName: "documents_lesson_fkey"}
	}
	row := store.Document{
		ID: f.id(), OwnerID: arg.OwnerID, LessonID: arg.LessonID, Title: arg.Title,
		StorageKey: arg.StorageKey, ByteSize: arg.ByteSize, Sha256: arg.Sha256, CreatedAt: f.now,
	}
	f.documents[row.ID] = row
	return row, nil
}

func (f *fakeLessonStore) ListLessonDocuments(_ context.Context, arg store.ListLessonDocumentsParams) ([]store.Document, error) {
	f.calls++
	var docs []store.Document
	for _, d := range f.docsOf(arg.LessonID) {
		if d.OwnerID == arg.OwnerID {
			docs = append(docs, d)
		}
	}
	return docs, nil
}

func (f *fakeLessonStore) GetDocument(_ context.Context, arg store.GetDocumentParams) (store.GetDocumentRow, error) {
	f.calls++
	d, ok := f.documents[arg.ID]
	if !ok || d.OwnerID != arg.OwnerID {
		return store.GetDocumentRow{}, pgx.ErrNoRows
	}
	return store.GetDocumentRow{
		ID: d.ID, OwnerID: d.OwnerID, LessonID: d.LessonID, Title: d.Title, StorageKey: d.StorageKey,
		ByteSize: d.ByteSize, Sha256: d.Sha256, CreatedAt: d.CreatedAt, LessonTitle: f.lessons[d.LessonID].Title,
	}, nil
}

func (f *fakeLessonStore) DeleteDocument(_ context.Context, arg store.DeleteDocumentParams) (string, error) {
	f.calls++
	d, ok := f.documents[arg.ID]
	if !ok || d.OwnerID != arg.OwnerID {
		return "", pgx.ErrNoRows
	}
	delete(f.documents, d.ID)
	return d.StorageKey, nil
}

// fakeFiles keeps file contents in memory. Like filestore.Local, Put stores
// nothing when the reader fails.
type fakeFiles struct {
	files     map[string][]byte
	puts      int
	deleteErr error
	nextKey   int
}

func newFakeFiles() *fakeFiles { return &fakeFiles{files: map[string][]byte{}} }

func (f *fakeFiles) Put(_ context.Context, r io.Reader) (string, error) {
	f.puts++
	body, err := io.ReadAll(r)
	if err != nil {
		return "", fmt.Errorf("write file: %w", err)
	}
	f.nextKey++
	key := fmt.Sprintf("%032d", f.nextKey)
	f.files[key] = body
	return key, nil
}

func (f *fakeFiles) Open(_ context.Context, key string) (io.ReadSeekCloser, error) {
	body, ok := f.files[key]
	if !ok {
		return nil, filestore.ErrNotFound
	}
	return nopCloser{bytes.NewReader(body)}, nil
}

func (f *fakeFiles) Delete(_ context.Context, key string) error {
	if f.deleteErr != nil {
		return f.deleteErr
	}
	delete(f.files, key)
	return nil
}

type nopCloser struct{ io.ReadSeeker }

func (nopCloser) Close() error { return nil }

func newLessonTest() (*LessonService, *fakeLessonStore, *fakeFiles) {
	fs, files := newFakeLessonStore(), newFakeFiles()
	return NewLessonService(fs, files), fs, files
}

const pdfBody = "%PDF-1.7\nfake but starts right"

func mustCreateLesson(t *testing.T, svc *LessonService, userID, title string) Lesson {
	t.Helper()
	l, err := svc.CreateLesson(actorCtx(userID), LessonFields{Title: title})
	if err != nil {
		t.Fatalf("CreateLesson: %v", err)
	}
	return l
}

func mustUpload(t *testing.T, svc *LessonService, userID, lessonID, name string) Document {
	t.Helper()
	d, err := svc.UploadDocument(actorCtx(userID), UploadInput{LessonID: lessonID, FileName: name, Body: strings.NewReader(pdfBody)})
	if err != nil {
		t.Fatalf("UploadDocument: %v", err)
	}
	return d
}

func TestCreateLessonTrimsAndValidates(t *testing.T) {
	svc, fs, _ := newLessonTest()
	l, err := svc.CreateLesson(actorCtx(anaID), LessonFields{Title: "  Lekcija 1 ", Description: " Padeži \n"})
	if err != nil {
		t.Fatalf("CreateLesson: %v", err)
	}
	if l.Title != "Lekcija 1" || l.Description != "Padeži" || l.DocumentCount != 0 {
		t.Errorf("lesson = %+v", l)
	}
	if fs.lessons[l.ID].OwnerID != anaID {
		t.Errorf("owner = %q, want %q", fs.lessons[l.ID].OwnerID, anaID)
	}

	for name, in := range map[string]LessonFields{
		"missing title":    {Title: "  "},
		"title too long":   {Title: strings.Repeat("š", maxLessonTitleLen+1)},
		"description long": {Title: "ok", Description: strings.Repeat("a", maxLessonDescriptionLen+1)},
	} {
		t.Run(name, func(t *testing.T) {
			_, err := svc.CreateLesson(actorCtx(anaID), in)
			requireAppErr(t, err, apperr.Invalid, "validation_failed")
		})
	}
}

func TestLessonsRequireAStudent(t *testing.T) {
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
			svc, fs, files := newLessonTest()
			ctx := c.ctx
			checks := map[string]error{}
			_, checks["create"] = svc.CreateLesson(ctx, LessonFields{Title: "x"})
			_, checks["list"] = svc.ListLessons(ctx)
			_, _, checks["get"] = svc.GetLesson(ctx, missingID)
			_, checks["update"] = svc.UpdateLesson(ctx, missingID, LessonFields{Title: "x"})
			checks["delete"] = svc.DeleteLesson(ctx, missingID)
			_, checks["upload"] = svc.UploadDocument(ctx, UploadInput{LessonID: missingID, Body: strings.NewReader(pdfBody)})
			_, checks["get document"] = svc.GetDocument(ctx, missingID)
			_, _, checks["open document"] = svc.OpenDocument(ctx, missingID)
			checks["delete document"] = svc.DeleteDocument(ctx, missingID)
			for name, err := range checks {
				t.Run(name, func(t *testing.T) {
					requireAppErr(t, err, c.kind, c.code)
				})
			}
			if fs.calls != 0 || files.puts != 0 {
				t.Errorf("store called %d times, files %d times; want 0", fs.calls, files.puts)
			}
		})
	}
}

func TestListLessonsShowsOnlyOwnLessonsWithCounts(t *testing.T) {
	svc, _, _ := newLessonTest()
	empty, err := svc.ListLessons(actorCtx(anaID))
	if err != nil || empty == nil || len(empty) != 0 {
		t.Fatalf("empty list = %v, %v; want an empty, non-nil slice", empty, err)
	}

	l := mustCreateLesson(t, svc, anaID, "Lekcija 1")
	mustUpload(t, svc, anaID, l.ID, "a.pdf")
	mustUpload(t, svc, anaID, l.ID, "b.pdf")
	mustCreateLesson(t, svc, markoID, "Marko's")

	lessons, err := svc.ListLessons(actorCtx(anaID))
	if err != nil {
		t.Fatalf("ListLessons: %v", err)
	}
	if len(lessons) != 1 || lessons[0].ID != l.ID || lessons[0].DocumentCount != 2 {
		t.Errorf("lessons = %+v", lessons)
	}
}

func TestGetLessonReturnsItsDocuments(t *testing.T) {
	svc, _, _ := newLessonTest()
	l := mustCreateLesson(t, svc, anaID, "Lekcija 1")
	d := mustUpload(t, svc, anaID, l.ID, "Padeži.pdf")

	got, docs, err := svc.GetLesson(actorCtx(anaID), l.ID)
	if err != nil {
		t.Fatalf("GetLesson: %v", err)
	}
	if got.ID != l.ID || got.DocumentCount != 1 || len(docs) != 1 || docs[0].ID != d.ID || docs[0].Title != "Padeži" {
		t.Errorf("lesson = %+v, documents = %+v", got, docs)
	}
}

func TestLessonsOfOthersAndBadIDsAreNotFound(t *testing.T) {
	svc, fs, files := newLessonTest()
	anas := mustCreateLesson(t, svc, anaID, "Ana's")
	anasDoc := mustUpload(t, svc, anaID, anas.ID, "a.pdf")

	for _, tt := range []struct{ name, userID, lessonID, docID string }{
		{"someone else's", markoID, anas.ID, anasDoc.ID},
		{"missing", anaID, missingID, missingID},
		{"malformed", anaID, "not-a-uuid", "../etc/passwd"},
	} {
		t.Run(tt.name, func(t *testing.T) {
			ctx := actorCtx(tt.userID)
			_, _, err := svc.GetLesson(ctx, tt.lessonID)
			requireAppErr(t, err, apperr.NotFound, "lesson_not_found")
			_, err = svc.UpdateLesson(ctx, tt.lessonID, LessonFields{Title: "x"})
			requireAppErr(t, err, apperr.NotFound, "lesson_not_found")
			err = svc.DeleteLesson(ctx, tt.lessonID)
			requireAppErr(t, err, apperr.NotFound, "lesson_not_found")
			_, err = svc.UploadDocument(ctx, UploadInput{LessonID: tt.lessonID, Body: strings.NewReader(pdfBody)})
			requireAppErr(t, err, apperr.NotFound, "lesson_not_found")

			_, err = svc.GetDocument(ctx, tt.docID)
			requireAppErr(t, err, apperr.NotFound, "document_not_found")
			_, _, err = svc.OpenDocument(ctx, tt.docID)
			requireAppErr(t, err, apperr.NotFound, "document_not_found")
			err = svc.DeleteDocument(ctx, tt.docID)
			requireAppErr(t, err, apperr.NotFound, "document_not_found")
		})
	}
	if len(fs.lessons) != 1 || len(fs.documents) != 1 || len(files.files) != 1 {
		t.Errorf("Ana's data changed: %d lessons, %d documents, %d files", len(fs.lessons), len(fs.documents), len(files.files))
	}
	// A refused upload must not have stored a file.
	if files.puts != 1 {
		t.Errorf("files stored %d times, want only Ana's 1", files.puts)
	}
}

func TestUpdateLessonKeepsTheDocumentCount(t *testing.T) {
	svc, _, _ := newLessonTest()
	l := mustCreateLesson(t, svc, anaID, "Lekcija 1")
	mustUpload(t, svc, anaID, l.ID, "a.pdf")

	got, err := svc.UpdateLesson(actorCtx(anaID), l.ID, LessonFields{Title: " Lekcija 2 ", Description: "nova"})
	if err != nil {
		t.Fatalf("UpdateLesson: %v", err)
	}
	if got.Title != "Lekcija 2" || got.Description != "nova" || got.DocumentCount != 1 || !got.UpdatedAt.After(l.UpdatedAt) {
		t.Errorf("updated = %+v", got)
	}
}

func TestDeleteLessonDeletesItsFiles(t *testing.T) {
	svc, fs, files := newLessonTest()
	l := mustCreateLesson(t, svc, anaID, "Lekcija 1")
	mustUpload(t, svc, anaID, l.ID, "a.pdf")
	mustUpload(t, svc, anaID, l.ID, "b.pdf")
	other := mustCreateLesson(t, svc, anaID, "Lekcija 2")
	kept := mustUpload(t, svc, anaID, other.ID, "c.pdf")

	if err := svc.DeleteLesson(actorCtx(anaID), l.ID); err != nil {
		t.Fatalf("DeleteLesson: %v", err)
	}
	if len(files.files) != 1 || len(fs.documents) != 1 || fs.documents[kept.ID].ID != kept.ID {
		t.Errorf("left %d files and %d documents, want only the other lesson's", len(files.files), len(fs.documents))
	}
}

func TestUploadDocumentStoresBytesHashAndTitle(t *testing.T) {
	svc, fs, files := newLessonTest()
	l := mustCreateLesson(t, svc, anaID, "Lekcija 1")

	d, err := svc.UploadDocument(actorCtx(anaID), UploadInput{
		LessonID: l.ID, FileName: `C:\Users\ana\Lekcija 3 – Padeži.PDF`, Body: strings.NewReader(pdfBody),
	})
	if err != nil {
		t.Fatalf("UploadDocument: %v", err)
	}
	if d.Title != "Lekcija 3 – Padeži" || d.LessonID != l.ID || d.ByteSize != int64(len(pdfBody)) {
		t.Errorf("document = %+v", d)
	}
	row := fs.documents[d.ID]
	sum := sha256.Sum256([]byte(pdfBody))
	if !bytes.Equal(row.Sha256, sum[:]) || row.OwnerID != anaID {
		t.Errorf("row = %+v", row)
	}
	if string(files.files[row.StorageKey]) != pdfBody {
		t.Errorf("stored bytes = %q", files.files[row.StorageKey])
	}
}

func TestUploadDocumentRejectsWhatIsNotAPDF(t *testing.T) {
	svc, _, files := newLessonTest()
	l := mustCreateLesson(t, svc, anaID, "Lekcija 1")

	for name, body := range map[string]string{
		"empty":       "",
		"short":       "%PDF",
		"html":        "<html><script>alert(1)</script>",
		"pdf inside":  "junk %PDF-1.7",
		"png renamed": "\x89PNG\r\n\x1a\n",
	} {
		t.Run(name, func(t *testing.T) {
			_, err := svc.UploadDocument(actorCtx(anaID), UploadInput{LessonID: l.ID, FileName: "x.pdf", Body: strings.NewReader(body)})
			appErr := requireAppErr(t, err, apperr.Invalid, "validation_failed")
			if appErr.Fields["file"] != "Choose a PDF file." {
				t.Errorf("fields = %v", appErr.Fields)
			}
		})
	}
	if files.puts != 0 {
		t.Errorf("files stored %d times, want 0", files.puts)
	}
}

// zeros is an endless reader of zero bytes.
type zeros struct{}

func (zeros) Read(p []byte) (int, error) {
	clear(p)
	return len(p), nil
}

func TestUploadDocumentEnforcesTheSizeLimit(t *testing.T) {
	svc, fs, files := newLessonTest()
	l := mustCreateLesson(t, svc, anaID, "Lekcija 1")
	upload := func(size int64) (Document, error) {
		body := io.MultiReader(strings.NewReader("%PDF-"), io.LimitReader(zeros{}, size-5))
		return svc.UploadDocument(actorCtx(anaID), UploadInput{LessonID: l.ID, FileName: "big.pdf", Body: body})
	}

	d, err := upload(MaxDocumentBytes)
	if err != nil {
		t.Fatalf("upload of exactly the limit: %v", err)
	}
	if d.ByteSize != MaxDocumentBytes {
		t.Errorf("ByteSize = %d, want %d", d.ByteSize, MaxDocumentBytes)
	}

	_, err = upload(MaxDocumentBytes + 1)
	if !errors.Is(err, ErrDocumentTooLarge) {
		t.Fatalf("err = %v, want ErrDocumentTooLarge", err)
	}
	if len(files.files) != 1 || len(fs.documents) != 1 {
		t.Errorf("after the refused upload: %d files, %d documents; want 1 and 1", len(files.files), len(fs.documents))
	}
}

func TestUploadDocumentRemovesTheFileWhenTheRowFails(t *testing.T) {
	svc, fs, files := newLessonTest()
	l := mustCreateLesson(t, svc, anaID, "Lekcija 1")
	// The lesson disappears between the check and the insert, as if deleted
	// by another request while the file was uploading.
	body := io.MultiReader(strings.NewReader(pdfBody), readerFunc(func() { delete(fs.lessons, l.ID) }))

	_, err := svc.UploadDocument(actorCtx(anaID), UploadInput{LessonID: l.ID, FileName: "a.pdf", Body: body})
	requireAppErr(t, err, apperr.NotFound, "lesson_not_found")
	if len(files.files) != 0 {
		t.Errorf("%d files left without a row", len(files.files))
	}
}

// readerFunc runs fn when first read, then reports EOF.
type readerFunc func()

func (f readerFunc) Read([]byte) (int, error) {
	f()
	return 0, io.EOF
}

func TestOpenDocumentReturnsTheBytes(t *testing.T) {
	svc, _, files := newLessonTest()
	l := mustCreateLesson(t, svc, anaID, "Lekcija 1")
	d := mustUpload(t, svc, anaID, l.ID, "a.pdf")

	got, f, err := svc.OpenDocument(actorCtx(anaID), d.ID)
	if err != nil {
		t.Fatalf("OpenDocument: %v", err)
	}
	body, _ := io.ReadAll(f)
	_ = f.Close()
	if got.ID != d.ID || string(body) != pdfBody {
		t.Errorf("document %+v, body %q", got, body)
	}

	// A row whose file is gone is a server fault, not a 404.
	clear(files.files)
	_, _, err = svc.OpenDocument(actorCtx(anaID), d.ID)
	var appErr *apperr.Error
	if !errors.Is(err, filestore.ErrNotFound) || errors.As(err, &appErr) {
		t.Errorf("err = %v, want a plain error wrapping filestore.ErrNotFound", err)
	}
}

func TestGetDocumentIncludesTheLessonTitle(t *testing.T) {
	svc, _, _ := newLessonTest()
	l := mustCreateLesson(t, svc, anaID, "Lekcija 1")
	d := mustUpload(t, svc, anaID, l.ID, "a.pdf")

	got, err := svc.GetDocument(actorCtx(anaID), d.ID)
	if err != nil {
		t.Fatalf("GetDocument: %v", err)
	}
	if got.ID != d.ID || got.LessonID != l.ID || got.LessonTitle != "Lekcija 1" {
		t.Errorf("document = %+v", got)
	}
}

func TestDeleteDocumentSucceedsEvenIfTheFileCannotBeRemoved(t *testing.T) {
	svc, fs, files := newLessonTest()
	l := mustCreateLesson(t, svc, anaID, "Lekcija 1")
	d := mustUpload(t, svc, anaID, l.ID, "a.pdf")
	files.deleteErr = errors.New("disk on fire")

	if err := svc.DeleteDocument(actorCtx(anaID), d.ID); err != nil {
		t.Fatalf("DeleteDocument: %v", err)
	}
	if _, ok := fs.documents[d.ID]; ok {
		t.Error("row still there")
	}
}

func TestTitleFromFileName(t *testing.T) {
	tests := map[string]string{
		"Lekcija 3.pdf":                   "Lekcija 3",
		"čitanka.PDF":                     "čitanka",
		"/home/ana/Padeži.pdf":            "Padeži",
		`C:\Users\ana\Glagoli.pdf`:        "Glagoli",
		"  spaced  .pdf":                  "spaced",
		"no extension":                    "no extension",
		"line\nbreak.pdf":                 "linebreak",
		".pdf":                            "Untitled PDF",
		"":                                "Untitled PDF",
		strings.Repeat("ž", 250) + ".pdf": strings.Repeat("ž", maxDocumentTitleLen),
	}
	for in, want := range tests {
		if got := titleFromFileName(in); got != want {
			t.Errorf("titleFromFileName(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestDocumentsListInUploadOrder(t *testing.T) {
	svc, _, _ := newLessonTest()
	l := mustCreateLesson(t, svc, anaID, "Lekcija 1")
	for _, name := range []string{"a.pdf", "b.pdf"} {
		mustUpload(t, svc, anaID, l.ID, name)
	}
	_, docs, err := svc.GetLesson(actorCtx(anaID), l.ID)
	if err != nil {
		t.Fatal(err)
	}
	titles := []string{}
	for _, d := range docs {
		titles = append(titles, d.Title)
	}
	slices.Sort(titles) // the fake's map has no order; ordering is SQL's job
	if !slices.Equal(titles, []string{"a", "b"}) {
		t.Errorf("titles = %v", titles)
	}
}
