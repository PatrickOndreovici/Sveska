package httpapi

import (
	"context"
	"encoding/json"
	"net/http"
	"strings"
	"testing"
	"time"

	"learning-serbian-platform/backend/internal/apperr"
	"learning-serbian-platform/backend/internal/learning"
)

const entryID = "019a0000-0000-7000-8000-0000000000e1"

var testEntry = learning.VocabularyEntry{
	ID: entryID,
	EntryFields: learning.EntryFields{
		Term: "kuća", Translation: "house",
		PartOfSpeech: learning.PartOfSpeechNoun, Gender: learning.GenderFeminine, Plural: "kuće",
	},
	CreatedAt: time.Date(2026, 10, 23, 12, 0, 0, 0, time.UTC),
	UpdatedAt: time.Date(2026, 10, 23, 12, 0, 0, 0, time.UTC),
}

// fakeVocabulary records what the handlers pass in and returns testEntry,
// or err when set.
type fakeVocabulary struct {
	err        error
	calls      int
	lastFields learning.EntryFields
	lastList   learning.ListEntriesInput
	lastID     string
	page       learning.EntryPage
}

func (f *fakeVocabulary) Create(_ context.Context, in learning.EntryFields) (learning.VocabularyEntry, error) {
	f.calls++
	f.lastFields = in
	return testEntry, f.err
}

func (f *fakeVocabulary) List(_ context.Context, in learning.ListEntriesInput) (learning.EntryPage, error) {
	f.calls++
	f.lastList = in
	return f.page, f.err
}

func (f *fakeVocabulary) Update(_ context.Context, id string, in learning.EntryFields) (learning.VocabularyEntry, error) {
	f.calls++
	f.lastID, f.lastFields = id, in
	return testEntry, f.err
}

func (f *fakeVocabulary) Delete(_ context.Context, id string) error {
	f.calls++
	f.lastID = id
	return f.err
}

func newVocabularyRouter(v *fakeVocabulary) http.Handler {
	return NewRouter(Dependencies{DB: fakePinger{}, Auth: &fakeAuth{}, Vocabulary: v, AppOrigin: testOrigin, CookieSecure: true})
}

// loggedIn is a request from Ana's browser.
func loggedIn(body string) requestOpts {
	return requestOpts{body: body, cookie: validToken, origin: testOrigin}
}

const kucaBody = `{"term":"kuća","translation":"house","partOfSpeech":"noun","gender":"feminine","plural":"kuće"}`

func TestVocabularyRequiresSession(t *testing.T) {
	routes := []struct{ method, path, body string }{
		{http.MethodGet, "/api/vocabulary", ""},
		{http.MethodPost, "/api/vocabulary", kucaBody},
		{http.MethodPut, "/api/vocabulary/" + entryID, kucaBody},
		{http.MethodDelete, "/api/vocabulary/" + entryID, ""},
	}
	for _, rt := range routes {
		t.Run(rt.method, func(t *testing.T) {
			v := &fakeVocabulary{}
			rec := do(t, newVocabularyRouter(v), rt.method, rt.path, requestOpts{body: rt.body, origin: testOrigin})
			if rec.Code != http.StatusUnauthorized {
				t.Errorf("status = %d, want 401", rec.Code)
			}
			if v.calls != 0 {
				t.Errorf("service called %d times, want 0", v.calls)
			}
		})
	}
}

func TestCreateVocabularyEntry(t *testing.T) {
	v := &fakeVocabulary{}
	rec := do(t, newVocabularyRouter(v), http.MethodPost, "/api/vocabulary", loggedIn(kucaBody))

	if rec.Code != http.StatusCreated {
		t.Fatalf("status = %d, body = %s", rec.Code, rec.Body)
	}
	want := learning.EntryFields{
		Term: "kuća", Translation: "house",
		PartOfSpeech: learning.PartOfSpeechNoun, Gender: learning.GenderFeminine, Plural: "kuće",
	}
	if v.lastFields != want {
		t.Errorf("service received %+v, want %+v", v.lastFields, want)
	}

	// The JSON keys are the API contract the frontend reads.
	var body map[string]map[string]any
	if err := json.NewDecoder(rec.Body).Decode(&body); err != nil {
		t.Fatalf("decode: %v", err)
	}
	entry := body["entry"]
	for key, value := range map[string]any{
		"id": entryID, "term": "kuća", "partOfSpeech": "noun", "gender": "feminine", "plural": "kuće",
		"aspect": "", "aspectPair": "", "createdAt": "2026-10-23T12:00:00Z",
	} {
		if entry[key] != value {
			t.Errorf("entry[%q] = %v, want %v", key, entry[key], value)
		}
	}
}

func TestCreateVocabularyEntryRejectsUnknownField(t *testing.T) {
	v := &fakeVocabulary{}
	rec := do(t, newVocabularyRouter(v), http.MethodPost, "/api/vocabulary", loggedIn(`{"term":"kuća","translation":"house","ownerId":"someone-else"}`))
	if rec.Code != http.StatusUnprocessableEntity || decodeError(t, rec).Code != "invalid_json" {
		t.Errorf("status = %d, body = %s; want 422 invalid_json", rec.Code, rec.Body)
	}
	if v.calls != 0 {
		t.Errorf("service called %d times, want 0", v.calls)
	}
}

