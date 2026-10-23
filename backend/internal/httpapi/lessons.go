package httpapi

import (
	"context"
	"errors"
	"io"
	"mime"
	"mime/multipart"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"

	"learning-serbian-platform/backend/internal/apperr"
	"learning-serbian-platform/backend/internal/learning"
)

// LessonService is what the HTTP layer needs from learning.
type LessonService interface {
	CreateLesson(ctx context.Context, in learning.LessonFields) (learning.Lesson, error)
	ListLessons(ctx context.Context) ([]learning.Lesson, error)
	GetLesson(ctx context.Context, id string) (learning.Lesson, []learning.Document, error)
	UpdateLesson(ctx context.Context, id string, in learning.LessonFields) (learning.Lesson, error)
	DeleteLesson(ctx context.Context, id string) error
	UploadDocument(ctx context.Context, in learning.UploadInput) (learning.Document, error)
	GetDocument(ctx context.Context, id string) (learning.DocumentInLesson, error)
	OpenDocument(ctx context.Context, id string) (learning.Document, io.ReadSeekCloser, error)
	DeleteDocument(ctx context.Context, id string) error
}

// lessonRequest is the body of POST /api/lessons and PUT /api/lessons/{id}.
type lessonRequest struct {
	Title       string `json:"title"`
	Description string `json:"description"`
}

type lessonResponse struct {
	ID            string    `json:"id"`
	Title         string    `json:"title"`
	Description   string    `json:"description"`
	DocumentCount int       `json:"documentCount"`
	CreatedAt     time.Time `json:"createdAt"`
	UpdatedAt     time.Time `json:"updatedAt"`
}

type lessonEnvelope struct {
	Lesson lessonResponse `json:"lesson"`
}

type lessonListResponse struct {
	Lessons []lessonResponse `json:"lessons"`
}

type lessonDetailResponse struct {
	Lesson    lessonResponse     `json:"lesson"`
	Documents []documentResponse `json:"documents"`
}

type documentResponse struct {
	ID        string    `json:"id"`
	LessonID  string    `json:"lessonId"`
	Title     string    `json:"title"`
	ByteSize  int64     `json:"byteSize"`
	CreatedAt time.Time `json:"createdAt"`
}

type documentEnvelope struct {
	Document documentResponse `json:"document"`
}

type lessonRef struct {
	ID    string `json:"id"`
	Title string `json:"title"`
}

type documentDetailResponse struct {
	Document documentResponse `json:"document"`
	Lesson   lessonRef        `json:"lesson"`
}

func toLessonResponse(l learning.Lesson) lessonResponse {
	return lessonResponse{
		ID: l.ID, Title: l.Title, Description: l.Description, DocumentCount: l.DocumentCount,
		CreatedAt: l.CreatedAt.UTC(), UpdatedAt: l.UpdatedAt.UTC(),
	}
}

func toDocumentResponse(d learning.Document) documentResponse {
	return documentResponse{ID: d.ID, LessonID: d.LessonID, Title: d.Title, ByteSize: d.ByteSize, CreatedAt: d.CreatedAt.UTC()}
}

func (a *api) listLessons(w http.ResponseWriter, r *http.Request) {
	lessons, err := a.lessons.ListLessons(r.Context())
	if err != nil {
		writeServiceError(w, r, err)
		return
	}
	out := make([]lessonResponse, len(lessons))
	for i, l := range lessons {
		out[i] = toLessonResponse(l)
	}
	writeJSON(w, http.StatusOK, lessonListResponse{Lessons: out})
}

func (a *api) createLesson(w http.ResponseWriter, r *http.Request) {
	var req lessonRequest
	if err := decodeJSON(w, r, &req); err != nil {
		writeServiceError(w, r, err)
		return
	}
	lesson, err := a.lessons.CreateLesson(r.Context(), learning.LessonFields{Title: req.Title, Description: req.Description})
	if err != nil {
		writeServiceError(w, r, err)
		return
	}
	writeJSON(w, http.StatusCreated, lessonEnvelope{Lesson: toLessonResponse(lesson)})
}

func (a *api) getLesson(w http.ResponseWriter, r *http.Request) {
	lesson, docs, err := a.lessons.GetLesson(r.Context(), chi.URLParam(r, "id"))
	if err != nil {
		writeServiceError(w, r, err)
		return
	}
	out := make([]documentResponse, len(docs))
	for i, d := range docs {
		out[i] = toDocumentResponse(d)
	}
	writeJSON(w, http.StatusOK, lessonDetailResponse{Lesson: toLessonResponse(lesson), Documents: out})
}

