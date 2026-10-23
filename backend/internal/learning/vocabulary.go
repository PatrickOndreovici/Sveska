// Package learning owns a student's study material. So far: vocabulary.
package learning

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"

	"learning-serbian-platform/backend/internal/access"
	"learning-serbian-platform/backend/internal/apperr"
	"learning-serbian-platform/backend/internal/store"
)

// PartOfSpeech is the optional word class of an entry. "" means not set,
// as do "" for Gender and Aspect.
type PartOfSpeech string

const (
	PartOfSpeechNoun      PartOfSpeech = "noun"
	PartOfSpeechVerb      PartOfSpeech = "verb"
	PartOfSpeechAdjective PartOfSpeech = "adjective"
	PartOfSpeechOther     PartOfSpeech = "other"
)

type Gender string

const (
	GenderMasculine Gender = "masculine"
	GenderFeminine  Gender = "feminine"
	GenderNeuter    Gender = "neuter"
)

type Aspect string

const (
	AspectImperfective Aspect = "imperfective"
	AspectPerfective   Aspect = "perfective"
	AspectBiaspectual  Aspect = "biaspectual"
)

// EntryFields are what the user types in. Create and Update both take the
// full set, the way the add and edit forms send it.
type EntryFields struct {
	Term         string
	Translation  string
	Notes        string
	Example      string
	PartOfSpeech PartOfSpeech
	Gender       Gender // nouns only
	Plural       string // nouns only
	Aspect       Aspect // verbs only
	AspectPair   string // verbs only
	FeminineForm string // adjectives only; the term itself is the masculine form
	NeuterForm   string // adjectives only
}

// VocabularyEntry is a saved word. EntryFields is embedded, so entry.Term
// works directly.
type VocabularyEntry struct {
	ID string
	EntryFields
	Source    *EntrySource // nil for a word typed in rather than saved from a PDF
	CreatedAt time.Time
	UpdatedAt time.Time
}

// EntrySource is the highlight a word was saved from, so the vocabulary can
// link back to the page it was read on.
type EntrySource struct {
	AnnotationID  string
	DocumentID    string
	DocumentTitle string
	PageNumber    int
}

// EntrySort is the order of the list.
type EntrySort string

const (
	SortNewest       EntrySort = "newest"
	SortAlphabetical EntrySort = "az"
)

// ListEntriesInput selects one page of the list. The zero value means no
// search, all time, newest first and the first page.
type ListEntriesInput struct {
	Query string    // substring of the word, its translation or its other forms
	Days  int       // 0 for all time, otherwise only entries added in the last N days
	Sort  EntrySort // "" sorts like SortNewest
	Page  int       // 1-based; 0 means the first page
}

// EntryPage is one page of entries. Total counts every match across all
// pages; UnfilteredTotal counts all of the user's entries, ignoring the
// search and the time range.
type EntryPage struct {
	Entries         []VocabularyEntry
	Total           int
	UnfilteredTotal int
	Page            int
	PageSize        int
}

const (
	pageSize      = 10
	maxPage       = 100_000 // keeps the row offset well inside int32
	maxDays       = 3650
	maxWordLen    = 200 // term, translation, plural and the other forms
	maxNotesLen   = 2000
	maxExampleLen = 500
)

// VocabularyStore is the subset of generated queries this service needs.
// *store.Queries satisfies it; tests substitute a fake.
type VocabularyStore interface {
	CreateVocabularyEntry(ctx context.Context, arg store.CreateVocabularyEntryParams) (store.VocabularyEntry, error)
	ListVocabularyEntries(ctx context.Context, arg store.ListVocabularyEntriesParams) ([]store.VocabularyEntry, error)
	CountVocabularyEntries(ctx context.Context, arg store.CountVocabularyEntriesParams) (int64, error)
	UpdateVocabularyEntry(ctx context.Context, arg store.UpdateVocabularyEntryParams) (store.VocabularyEntry, error)
	DeleteVocabularyEntry(ctx context.Context, arg store.DeleteVocabularyEntryParams) (int64, error)
	GetAnnotationSource(ctx context.Context, arg store.GetAnnotationSourceParams) (store.GetAnnotationSourceRow, error)
	ListEntrySources(ctx context.Context, arg store.ListEntrySourcesParams) ([]store.ListEntrySourcesRow, error)
}

