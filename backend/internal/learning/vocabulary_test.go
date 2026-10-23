package learning

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"

	"learning-serbian-platform/backend/internal/access"
	"learning-serbian-platform/backend/internal/apperr"
	"learning-serbian-platform/backend/internal/identity"
	"learning-serbian-platform/backend/internal/store"
)

// fakeStore keeps entries in memory. It mirrors the owner filter and the
// unique (owner, lower(term), translation) index. Search, date filtering and
// ordering are SQL's job and are covered by the integration tests; List here
// returns the owner's entries and records the parameters it was given.
type fakeStore struct {
	entries    map[string]store.VocabularyEntry // keyed by id
	lastList   store.ListVocabularyEntriesParams
	counts     []store.CountVocabularyEntriesParams
	nextID     int
	now        time.Time
	storeCalls int
}

func newFakeStore() *fakeStore {
	return &fakeStore{
		entries: map[string]store.VocabularyEntry{},
		now:     time.Date(2026, 10, 23, 12, 0, 0, 0, time.UTC),
	}
}

func (f *fakeStore) isDuplicate(ownerID, term, translation, exceptID string) bool {
	for _, e := range f.entries {
		if e.ID != exceptID && e.OwnerID == ownerID && strings.EqualFold(e.Term, term) && e.Translation == translation {
			return true
		}
	}
	return false
}

var errDuplicate = &pgconn.PgError{Code: "23505", ConstraintName: duplicateEntryIndex}

func (f *fakeStore) CreateVocabularyEntry(_ context.Context, arg store.CreateVocabularyEntryParams) (store.VocabularyEntry, error) {
	f.storeCalls++
	if f.isDuplicate(arg.OwnerID, arg.Term, arg.Translation, "") {
		return store.VocabularyEntry{}, errDuplicate
	}
	f.nextID++
	row := store.VocabularyEntry{
		ID: fmt.Sprintf("00000000-0000-7000-8000-%012d", f.nextID), OwnerID: arg.OwnerID,
		Term: arg.Term, Translation: arg.Translation, Notes: arg.Notes, Example: arg.Example,
		PartOfSpeech: arg.PartOfSpeech, Gender: arg.Gender, Plural: arg.Plural,
		Aspect: arg.Aspect, AspectPair: arg.AspectPair, FeminineForm: arg.FeminineForm, NeuterForm: arg.NeuterForm,
		CreatedAt: f.now, UpdatedAt: f.now,
	}
	f.entries[row.ID] = row
	return row, nil
}

func (f *fakeStore) ListVocabularyEntries(_ context.Context, arg store.ListVocabularyEntriesParams) ([]store.VocabularyEntry, error) {
	f.storeCalls++
	f.lastList = arg
	var rows []store.VocabularyEntry
	for _, e := range f.entries {
		if e.OwnerID == arg.OwnerID {
			rows = append(rows, e)
		}
	}
	return rows, nil
}

func (f *fakeStore) CountVocabularyEntries(_ context.Context, arg store.CountVocabularyEntriesParams) (int64, error) {
	f.storeCalls++
	f.counts = append(f.counts, arg)
	var n int64
	for _, e := range f.entries {
		// Like List, the fake ignores the search and time range.
		if e.OwnerID == arg.OwnerID {
			n++
		}
	}
	return n, nil
}

func (f *fakeStore) UpdateVocabularyEntry(_ context.Context, arg store.UpdateVocabularyEntryParams) (store.VocabularyEntry, error) {
	f.storeCalls++
	row, ok := f.entries[arg.ID]
	if !ok || row.OwnerID != arg.OwnerID {
		return store.VocabularyEntry{}, pgx.ErrNoRows
	}
	if f.isDuplicate(arg.OwnerID, arg.Term, arg.Translation, arg.ID) {
		return store.VocabularyEntry{}, errDuplicate
	}
	row.Term, row.Translation, row.Notes, row.Example = arg.Term, arg.Translation, arg.Notes, arg.Example
	row.PartOfSpeech, row.Gender, row.Plural = arg.PartOfSpeech, arg.Gender, arg.Plural
	row.Aspect, row.AspectPair = arg.Aspect, arg.AspectPair
	row.FeminineForm, row.NeuterForm = arg.FeminineForm, arg.NeuterForm
	row.UpdatedAt = f.now.Add(time.Minute)
	f.entries[row.ID] = row
	return row, nil
}

func (f *fakeStore) DeleteVocabularyEntry(_ context.Context, arg store.DeleteVocabularyEntryParams) (int64, error) {
	f.storeCalls++
	row, ok := f.entries[arg.ID]
	if !ok || row.OwnerID != arg.OwnerID {
		return 0, nil
	}
	delete(f.entries, arg.ID)
	return 1, nil
}