func (a *api) updateLesson(w http.ResponseWriter, r *http.Request) {
	var req lessonRequest
	if err := decodeJSON(w, r, &req); err != nil {
		writeServiceError(w, r, err)
		return
	}
	lesson, err := a.lessons.UpdateLesson(r.Context(), chi.URLParam(r, "id"), learning.LessonFields{Title: req.Title, Description: req.Description})
	if err != nil {
		writeServiceError(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, lessonEnvelope{Lesson: toLessonResponse(lesson)})
}

func (a *api) deleteLesson(w http.ResponseWriter, r *http.Request) {
	if err := a.lessons.DeleteLesson(r.Context(), chi.URLParam(r, "id")); err != nil {
		writeServiceError(w, r, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// maxMultipartOverhead is room for the multipart boundaries and headers
// around the file, on top of the file's own limit.
const maxMultipartOverhead = 64 << 10

func (a *api) uploadDocument(w http.ResponseWriter, r *http.Request) {
	r.Body = http.MaxBytesReader(w, r.Body, learning.MaxDocumentBytes+maxMultipartOverhead)
	part, err := filePart(r, "Choose a PDF to upload.")
	if err != nil {
		writeServiceError(w, r, uploadError(err, learning.ErrDocumentTooLarge))
		return
	}
	defer part.Close()

	doc, err := a.lessons.UploadDocument(r.Context(), learning.UploadInput{
		LessonID: chi.URLParam(r, "id"),
		FileName: part.FileName(),
		Body:     part,
	})
	if err != nil {
		writeServiceError(w, r, uploadError(err, learning.ErrDocumentTooLarge))
		return
	}
	writeJSON(w, http.StatusCreated, documentEnvelope{Document: toDocumentResponse(doc)})
}

// filePart returns the part named "file" of a multipart/form-data body,
// streaming: the file is read by the service, not buffered here. Other
// parts are skipped. missing is the problem reported when there is no file.
func filePart(r *http.Request, missing string) (*multipart.Part, error) {
	mr, err := r.MultipartReader()
	if err != nil {
		return nil, apperr.New(apperr.Invalid, "unsupported_content_type", "Send the file as multipart/form-data.")
	}
	for {
		part, err := mr.NextPart()
		if errors.Is(err, io.EOF) {
			return nil, apperr.Validation(map[string]string{"file": missing})
		}
		if err != nil {
			return nil, err
		}
		if part.FormName() == "file" {
			return part, nil
		}
	}
}

// uploadError gives a body over the request limit the same answer as a
// file over the service's limit (tooLarge), and any other unreadable body
// a 422.
func uploadError(err error, tooLarge error) error {
	var overLimit *http.MaxBytesError
	if errors.As(err, &overLimit) {
		return tooLarge
	}
	var appErr *apperr.Error
	if !errors.As(err, &appErr) && isMultipartError(err) {
		return apperr.New(apperr.Invalid, "invalid_multipart", "The upload could not be read. Try again.")
	}
	return err
}

func isMultipartError(err error) bool {
	return errors.Is(err, io.ErrUnexpectedEOF) || errors.Is(err, multipart.ErrMessageTooLarge)
}

func (a *api) getDocument(w http.ResponseWriter, r *http.Request) {
	doc, err := a.lessons.GetDocument(r.Context(), chi.URLParam(r, "id"))
	if err != nil {
		writeServiceError(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, documentDetailResponse{
		Document: toDocumentResponse(doc.Document),
		Lesson:   lessonRef{ID: doc.LessonID, Title: doc.LessonTitle},
	})
}

// downloadDocument sends the PDF bytes. http.ServeContent answers Range
// requests, which PDF.js uses to load large files a piece at a time.
func (a *api) downloadDocument(w http.ResponseWriter, r *http.Request) {
	doc, file, err := a.lessons.OpenDocument(r.Context(), chi.URLParam(r, "id"))
	if err != nil {
		writeServiceError(w, r, err)
		return
	}
	defer file.Close()

	h := w.Header()
	h.Set("Content-Type", "application/pdf")
	// attachment: opening the URL directly downloads the file instead of
	// rendering a user-supplied document on our origin. The sandbox
	// directive is a second layer if a browser renders it anyway.
	h.Set("Content-Disposition", contentDisposition(doc.Title+".pdf"))
	h.Set("Content-Security-Policy", "default-src 'none'; frame-ancestors 'none'; sandbox")
	http.ServeContent(w, r, "", doc.CreatedAt, file)
}

// contentDisposition encodes the file name for the header; mime encodes
// non-ASCII names (č, ć, đ) as RFC 2231 filename*.
func contentDisposition(filename string) string {
	if v := mime.FormatMediaType("attachment", map[string]string{"filename": filename}); v != "" {
		return v
	}
	return "attachment"
}

func (a *api) deleteDocument(w http.ResponseWriter, r *http.Request) {
	if err := a.lessons.DeleteDocument(r.Context(), chi.URLParam(r, "id")); err != nil {
		writeServiceError(w, r, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
