package learning

import (
	"bufio"
	"bytes"
	"context"
	"crypto/sha256"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"strings"
	"time"
	"unicode"
	"unicode/utf8"

	"github.com/jackc/pgx/v5"

	"learning-serbian-platform/backend/internal/access"
	"learning-serbian-platform/backend/internal/apperr"
	"learning-serbian-platform/backend/internal/store"
)

// Lesson is a student's unit of study: a title, a description and the PDFs
// uploaded into it.
type Lesson struct {
	ID            string
	Title         string
	Description   string
	DocumentCount int
	CreatedAt     time.Time
	UpdatedAt     time.Time
}

// LessonFields are what the user types in. Update replaces both.
type LessonFields struct {
	Title       string
	Description string
}

// Document is an uploaded PDF. The bytes are in file storage.
type Document struct {
	ID        string
	LessonID  string
	Title     string
	ByteSize  int64
	CreatedAt time.Time
}

// DocumentInLesson is a document with the title of its lesson, for the
// viewer's way back.
type DocumentInLesson struct {
	Document
	LessonTitle string
}

// UploadInput is one PDF being uploaded into a lesson.
type UploadInput struct {
	LessonID string
	FileName string    // the name on the user's computer; becomes the title
	Body     io.Reader // read once, up to MaxDocumentBytes
}

// MaxDocumentBytes is the largest PDF accepted: enough for a scanned
// textbook chapter, small enough to keep uploads quick.
const MaxDocumentBytes = 20 << 20

const (
	maxLessonTitleLen       = 200
	maxLessonDescriptionLen = 2000
	maxDocumentTitleLen     = 200
)

// LessonStore is the subset of generated queries this service needs.
// *store.Queries satisfies it; tests substitute a fake.
type LessonStore interface {
	CreateLesson(ctx context.Context, arg store.CreateLessonParams) (store.Lesson, error)
	ListLessons(ctx context.Context, ownerID string) ([]store.ListLessonsRow, error)
	GetLesson(ctx context.Context, arg store.GetLessonParams) (store.Lesson, error)
	UpdateLesson(ctx context.Context, arg store.UpdateLessonParams) (store.Lesson, error)
	DeleteLesson(ctx context.Context, arg store.DeleteLessonParams) ([]string, error)
	CreateDocument(ctx context.Context, arg store.CreateDocumentParams) (store.Document, error)
	ListLessonDocuments(ctx context.Context, arg store.ListLessonDocumentsParams) ([]store.Document, error)
	GetDocument(ctx context.Context, arg store.GetDocumentParams) (store.GetDocumentRow, error)
	DeleteDocument(ctx context.Context, arg store.DeleteDocumentParams) (string, error)
}

// FileStore keeps the PDF bytes. *filestore.Local satisfies it.
type FileStore interface {
	Put(ctx context.Context, r io.Reader) (key string, err error)
	Open(ctx context.Context, key string) (io.ReadSeekCloser, error)
	Delete(ctx context.Context, key string) error
}

// LessonService manages the current actor's lessons and the PDFs in them.
// Like vocabulary, every query filters on the owner, so someone else's
// lesson or document behaves exactly like a missing one.
type LessonService struct {
	store LessonStore
	files FileStore
}

func NewLessonService(s LessonStore, files FileStore) *LessonService {
	return &LessonService{store: s, files: files}
}

var (
	errLessonNotFound   = apperr.New(apperr.NotFound, "lesson_not_found", "This lesson does not exist.")
	errDocumentNotFound = apperr.New(apperr.NotFound, "document_not_found", "This PDF does not exist.")
	errNotAPDF          = apperr.Validation(map[string]string{"file": "Choose a PDF file."})

	// ErrDocumentTooLarge is exported so the HTTP layer can give the same
	// answer when the request body itself goes over its limit.
	ErrDocumentTooLarge = apperr.Validation(map[string]string{
		"file": fmt.Sprintf("Choose a PDF of at most %d MB.", MaxDocumentBytes>>20),
	})
)