const (
	anaID   = "00000000-0000-7000-8000-00000000000a"
	markoID = "00000000-0000-7000-8000-00000000000b"
	// Well-formed but never created.
	missingID = "00000000-0000-7000-8000-999999999999"
)

func actorCtx(userID string) context.Context {
	return access.WithActor(context.Background(), access.Actor{User: identity.User{ID: userID, Role: identity.RoleStudent}})
}

func teacherCtx() context.Context {
	return access.WithActor(context.Background(), access.Actor{User: identity.User{ID: anaID, Role: identity.RoleTeacher}})
}

var kuca = EntryFields{
	Term: "kuća", Translation: "house",
	PartOfSpeech: PartOfSpeechNoun, Gender: GenderFeminine, Plural: "kuće",
}

func requireAppErr(t *testing.T, err error, kind apperr.Kind, code string) *apperr.Error {
	t.Helper()
	var appErr *apperr.Error
	if !errors.As(err, &appErr) {
		t.Fatalf("error = %v, want *apperr.Error", err)
	}
	if appErr.Kind != kind || appErr.Code != code {
		t.Fatalf("error = %s/%s, want %s/%s", appErr.Kind, appErr.Code, kind, code)
	}
	return appErr
}

func TestCreateSavesTrimmedEntryForActor(t *testing.T) {
	fs := newFakeStore()
	svc := NewVocabularyService(fs)
	in := kuca
	in.Term = "  kuća  "
	in.Notes = " my notes\n"

	entry, err := svc.Create(actorCtx(anaID), in)
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	if entry.Term != "kuća" || entry.Notes != "my notes" {
		t.Errorf("term, notes = %q, %q, want trimmed", entry.Term, entry.Notes)
	}
	if entry.PartOfSpeech != PartOfSpeechNoun || entry.Gender != GenderFeminine || entry.Plural != "kuće" {
		t.Errorf("grammar = %q %q %q, want noun feminine kuće", entry.PartOfSpeech, entry.Gender, entry.Plural)
	}
	if entry.Aspect != "" {
		t.Errorf("aspect = %q, want unset", entry.Aspect)
	}
	saved := fs.entries[entry.ID]
	if saved.OwnerID != anaID {
		t.Errorf("owner = %q, want %q", saved.OwnerID, anaID)
	}
	// Unset enums must reach the database as NULL, not as an empty enum value.
	if saved.Aspect.Valid {
		t.Error("aspect saved as a value, want NULL")
	}
	if !saved.PartOfSpeech.Valid || !saved.Gender.Valid {
		t.Error("part of speech or gender saved as NULL, want a value")
	}
}

func TestCreateRequiresActor(t *testing.T) {
	fs := newFakeStore()
	_, err := NewVocabularyService(fs).Create(context.Background(), kuca)
	requireAppErr(t, err, apperr.Unauthenticated, "unauthenticated")
	if fs.storeCalls != 0 {
		t.Errorf("store called %d times, want 0", fs.storeCalls)
	}
}

func TestVocabularyIsForStudentsOnly(t *testing.T) {
	fs := newFakeStore()
	svc := NewVocabularyService(fs)
	ctx := teacherCtx()
	checks := map[string]error{}
	_, checks["create"] = svc.Create(ctx, kuca)
	_, checks["list"] = svc.List(ctx, ListEntriesInput{})
	_, checks["update"] = svc.Update(ctx, missingID, kuca)
	checks["delete"] = svc.Delete(ctx, missingID)
	for name, err := range checks {
		t.Run(name, func(t *testing.T) {
			requireAppErr(t, err, apperr.Forbidden, "students_only")
		})
	}
	if fs.storeCalls != 0 {
		t.Errorf("store called %d times, want 0", fs.storeCalls)
	}
}