// VocabularyService lets a user keep their own list of words. Every method
// works on the current actor's entries only: the queries filter on owner_id,
// so someone else's entry behaves exactly like a missing one.
type VocabularyService struct {
	store VocabularyStore
}

func NewVocabularyService(s VocabularyStore) *VocabularyService {
	return &VocabularyService{store: s}
}

var (
	errEntryNotFound  = apperr.New(apperr.NotFound, "entry_not_found", "This word is not in your vocabulary.")
	errDuplicateEntry = apperr.New(apperr.Conflict, "duplicate_entry", "You already have this word with this translation.")
)

// Create saves a new entry for the current user.
func (s *VocabularyService) Create(ctx context.Context, in EntryFields) (VocabularyEntry, error) {
	actor, err := access.RequireStudent(ctx)
	if err != nil {
		return VocabularyEntry{}, err
	}
	in = normalizeEntry(in)
	if fields := validateEntry(in); len(fields) > 0 {
		return VocabularyEntry{}, apperr.Validation(fields)
	}
	return s.create(ctx, actor, in, nil)
}

// CreateFromAnnotation saves a new entry that cites the highlight it was
// selected from. The highlight must be one of the current user's own.
func (s *VocabularyService) CreateFromAnnotation(ctx context.Context, annotationID string, in EntryFields) (VocabularyEntry, error) {
	actor, err := access.RequireStudent(ctx)
	if err != nil {
		return VocabularyEntry{}, err
	}
	in = normalizeEntry(in)
	if fields := validateEntry(in); len(fields) > 0 {
		return VocabularyEntry{}, apperr.Validation(fields)
	}
	if !isUUID(annotationID) {
		return VocabularyEntry{}, errAnnotationNotFound
	}
	src, err := s.store.GetAnnotationSource(ctx, store.GetAnnotationSourceParams{ID: annotationID, OwnerID: actor.User.ID})
	if errors.Is(err, pgx.ErrNoRows) {
		return VocabularyEntry{}, errAnnotationNotFound
	}
	if err != nil {
		return VocabularyEntry{}, fmt.Errorf("get annotation %s: %w", annotationID, err)
	}

	entry, err := s.create(ctx, actor, in, &annotationID)
	if err != nil {
		return VocabularyEntry{}, err
	}
	entry.Source = &EntrySource{
		AnnotationID:  src.ID,
		DocumentID:    src.DocumentID,
		DocumentTitle: src.DocumentTitle,
		PageNumber:    int(src.PageNumber),
	}
	return entry, nil
}

// create inserts already validated fields.
func (s *VocabularyService) create(ctx context.Context, actor access.Actor, in EntryFields, sourceAnnotationID *string) (VocabularyEntry, error) {
	row, err := s.store.CreateVocabularyEntry(ctx, store.CreateVocabularyEntryParams{
		OwnerID:      actor.User.ID,
		Term:         in.Term,
		Translation:  in.Translation,
		Notes:        in.Notes,
		Example:      in.Example,
		PartOfSpeech: nullPartOfSpeech(in.PartOfSpeech),
		Gender:       nullGender(in.Gender),
		Plural:       in.Plural,
		Aspect:       nullAspect(in.Aspect),
		AspectPair:   in.AspectPair,
		FeminineForm: in.FeminineForm,
		NeuterForm:   in.NeuterForm,

		SourceAnnotationID: sourceAnnotationID,
	})
	if isUniqueViolation(err, duplicateEntryIndex) {
		return VocabularyEntry{}, errDuplicateEntry
	}
	// The highlight was deleted between the check and the insert.
	if isForeignKeyViolation(err, sourceAnnotationFkey) {
		return VocabularyEntry{}, errAnnotationNotFound
	}
	if err != nil {
		return VocabularyEntry{}, fmt.Errorf("create vocabulary entry: %w", err)
	}
	return entryFromRow(row), nil
}