// CreateLesson adds a lesson for the current user.
func (s *LessonService) CreateLesson(ctx context.Context, in LessonFields) (Lesson, error) {
	actor, err := access.RequireStudent(ctx)
	if err != nil {
		return Lesson{}, err
	}
	in = normalizeLesson(in)
	if fields := validateLesson(in); len(fields) > 0 {
		return Lesson{}, apperr.Validation(fields)
	}
	row, err := s.store.CreateLesson(ctx, store.CreateLessonParams{
		OwnerID: actor.User.ID, Title: in.Title, Description: in.Description,
	})
	if err != nil {
		return Lesson{}, fmt.Errorf("create lesson: %w", err)
	}
	return lessonFromRow(row, 0), nil
}

// ListLessons returns the current user's lessons, newest first.
func (s *LessonService) ListLessons(ctx context.Context) ([]Lesson, error) {
	actor, err := access.RequireStudent(ctx)
	if err != nil {
		return nil, err
	}
	rows, err := s.store.ListLessons(ctx, actor.User.ID)
	if err != nil {
		return nil, fmt.Errorf("list lessons: %w", err)
	}
	// make, not a nil slice, so no lessons encodes as [] rather than null.
	lessons := make([]Lesson, len(rows))
	for i, row := range rows {
		lessons[i] = Lesson{
			ID: row.ID, Title: row.Title, Description: row.Description,
			DocumentCount: int(row.DocumentCount), CreatedAt: row.CreatedAt, UpdatedAt: row.UpdatedAt,
		}
	}
	return lessons, nil
}

// GetLesson returns one of the current user's lessons with its PDFs in
// upload order.
func (s *LessonService) GetLesson(ctx context.Context, id string) (Lesson, []Document, error) {
	actor, err := access.RequireStudent(ctx)
	if err != nil {
		return Lesson{}, nil, err
	}
	if !isUUID(id) {
		return Lesson{}, nil, errLessonNotFound
	}
	row, err := s.store.GetLesson(ctx, store.GetLessonParams{ID: id, OwnerID: actor.User.ID})
	if errors.Is(err, pgx.ErrNoRows) {
		return Lesson{}, nil, errLessonNotFound
	}
	if err != nil {
		return Lesson{}, nil, fmt.Errorf("get lesson %s: %w", id, err)
	}
	docRows, err := s.store.ListLessonDocuments(ctx, store.ListLessonDocumentsParams{LessonID: id, OwnerID: actor.User.ID})
	if err != nil {
		return Lesson{}, nil, fmt.Errorf("list documents of lesson %s: %w", id, err)
	}
	docs := make([]Document, len(docRows))
	for i, d := range docRows {
		docs[i] = documentFromRow(d)
	}
	return lessonFromRow(row, len(docs)), docs, nil
}

// UpdateLesson replaces the title and description of one of the current
// user's lessons.
func (s *LessonService) UpdateLesson(ctx context.Context, id string, in LessonFields) (Lesson, error) {
	actor, err := access.RequireStudent(ctx)
	if err != nil {
		return Lesson{}, err
	}
	if !isUUID(id) {
		return Lesson{}, errLessonNotFound
	}
	in = normalizeLesson(in)
	if fields := validateLesson(in); len(fields) > 0 {
		return Lesson{}, apperr.Validation(fields)
	}
	row, err := s.store.UpdateLesson(ctx, store.UpdateLessonParams{
		ID: id, OwnerID: actor.User.ID, Title: in.Title, Description: in.Description,
	})
	if errors.Is(err, pgx.ErrNoRows) {
		return Lesson{}, errLessonNotFound
	}
	if err != nil {
		return Lesson{}, fmt.Errorf("update lesson %s: %w", id, err)
	}
	// The count is not part of the update; read it so the answer is complete.
	docs, err := s.store.ListLessonDocuments(ctx, store.ListLessonDocumentsParams{LessonID: id, OwnerID: actor.User.ID})
	if err != nil {
		return Lesson{}, fmt.Errorf("list documents of lesson %s: %w", id, err)
	}
	return lessonFromRow(row, len(docs)), nil
}

// DeleteLesson removes one of the current user's lessons together with its
// PDFs, rows and files.
func (s *LessonService) DeleteLesson(ctx context.Context, id string) error {
	actor, err := access.RequireStudent(ctx)
	if err != nil {
		return err
	}
	if !isUUID(id) {
		return errLessonNotFound
	}
	keys, err := s.store.DeleteLesson(ctx, store.DeleteLessonParams{ID: id, OwnerID: actor.User.ID})
	if errors.Is(err, pgx.ErrNoRows) {
		return errLessonNotFound
	}
	if err != nil {
		return fmt.Errorf("delete lesson %s: %w", id, err)
	}
	for _, key := range keys {
		s.deleteFile(ctx, key)
	}
	return nil
}