func TestCreateRejectsInvalidFields(t *testing.T) {
	tests := []struct {
		name  string
		edit  func(*EntryFields)
		field string
		want  string
	}{
		{"missing term", func(e *EntryFields) { e.Term = "   " }, "term", "Enter the Serbian word."},
		{"missing translation", func(e *EntryFields) { e.Translation = "" }, "translation", "Enter a translation."},
		{"term too long", func(e *EntryFields) { e.Term = strings.Repeat("ž", maxWordLen+1) }, "term", "Use at most 200 characters."},
		{"notes too long", func(e *EntryFields) { e.Notes = strings.Repeat("a", maxNotesLen+1) }, "notes", "Use at most 2000 characters."},
		{"unknown part of speech", func(e *EntryFields) { e.PartOfSpeech = "pronoun" }, "partOfSpeech", "Choose noun, verb, adjective or other."},
		{"unknown gender", func(e *EntryFields) { e.Gender = "plural" }, "gender", "Choose masculine, feminine or neuter."},
		{"gender on a verb", func(e *EntryFields) { e.PartOfSpeech = PartOfSpeechVerb }, "gender", "Only nouns have a gender."},
		{"plural without part of speech", func(e *EntryFields) { e.PartOfSpeech, e.Gender = "", "" }, "plural", "Only nouns have a plural."},
		{"aspect on a noun", func(e *EntryFields) { e.Aspect = AspectPerfective }, "aspect", "Only verbs have an aspect."},
		{"feminine form on a noun", func(e *EntryFields) { e.FeminineForm = "lepa" }, "feminineForm", "Only adjectives have a feminine form."},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			fs := newFakeStore()
			in := kuca
			tt.edit(&in)

			_, err := NewVocabularyService(fs).Create(actorCtx(anaID), in)
			appErr := requireAppErr(t, err, apperr.Invalid, "validation_failed")
			if got := appErr.Fields[tt.field]; got != tt.want {
				t.Errorf("Fields[%q] = %q, want %q (all: %v)", tt.field, got, tt.want, appErr.Fields)
			}
			if fs.storeCalls != 0 {
				t.Errorf("store called %d times, want 0", fs.storeCalls)
			}
		})
	}
}

func TestCreateCountsCharactersNotBytes(t *testing.T) {
	// 200 "ž" are 400 bytes but 200 characters, which the database accepts.
	in := kuca
	in.Term = strings.Repeat("ž", maxWordLen)
	if _, err := NewVocabularyService(newFakeStore()).Create(actorCtx(anaID), in); err != nil {
		t.Fatalf("Create: %v", err)
	}
}

func TestCreateRejectsDuplicate(t *testing.T) {
	svc := NewVocabularyService(newFakeStore())
	if _, err := svc.Create(actorCtx(anaID), kuca); err != nil {
		t.Fatalf("first Create: %v", err)
	}
	again := kuca
	again.Term = "Kuća"
	_, err := svc.Create(actorCtx(anaID), again)
	requireAppErr(t, err, apperr.Conflict, "duplicate_entry")

	// Another user may have the same word.
	if _, err := svc.Create(actorCtx(markoID), kuca); err != nil {
		t.Fatalf("Create for another user: %v", err)
	}
}

func TestListDefaultsToFirstPageNewestFirst(t *testing.T) {
	fs := newFakeStore()
	svc := NewVocabularyService(fs)
	if _, err := svc.Create(actorCtx(anaID), kuca); err != nil {
		t.Fatalf("Create: %v", err)
	}
	if _, err := svc.Create(actorCtx(markoID), kuca); err != nil {
		t.Fatalf("Create: %v", err)
	}

	page, err := svc.List(actorCtx(anaID), ListEntriesInput{})
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(page.Entries) != 1 || page.Total != 1 || page.UnfilteredTotal != 1 || page.Page != 1 || page.PageSize != pageSize {
		t.Errorf("page = %d entries, total %d, unfiltered %d, page %d, size %d; want 1, 1, 1, 1, %d",
			len(page.Entries), page.Total, page.UnfilteredTotal, page.Page, page.PageSize, pageSize)
	}
	// Without filters one count serves both totals.
	if len(fs.counts) != 1 {
		t.Errorf("count called %d times, want 1", len(fs.counts))
	}
	want := store.ListVocabularyEntriesParams{OwnerID: anaID, Sort: "newest", PageOffset: 0, PageLimit: pageSize}
	if fs.lastList != want {
		t.Errorf("list params = %+v, want %+v", fs.lastList, want)
	}
}

func TestListPassesFiltersAndOffset(t *testing.T) {
	fs := newFakeStore()
	_, err := NewVocabularyService(fs).List(actorCtx(anaID), ListEntriesInput{Query: "  cevap ", Days: 7, Sort: SortAlphabetical, Page: 3})
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	wantList := store.ListVocabularyEntriesParams{OwnerID: anaID, Query: "cevap", Days: 7, Sort: "az", PageOffset: 20, PageLimit: pageSize}
	if fs.lastList != wantList {
		t.Errorf("list params = %+v, want %+v", fs.lastList, wantList)
	}
	// The total uses the same filters as the page; the second count uses none.
	wantCounts := []store.CountVocabularyEntriesParams{
		{OwnerID: anaID, Query: "cevap", Days: 7},
		{OwnerID: anaID},
	}
	if !slices.Equal(fs.counts, wantCounts) {
		t.Errorf("count params = %+v, want %+v", fs.counts, wantCounts)
	}
}

func TestListEmptyPageIsNotNil(t *testing.T) {
	page, err := NewVocabularyService(newFakeStore()).List(actorCtx(anaID), ListEntriesInput{})
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if page.Entries == nil {
		t.Error("Entries is nil, want an empty slice so JSON gets []")
	}
}