// List returns one page of the current user's entries.
func (s *VocabularyService) List(ctx context.Context, in ListEntriesInput) (EntryPage, error) {
	actor, err := access.RequireStudent(ctx)
	if err != nil {
		return EntryPage{}, err
	}
	in.Query = strings.TrimSpace(in.Query)
	if in.Sort == "" {
		in.Sort = SortNewest
	}
	if in.Page == 0 {
		in.Page = 1
	}
	if fields := validateList(in); len(fields) > 0 {
		return EntryPage{}, apperr.Validation(fields)
	}

	rows, err := s.store.ListVocabularyEntries(ctx, store.ListVocabularyEntriesParams{
		OwnerID:    actor.User.ID,
		Query:      in.Query,
		Days:       int32(in.Days),
		Sort:       string(in.Sort),
		PageOffset: int32((in.Page - 1) * pageSize),
		PageLimit:  pageSize,
	})
	if err != nil {
		return EntryPage{}, fmt.Errorf("list vocabulary entries: %w", err)
	}
	total, err := s.store.CountVocabularyEntries(ctx, store.CountVocabularyEntriesParams{
		OwnerID: actor.User.ID,
		Query:   in.Query,
		Days:    int32(in.Days),
	})
	if err != nil {
		return EntryPage{}, fmt.Errorf("count vocabulary entries: %w", err)
	}
	// Without filters the two counts are the same, so the second query is skipped.
	unfilteredTotal := total
	if in.Query != "" || in.Days != 0 {
		unfilteredTotal, err = s.store.CountVocabularyEntries(ctx, store.CountVocabularyEntriesParams{OwnerID: actor.User.ID})
		if err != nil {
			return EntryPage{}, fmt.Errorf("count all vocabulary entries: %w", err)
		}
	}

	// make, not a nil slice, so an empty page encodes as [] rather than null.
	entries := make([]VocabularyEntry, len(rows))
	for i, row := range rows {
		entries[i] = entryFromRow(row)
	}
	if err := s.addSources(ctx, actor, entries); err != nil {
		return EntryPage{}, err
	}
	return EntryPage{
		Entries:         entries,
		Total:           int(total),
		UnfilteredTotal: int(unfilteredTotal),
		Page:            in.Page,
		PageSize:        pageSize,
	}, nil
}

// Update replaces every editable field of one of the current user's entries.
func (s *VocabularyService) Update(ctx context.Context, id string, in EntryFields) (VocabularyEntry, error) {
	actor, err := access.RequireStudent(ctx)
	if err != nil {
		return VocabularyEntry{}, err
	}
	if !isUUID(id) {
		return VocabularyEntry{}, errEntryNotFound
	}
	in = normalizeEntry(in)
	if fields := validateEntry(in); len(fields) > 0 {
		return VocabularyEntry{}, apperr.Validation(fields)
	}

	row, err := s.store.UpdateVocabularyEntry(ctx, store.UpdateVocabularyEntryParams{
		ID:           id,
		OwnerID:      actor.User.ID,
		Term:         in.Term,
		Translation:  in.Translation,
		Notes:        in.Notes,
		Example:      in.Example,
		PartOfSpeech: nullPartOfSpeech(in.PartOfSpeech),
		Gender:       nullGender(in.Gender),
		Plural:       in.Plural,
		Aspect:       nullAspect(in.Aspect),
		AspectPair:   in.AspectPair,
		FeminineForm: in.FeminineForm,
		NeuterForm:   in.NeuterForm,
	})
	if errors.Is(err, pgx.ErrNoRows) {
		return VocabularyEntry{}, errEntryNotFound
	}
	if isUniqueViolation(err, duplicateEntryIndex) {
		return VocabularyEntry{}, errDuplicateEntry
	}
	if err != nil {
		return VocabularyEntry{}, fmt.Errorf("update vocabulary entry %s: %w", id, err)
	}
	entries := []VocabularyEntry{entryFromRow(row)}
	if err := s.addSources(ctx, actor, entries); err != nil {
		return VocabularyEntry{}, err
	}
	return entries[0], nil
}