// UploadDocument stores a PDF in one of the current user's lessons. The
// bytes are checked and hashed while they stream to file storage, so the
// upload is never held in memory.
func (s *LessonService) UploadDocument(ctx context.Context, in UploadInput) (Document, error) {
	actor, err := access.RequireStudent(ctx)
	if err != nil {
		return Document{}, err
	}
	if !isUUID(in.LessonID) {
		return Document{}, errLessonNotFound
	}
	// Check the lesson first, so a wrong id does not cost a 20 MB write.
	_, err = s.store.GetLesson(ctx, store.GetLessonParams{ID: in.LessonID, OwnerID: actor.User.ID})
	if errors.Is(err, pgx.ErrNoRows) {
		return Document{}, errLessonNotFound
	}
	if err != nil {
		return Document{}, fmt.Errorf("get lesson %s: %w", in.LessonID, err)
	}

	// Every PDF starts with "%PDF-". The browser's claimed content type
	// is not trusted; the bytes are.
	body := bufio.NewReader(in.Body)
	if head, err := body.Peek(5); err != nil || !bytes.Equal(head, []byte("%PDF-")) {
		return Document{}, errNotAPDF
	}
	hash := sha256.New()
	limited := &limitedReader{r: io.TeeReader(body, hash), remaining: MaxDocumentBytes}
	key, err := s.files.Put(ctx, limited)
	if errors.Is(err, errTooLarge) {
		return Document{}, ErrDocumentTooLarge
	}
	if err != nil {
		return Document{}, fmt.Errorf("store uploaded file: %w", err)
	}

	row, err := s.store.CreateDocument(ctx, store.CreateDocumentParams{
		OwnerID:    actor.User.ID,
		LessonID:   in.LessonID,
		Title:      titleFromFileName(in.FileName),
		StorageKey: key,
		ByteSize:   limited.read,
		Sha256:     hash.Sum(nil),
	})
	if err != nil {
		// Without a row nothing refers to the file, so remove it.
		s.deleteFile(ctx, key)
		// The lesson was deleted while the file was uploading.
		if isForeignKeyViolation(err, "documents_lesson_fkey") {
			return Document{}, errLessonNotFound
		}
		return Document{}, fmt.Errorf("create document: %w", err)
	}
	return documentFromRow(row), nil
}

// GetDocument returns one of the current user's documents.
func (s *LessonService) GetDocument(ctx context.Context, id string) (DocumentInLesson, error) {
	row, err := s.getDocumentRow(ctx, id)
	if err != nil {
		return DocumentInLesson{}, err
	}
	return DocumentInLesson{Document: documentFromGetRow(row), LessonTitle: row.LessonTitle}, nil
}

// OpenDocument returns one of the current user's documents and its bytes.
// The caller must close the file.
func (s *LessonService) OpenDocument(ctx context.Context, id string) (Document, io.ReadSeekCloser, error) {
	row, err := s.getDocumentRow(ctx, id)
	if err != nil {
		return Document{}, nil, err
	}
	// A missing file here means storage and database disagree: a real
	// failure, not a 404, so it is not mapped to errDocumentNotFound.
	file, err := s.files.Open(ctx, row.StorageKey)
	if err != nil {
		return Document{}, nil, fmt.Errorf("open file of document %s: %w", id, err)
	}
	return documentFromGetRow(row), file, nil
}

// DeleteDocument removes one of the current user's documents, row and file.
func (s *LessonService) DeleteDocument(ctx context.Context, id string) error {
	actor, err := access.RequireStudent(ctx)
	if err != nil {
		return err
	}
	if !isUUID(id) {
		return errDocumentNotFound
	}
	key, err := s.store.DeleteDocument(ctx, store.DeleteDocumentParams{ID: id, OwnerID: actor.User.ID})
	if errors.Is(err, pgx.ErrNoRows) {
		return errDocumentNotFound
	}
	if err != nil {
		return fmt.Errorf("delete document %s: %w", id, err)
	}
	s.deleteFile(ctx, key)
	return nil
}

