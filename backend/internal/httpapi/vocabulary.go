package httpapi

import (
	"context"
	"net/http"
	"net/url"
	"strconv"
	"time"

	"github.com/go-chi/chi/v5"

	"learning-serbian-platform/backend/internal/apperr"
	"learning-serbian-platform/backend/internal/learning"
)

// VocabularyService is what the HTTP layer needs from learning.
type VocabularyService interface {
	Create(ctx context.Context, in learning.EntryFields) (learning.VocabularyEntry, error)
	List(ctx context.Context, in learning.ListEntriesInput) (learning.EntryPage, error)
	Update(ctx context.Context, id string, in learning.EntryFields) (learning.VocabularyEntry, error)
	Delete(ctx context.Context, id string) error
}

// entryRequest is the body of POST /api/vocabulary and PUT /api/vocabulary/{id}.
// Only term and translation are required. PUT replaces the whole entry, so a
// field left out there is cleared.
type entryRequest struct {
	Term         string `json:"term"`
	Translation  string `json:"translation"`
	Notes        string `json:"notes"`
	Example      string `json:"example"`
	PartOfSpeech string `json:"partOfSpeech"`
	Gender       string `json:"gender"`
	Plural       string `json:"plural"`
	Aspect       string `json:"aspect"`
	AspectPair   string `json:"aspectPair"`
	FeminineForm string `json:"feminineForm"`
	NeuterForm   string `json:"neuterForm"`
}

func (req entryRequest) toFields() learning.EntryFields {
	return learning.EntryFields{
		Term:         req.Term,
		Translation:  req.Translation,
		Notes:        req.Notes,
		Example:      req.Example,
		PartOfSpeech: learning.PartOfSpeech(req.PartOfSpeech),
		Gender:       learning.Gender(req.Gender),
		Plural:       req.Plural,
		Aspect:       learning.Aspect(req.Aspect),
		AspectPair:   req.AspectPair,
		FeminineForm: req.FeminineForm,
		NeuterForm:   req.NeuterForm,
	}
}

type entryResponse struct {
	ID           string    `json:"id"`
	Term         string    `json:"term"`
	Translation  string    `json:"translation"`
	Notes        string    `json:"notes"`
	Example      string    `json:"example"`
	PartOfSpeech string    `json:"partOfSpeech"`
	Gender       string    `json:"gender"`
	Plural       string    `json:"plural"`
	Aspect       string    `json:"aspect"`
	AspectPair   string    `json:"aspectPair"`
	FeminineForm string    `json:"feminineForm"`
	NeuterForm   string    `json:"neuterForm"`
	CreatedAt    time.Time `json:"createdAt"`
	UpdatedAt    time.Time `json:"updatedAt"`
}

type entryEnvelope struct {
	Entry entryResponse `json:"entry"`
}

type entryPageResponse struct {
	Entries         []entryResponse `json:"entries"`
	Total           int             `json:"total"`
	UnfilteredTotal int             `json:"unfilteredTotal"`
	Page            int             `json:"page"`
	PageSize        int             `json:"pageSize"`
}

func toEntryResponse(e learning.VocabularyEntry) entryResponse {
	return entryResponse{
		ID:           e.ID,
		Term:         e.Term,
		Translation:  e.Translation,
		Notes:        e.Notes,
		Example:      e.Example,
		PartOfSpeech: string(e.PartOfSpeech),
		Gender:       string(e.Gender),
		Plural:       e.Plural,
		Aspect:       string(e.Aspect),
		AspectPair:   e.AspectPair,
		FeminineForm: e.FeminineForm,
		NeuterForm:   e.NeuterForm,
		CreatedAt:    e.CreatedAt.UTC(),
		UpdatedAt:    e.UpdatedAt.UTC(),
	}
}

func (a *api) listVocabulary(w http.ResponseWriter, r *http.Request) {
	in, err := listInputFromQuery(r.URL.Query())
	if err != nil {
		writeServiceError(w, r, err)
		return
	}
	page, err := a.vocabulary.List(r.Context(), in)
	if err != nil {
		writeServiceError(w, r, err)
		return
	}
	// make, not a nil slice, so an empty page encodes as [] rather than null.
	entries := make([]entryResponse, len(page.Entries))
	for i, e := range page.Entries {
		entries[i] = toEntryResponse(e)
	}
	writeJSON(w, http.StatusOK, entryPageResponse{
		Entries:         entries,
		Total:           page.Total,
		UnfilteredTotal: page.UnfilteredTotal,
		Page:            page.Page,
		PageSize:        page.PageSize,
	})
}

// listInputFromQuery reads ?query=&days=&sort=&page=. A missing parameter
// stays at its zero value, which the service treats as the default. Only the
// parsing happens here; the service checks the values.
func listInputFromQuery(q url.Values) (learning.ListEntriesInput, error) {
	in := learning.ListEntriesInput{Query: q.Get("query"), Sort: learning.EntrySort(q.Get("sort"))}
	fields := map[string]string{}
	var err error
	if in.Days, err = intParam(q, "days"); err != nil {
		fields["days"] = "Use a whole number of days."
	}
	if in.Page, err = intParam(q, "page"); err != nil {
		fields["page"] = "Use a whole page number."
	}
	if len(fields) > 0 {
		return learning.ListEntriesInput{}, apperr.Validation(fields)
	}
	return in, nil
}

func intParam(q url.Values, name string) (int, error) {
	s := q.Get(name)
	if s == "" {
		return 0, nil
	}
	return strconv.Atoi(s)
}

func (a *api) createVocabularyEntry(w http.ResponseWriter, r *http.Request) {
	var req entryRequest
	if err := decodeJSON(w, r, &req); err != nil {
		writeServiceError(w, r, err)
		return
	}
	entry, err := a.vocabulary.Create(r.Context(), req.toFields())
	if err != nil {
		writeServiceError(w, r, err)
		return
	}
	writeJSON(w, http.StatusCreated, entryEnvelope{Entry: toEntryResponse(entry)})
}

func (a *api) updateVocabularyEntry(w http.ResponseWriter, r *http.Request) {
	var req entryRequest
	if err := decodeJSON(w, r, &req); err != nil {
		writeServiceError(w, r, err)
		return
	}
	entry, err := a.vocabulary.Update(r.Context(), chi.URLParam(r, "id"), req.toFields())
	if err != nil {
		writeServiceError(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, entryEnvelope{Entry: toEntryResponse(entry)})
}

func (a *api) deleteVocabularyEntry(w http.ResponseWriter, r *http.Request) {
	if err := a.vocabulary.Delete(r.Context(), chi.URLParam(r, "id")); err != nil {
		writeServiceError(w, r, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