// addSources fills in Source for the entries saved from a highlight. Most
// words are typed in, so the query only runs when one of them cites one.
func (s *VocabularyService) addSources(ctx context.Context, actor access.Actor, entries []VocabularyEntry) error {
	var ids []string
	for _, e := range entries {
		if e.Source != nil {
			ids = append(ids, e.ID)
		}
	}
	if len(ids) == 0 {
		return nil
	}
	rows, err := s.store.ListEntrySources(ctx, store.ListEntrySourcesParams{OwnerID: actor.User.ID, EntryIds: ids})
	if err != nil {
		return fmt.Errorf("list entry sources: %w", err)
	}
	byEntry := make(map[string]EntrySource, len(rows))
	for _, r := range rows {
		byEntry[r.EntryID] = EntrySource{
			AnnotationID:  r.AnnotationID,
			DocumentID:    r.DocumentID,
			DocumentTitle: r.DocumentTitle,
			PageNumber:    int(r.PageNumber),
		}
	}
	for i := range entries {
		if entries[i].Source == nil {
			continue
		}
		// Deleted since the entry was read: the database has cleared the link.
		src, ok := byEntry[entries[i].ID]
		if !ok {
			entries[i].Source = nil
			continue
		}
		entries[i].Source = &src
	}
	return nil
}

// Delete removes one of the current user's entries.
func (s *VocabularyService) Delete(ctx context.Context, id string) error {
	actor, err := access.RequireStudent(ctx)
	if err != nil {
		return err
	}
	if !isUUID(id) {
		return errEntryNotFound
	}
	deleted, err := s.store.DeleteVocabularyEntry(ctx, store.DeleteVocabularyEntryParams{ID: id, OwnerID: actor.User.ID})
	if err != nil {
		return fmt.Errorf("delete vocabulary entry %s: %w", id, err)
	}
	if deleted == 0 {
		return errEntryNotFound
	}
	return nil
}

func normalizeEntry(in EntryFields) EntryFields {
	in.Term = strings.TrimSpace(in.Term)
	in.Translation = strings.TrimSpace(in.Translation)
	in.Notes = strings.TrimSpace(in.Notes)
	in.Example = strings.TrimSpace(in.Example)
	in.Plural = strings.TrimSpace(in.Plural)
	in.AspectPair = strings.TrimSpace(in.AspectPair)
	in.FeminineForm = strings.TrimSpace(in.FeminineForm)
	in.NeuterForm = strings.TrimSpace(in.NeuterForm)
	return in
}

// validateEntry returns the problems keyed by JSON field name, like
// identity's sign-up. It repeats the database's CHECKs so a bad request is
// a 422 with a message instead of a 500.
func validateEntry(in EntryFields) map[string]string {
	fields := map[string]string{}
	addProblem(fields, "term", validateLength(in.Term, maxWordLen))
	addProblem(fields, "translation", validateLength(in.Translation, maxWordLen))
	addProblem(fields, "notes", validateLength(in.Notes, maxNotesLen))
	addProblem(fields, "example", validateLength(in.Example, maxExampleLen))
	addProblem(fields, "plural", validateLength(in.Plural, maxWordLen))
	addProblem(fields, "aspectPair", validateLength(in.AspectPair, maxWordLen))
	addProblem(fields, "feminineForm", validateLength(in.FeminineForm, maxWordLen))
	addProblem(fields, "neuterForm", validateLength(in.NeuterForm, maxWordLen))
	addProblem(fields, "partOfSpeech", validatePartOfSpeech(in.PartOfSpeech))
	addProblem(fields, "gender", validateGender(in.Gender))
	addProblem(fields, "aspect", validateAspect(in.Aspect))

	if in.Term == "" {
		fields["term"] = "Enter the Serbian word."
	}
	if in.Translation == "" {
		fields["translation"] = "Enter a translation."
	}

	// Each grammar field belongs to one part of speech only.
	if in.PartOfSpeech != PartOfSpeechNoun {
		if in.Gender != "" {
			fields["gender"] = "Only nouns have a gender."
		}
		if in.Plural != "" {
			fields["plural"] = "Only nouns have a plural."
		}
	}
	if in.PartOfSpeech != PartOfSpeechVerb {
		if in.Aspect != "" {
			fields["aspect"] = "Only verbs have an aspect."
		}
		if in.AspectPair != "" {
			fields["aspectPair"] = "Only verbs have an aspect pair."
		}
	}
	if in.PartOfSpeech != PartOfSpeechAdjective {
		if in.FeminineForm != "" {
			fields["feminineForm"] = "Only adjectives have a feminine form."
		}
		if in.NeuterForm != "" {
			fields["neuterForm"] = "Only adjectives have a neuter form."
		}
	}
	return fields
}