func (s *LessonService) getDocumentRow(ctx context.Context, id string) (store.GetDocumentRow, error) {
	actor, err := access.RequireStudent(ctx)
	if err != nil {
		return store.GetDocumentRow{}, err
	}
	if !isUUID(id) {
		return store.GetDocumentRow{}, errDocumentNotFound
	}
	row, err := s.store.GetDocument(ctx, store.GetDocumentParams{ID: id, OwnerID: actor.User.ID})
	if errors.Is(err, pgx.ErrNoRows) {
		return store.GetDocumentRow{}, errDocumentNotFound
	}
	if err != nil {
		return store.GetDocumentRow{}, fmt.Errorf("get document %s: %w", id, err)
	}
	return row, nil
}

// deleteFile removes a file whose row is already gone. A failure only
// leaves an unreferenced file behind, so it is logged rather than returned:
// the user's delete has succeeded either way.
func (s *LessonService) deleteFile(ctx context.Context, key string) {
	if err := s.files.Delete(ctx, key); err != nil {
		slog.WarnContext(ctx, "delete stored file", "key", key, "error", err)
	}
}

var errTooLarge = errors.New("upload exceeds the size limit")

// limitedReader passes reads through until more than remaining bytes have
// been read, then fails with errTooLarge. Unlike io.LimitReader, which
// silently stops, it lets the caller tell a too-large upload from a
// complete one. read counts the bytes passed through.
type limitedReader struct {
	r         io.Reader
	remaining int64
	read      int64
}

func (l *limitedReader) Read(p []byte) (int, error) {
	// Allow one byte past the limit so an exactly-full upload still succeeds
	// and only a larger one fails.
	if int64(len(p)) > l.remaining+1 {
		p = p[:l.remaining+1]
	}
	n, err := l.r.Read(p)
	l.read += int64(n)
	l.remaining -= int64(n)
	if l.remaining < 0 {
		return n, errTooLarge
	}
	return n, err
}

func normalizeLesson(in LessonFields) LessonFields {
	in.Title = strings.TrimSpace(in.Title)
	in.Description = strings.TrimSpace(in.Description)
	return in
}

func validateLesson(in LessonFields) map[string]string {
	fields := map[string]string{}
	addProblem(fields, "title", validateLength(in.Title, maxLessonTitleLen))
	addProblem(fields, "description", validateLength(in.Description, maxLessonDescriptionLen))
	if in.Title == "" {
		fields["title"] = "Enter a title."
	}
	return fields
}

// titleFromFileName turns "C:\Users\ana\Lekcija 3.pdf" into "Lekcija 3".
// Some browsers send a full path; control characters are dropped; an
// empty result becomes "Untitled PDF".
func titleFromFileName(name string) string {
	if i := strings.LastIndexAny(name, `/\`); i >= 0 {
		name = name[i+1:]
	}
	if len(name) >= 4 && strings.EqualFold(name[len(name)-4:], ".pdf") {
		name = name[:len(name)-4]
	}
	name = strings.Map(func(r rune) rune {
		if unicode.IsControl(r) {
			return -1
		}
		return r
	}, name)
	name = strings.TrimSpace(strings.ToValidUTF8(name, ""))
	if utf8.RuneCountInString(name) > maxDocumentTitleLen {
		name = strings.TrimSpace(string([]rune(name)[:maxDocumentTitleLen]))
	}
	if name == "" {
		return "Untitled PDF"
	}
	return name
}

func lessonFromRow(row store.Lesson, documentCount int) Lesson {
	return Lesson{
		ID: row.ID, Title: row.Title, Description: row.Description,
		DocumentCount: documentCount, CreatedAt: row.CreatedAt, UpdatedAt: row.UpdatedAt,
	}
}

func documentFromRow(row store.Document) Document {
	return Document{ID: row.ID, LessonID: row.LessonID, Title: row.Title, ByteSize: row.ByteSize, CreatedAt: row.CreatedAt}
}

func documentFromGetRow(row store.GetDocumentRow) Document {
	return Document{ID: row.ID, LessonID: row.LessonID, Title: row.Title, ByteSize: row.ByteSize, CreatedAt: row.CreatedAt}
}