func TestListVocabularyReadsQueryParameters(t *testing.T) {
	v := &fakeVocabulary{page: learning.EntryPage{Entries: []learning.VocabularyEntry{testEntry}, Total: 11, UnfilteredTotal: 40, Page: 2, PageSize: 10}}
	rec := do(t, newVocabularyRouter(v), http.MethodGet, "/api/vocabulary?query=kuca&days=7&sort=az&page=2", loggedIn(""))

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", rec.Code, rec.Body)
	}
	want := learning.ListEntriesInput{Query: "kuca", Days: 7, Sort: learning.SortAlphabetical, Page: 2}
	if v.lastList != want {
		t.Errorf("service received %+v, want %+v", v.lastList, want)
	}
	var body entryPageResponse
	if err := json.NewDecoder(rec.Body).Decode(&body); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if len(body.Entries) != 1 || body.Entries[0].ID != entryID || body.Total != 11 || body.UnfilteredTotal != 40 || body.Page != 2 || body.PageSize != 10 {
		t.Errorf("body = %+v", body)
	}
}

func TestListVocabularyWithoutParametersUsesDefaults(t *testing.T) {
	v := &fakeVocabulary{page: learning.EntryPage{Entries: []learning.VocabularyEntry{}, Page: 1, PageSize: 10}}
	rec := do(t, newVocabularyRouter(v), http.MethodGet, "/api/vocabulary", loggedIn(""))

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", rec.Code, rec.Body)
	}
	if v.lastList != (learning.ListEntriesInput{}) {
		t.Errorf("service received %+v, want the zero value", v.lastList)
	}
	if !strings.Contains(rec.Body.String(), `"entries":[]`) {
		t.Errorf("body = %s, want an empty entries array", rec.Body)
	}
}

func TestListVocabularyRejectsNonNumbers(t *testing.T) {
	v := &fakeVocabulary{}
	rec := do(t, newVocabularyRouter(v), http.MethodGet, "/api/vocabulary?days=week&page=two", loggedIn(""))

	if rec.Code != http.StatusUnprocessableEntity {
		t.Fatalf("status = %d, want 422", rec.Code)
	}
	fields := decodeError(t, rec).Fields
	if fields["days"] == "" || fields["page"] == "" {
		t.Errorf("fields = %v, want days and page", fields)
	}
	if v.calls != 0 {
		t.Errorf("service called %d times, want 0", v.calls)
	}
}

func TestUpdateVocabularyEntry(t *testing.T) {
	v := &fakeVocabulary{}
	rec := do(t, newVocabularyRouter(v), http.MethodPut, "/api/vocabulary/"+entryID, loggedIn(`{"term":"dom","translation":"home"}`))

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", rec.Code, rec.Body)
	}
	// Fields left out of a PUT reach the service empty, which clears them.
	if v.lastID != entryID || v.lastFields != (learning.EntryFields{Term: "dom", Translation: "home"}) {
		t.Errorf("service received %q %+v", v.lastID, v.lastFields)
	}
	var body entryEnvelope
	if err := json.NewDecoder(rec.Body).Decode(&body); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if body.Entry.ID != entryID {
		t.Errorf("body = %+v", body)
	}
}

func TestDeleteVocabularyEntry(t *testing.T) {
	v := &fakeVocabulary{}
	rec := do(t, newVocabularyRouter(v), http.MethodDelete, "/api/vocabulary/"+entryID, loggedIn(""))

	if rec.Code != http.StatusNoContent || rec.Body.Len() != 0 {
		t.Errorf("status = %d, body = %q; want 204 with no body", rec.Code, rec.Body)
	}
	if v.lastID != entryID {
		t.Errorf("service received id %q", v.lastID)
	}
}

func TestVocabularyServiceErrorsReachTheClient(t *testing.T) {
	tests := []struct {
		name   string
		method string
		path   string
		body   string
		err    error
		status int
		code   string
	}{
		{"missing entry", http.MethodDelete, "/api/vocabulary/" + entryID, "",
			apperr.New(apperr.NotFound, "entry_not_found", "This word is not in your vocabulary."), http.StatusNotFound, "entry_not_found"},
		{"duplicate", http.MethodPost, "/api/vocabulary", kucaBody,
			apperr.New(apperr.Conflict, "duplicate_entry", "You already have this word with this translation."), http.StatusConflict, "duplicate_entry"},
		{"invalid fields", http.MethodPut, "/api/vocabulary/" + entryID, kucaBody,
			apperr.Validation(map[string]string{"term": "Enter the Serbian word."}), http.StatusUnprocessableEntity, "validation_failed"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rec := do(t, newVocabularyRouter(&fakeVocabulary{err: tt.err}), tt.method, tt.path, loggedIn(tt.body))
			if rec.Code != tt.status {
				t.Fatalf("status = %d, want %d", rec.Code, tt.status)
			}
			if got := decodeError(t, rec).Code; got != tt.code {
				t.Errorf("code = %q, want %q", got, tt.code)
			}
		})
	}
}