func validateList(in ListEntriesInput) map[string]string {
	fields := map[string]string{}
	addProblem(fields, "query", validateLength(in.Query, maxWordLen))
	if in.Days < 0 || in.Days > maxDays {
		fields["days"] = fmt.Sprintf("Use 0 for all time, or up to %d days.", maxDays)
	}
	if in.Sort != SortNewest && in.Sort != SortAlphabetical {
		fields["sort"] = "Sort by newest or az."
	}
	if in.Page < 1 || in.Page > maxPage {
		fields["page"] = fmt.Sprintf("Use a page number from 1 to %d.", maxPage)
	}
	return fields
}

func validatePartOfSpeech(p PartOfSpeech) string {
	switch p {
	case "", PartOfSpeechNoun, PartOfSpeechVerb, PartOfSpeechAdjective, PartOfSpeechOther:
		return ""
	}
	return "Choose noun, verb, adjective or other."
}

func validateGender(g Gender) string {
	switch g {
	case "", GenderMasculine, GenderFeminine, GenderNeuter:
		return ""
	}
	return "Choose masculine, feminine or neuter."
}

func validateAspect(a Aspect) string {
	switch a {
	case "", AspectImperfective, AspectPerfective, AspectBiaspectual:
		return ""
	}
	return "Choose imperfective, perfective or biaspectual."
}

// The database stores an unset enum as NULL; this package uses "". The
// generated Null* types bridge the two: Valid false is NULL.

func nullPartOfSpeech(p PartOfSpeech) store.NullPartOfSpeech {
	return store.NullPartOfSpeech{PartOfSpeech: store.PartOfSpeech(p), Valid: p != ""}
}

func nullGender(g Gender) store.NullGrammaticalGender {
	return store.NullGrammaticalGender{GrammaticalGender: store.GrammaticalGender(g), Valid: g != ""}
}

func nullAspect(a Aspect) store.NullVerbAspect {
	return store.NullVerbAspect{VerbAspect: store.VerbAspect(a), Valid: a != ""}
}

// entryFromRow converts a database row. A NULL enum scans as "", which is
// already this package's "not set".
func entryFromRow(row store.VocabularyEntry) VocabularyEntry {
	entry := VocabularyEntry{
		ID: row.ID,
		EntryFields: EntryFields{
			Term:         row.Term,
			Translation:  row.Translation,
			Notes:        row.Notes,
			Example:      row.Example,
			PartOfSpeech: PartOfSpeech(row.PartOfSpeech.PartOfSpeech),
			Gender:       Gender(row.Gender.GrammaticalGender),
			Plural:       row.Plural,
			Aspect:       Aspect(row.Aspect.VerbAspect),
			AspectPair:   row.AspectPair,
			FeminineForm: row.FeminineForm,
			NeuterForm:   row.NeuterForm,
		},
		CreatedAt: row.CreatedAt,
		UpdatedAt: row.UpdatedAt,
	}
	// Only the id is known here; addSources replaces it with the full source.
	if row.SourceAnnotationID != nil {
		entry.Source = &EntrySource{AnnotationID: *row.SourceAnnotationID}
	}
	return entry
}

const (
	// duplicateEntryIndex is the unique index on (owner_id, lower(term), translation).
	duplicateEntryIndex = "vocabulary_entries_owner_term_translation_key"
	// sourceAnnotationFkey ties source_annotation_id to the owner's own annotations.
	sourceAnnotationFkey = "vocabulary_entries_source_annotation_fkey"
)