func TestListRejectsInvalidInput(t *testing.T) {
	tests := []struct {
		name  string
		in    ListEntriesInput
		field string
	}{
		{"negative days", ListEntriesInput{Days: -1}, "days"},
		{"too many days", ListEntriesInput{Days: maxDays + 1}, "days"},
		{"unknown sort", ListEntriesInput{Sort: "oldest"}, "sort"},
		{"negative page", ListEntriesInput{Page: -1}, "page"},
		{"page past the limit", ListEntriesInput{Page: maxPage + 1}, "page"},
		{"query too long", ListEntriesInput{Query: strings.Repeat("a", maxWordLen+1)}, "query"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			fs := newFakeStore()
			_, err := NewVocabularyService(fs).List(actorCtx(anaID), tt.in)
			appErr := requireAppErr(t, err, apperr.Invalid, "validation_failed")
			if _, ok := appErr.Fields[tt.field]; !ok {
				t.Errorf("Fields = %v, want a problem for %q", appErr.Fields, tt.field)
			}
			if fs.storeCalls != 0 {
				t.Errorf("store called %d times, want 0", fs.storeCalls)
			}
		})
	}
}

func TestUpdateReplacesFields(t *testing.T) {
	svc := NewVocabularyService(newFakeStore())
	created, err := svc.Create(actorCtx(anaID), kuca)
	if err != nil {
		t.Fatalf("Create: %v", err)
	}

	// Turning the noun into a verb: the noun fields must be cleared in the same request.
	updated, err := svc.Update(actorCtx(anaID), created.ID, EntryFields{
		Term: "čitati", Translation: "to read", PartOfSpeech: PartOfSpeechVerb, Aspect: AspectImperfective,
	})
	if err != nil {
		t.Fatalf("Update: %v", err)
	}
	if updated.ID != created.ID || updated.Term != "čitati" || updated.Aspect != AspectImperfective {
		t.Errorf("updated = %+v", updated)
	}
	if updated.Gender != "" || updated.Plural != "" {
		t.Errorf("noun fields = %q %q, want cleared", updated.Gender, updated.Plural)
	}
	if !updated.UpdatedAt.After(created.UpdatedAt) {
		t.Error("UpdatedAt did not move forward")
	}
}

func TestUpdateRejectsDuplicate(t *testing.T) {
	svc := NewVocabularyService(newFakeStore())
	if _, err := svc.Create(actorCtx(anaID), kuca); err != nil {
		t.Fatalf("Create: %v", err)
	}
	home := kuca
	home.Translation = "home"
	other, err := svc.Create(actorCtx(anaID), home)
	if err != nil {
		t.Fatalf("Create: %v", err)
	}

	_, err = svc.Update(actorCtx(anaID), other.ID, kuca)
	requireAppErr(t, err, apperr.Conflict, "duplicate_entry")
}

func TestUpdateAndDeleteReportNotFound(t *testing.T) {
	fs := newFakeStore()
	svc := NewVocabularyService(fs)
	anas, err := svc.Create(actorCtx(anaID), kuca)
	if err != nil {
		t.Fatalf("Create: %v", err)
	}

	tests := []struct {
		name   string
		userID string
		id     string
	}{
		{"missing id", anaID, missingID},
		// Someone else's entry must look exactly like a missing one.
		{"another user's entry", markoID, anas.ID},
		{"malformed id", anaID, "not-a-uuid"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := svc.Update(actorCtx(tt.userID), tt.id, kuca)
			requireAppErr(t, err, apperr.NotFound, "entry_not_found")

			err = svc.Delete(actorCtx(tt.userID), tt.id)
			requireAppErr(t, err, apperr.NotFound, "entry_not_found")
		})
	}
	if _, ok := fs.entries[anas.ID]; !ok {
		t.Error("another user deleted Ana's entry")
	}
}

func TestMalformedIDNeverReachesStore(t *testing.T) {
	fs := newFakeStore()
	svc := NewVocabularyService(fs)
	_, _ = svc.Update(actorCtx(anaID), "1; DROP TABLE", kuca)
	_ = svc.Delete(actorCtx(anaID), "")
	if fs.storeCalls != 0 {
		t.Errorf("store called %d times, want 0", fs.storeCalls)
	}
}

func TestDeleteRemovesEntry(t *testing.T) {
	fs := newFakeStore()
	svc := NewVocabularyService(fs)
	entry, err := svc.Create(actorCtx(anaID), kuca)
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	if err := svc.Delete(actorCtx(anaID), entry.ID); err != nil {
		t.Fatalf("Delete: %v", err)
	}
	if _, ok := fs.entries[entry.ID]; ok {
		t.Error("entry still stored after Delete")
	}
	err = svc.Delete(actorCtx(anaID), entry.ID)
	requireAppErr(t, err, apperr.NotFound, "entry_not_found")
}
