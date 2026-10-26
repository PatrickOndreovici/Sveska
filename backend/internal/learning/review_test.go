package learning

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"

	"learning-serbian-platform/backend/internal/apperr"
	"learning-serbian-platform/backend/internal/store"
)

func TestScheduleOfAWordNotYetLearned(t *testing.T) {
	forgotten := schedule{IntervalDays: 0, EasePercent: 210, Repetitions: 0, Lapses: 2}
	tests := []struct {
		name   string
		from   schedule
		rating Rating
		want   schedule
	}{
		{"new, again", newCard, RatingAgain, schedule{EasePercent: 250}},
		{"new, hard", newCard, RatingHard, schedule{IntervalDays: 1, EasePercent: 250, Repetitions: 1}},
		{"new, good", newCard, RatingGood, schedule{IntervalDays: 2, EasePercent: 250, Repetitions: 1}},
		{"new, easy", newCard, RatingEasy, schedule{IntervalDays: 4, EasePercent: 250, Repetitions: 1}},
		// Forgotten again before it was relearned: no second lapse, ease kept.
		{"forgotten, again", forgotten, RatingAgain, forgotten},
		{"forgotten, good", forgotten, RatingGood, schedule{IntervalDays: 2, EasePercent: 210, Repetitions: 1, Lapses: 2}},
	}
	for _, tt := range tests {
		if got := tt.from.next(tt.rating); got != tt.want {
			t.Errorf("%s: %+v, want %+v", tt.name, got, tt.want)
		}
	}
}

func TestScheduleOfALearnedWord(t *testing.T) {
	learned := schedule{IntervalDays: 10, EasePercent: 250, Repetitions: 3, Lapses: 1}
	tests := []struct {
		rating Rating
		want   schedule
	}{
		{RatingAgain, schedule{IntervalDays: 0, EasePercent: 230, Repetitions: 0, Lapses: 2}},
		{RatingHard, schedule{IntervalDays: 12, EasePercent: 235, Repetitions: 4, Lapses: 1}},
		{RatingGood, schedule{IntervalDays: 25, EasePercent: 250, Repetitions: 4, Lapses: 1}},
		{RatingEasy, schedule{IntervalDays: 33, EasePercent: 265, Repetitions: 4, Lapses: 1}},
	}
	for _, tt := range tests {
		if got := learned.next(tt.rating); got != tt.want {
			t.Errorf("%s: %+v, want %+v", tt.rating, got, tt.want)
		}
	}
}

func TestScheduleStretchesWithEachGoodAnswer(t *testing.T) {
	var got []int
	c := newCard
	for range 8 {
		c = c.next(RatingGood)
		got = append(got, c.IntervalDays)
	}
	want := []int{2, 5, 13, 33, 83, 208, 365, 365}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("intervals = %v, want %v", got, want)
		}
	}
}

func TestScheduleKeepsTheButtonsApart(t *testing.T) {
	// At the lowest ease, 1.2 and the ease round to the same day; each
	// button still gives a day more than the one before.
	for _, c := range []schedule{
		newCard,
		{IntervalDays: 1, EasePercent: minEase, Repetitions: 1},
		{IntervalDays: 2, EasePercent: minEase, Repetitions: 2},
		{IntervalDays: 10, EasePercent: minEase, Repetitions: 5},
	} {
		n := c.nextIntervals()
		if !(n.Again == 0 && n.Hard < n.Good && n.Good < n.Easy) {
			t.Errorf("%+v: next intervals %+v", c, n)
		}
	}
}

func TestScheduleEaseHasAFloor(t *testing.T) {
	c := schedule{IntervalDays: 5, EasePercent: minEase + 5, Repetitions: 2}
	if got := c.next(RatingHard).EasePercent; got != minEase {
		t.Errorf("hard: ease = %d, want %d", got, minEase)
	}
	if got := c.next(RatingAgain).EasePercent; got != minEase {
		t.Errorf("again: ease = %d, want %d", got, minEase)
	}
}

// fakeReviewStore keeps words and cards in maps and answers the queue
// queries with fixed rows.
type fakeReviewStore struct {
	entries map[string]store.VocabularyEntry
	cards   map[string]store.ReviewCard

	counts   store.GetReviewCountsRow
	due      []store.ListDueReviewCardsRow
	fresh    []store.VocabularyEntry
	nextDate *time.Time

	recordErr error // returned by RecordReview instead of saving

	countArgs store.GetReviewCountsParams
	dueArgs   store.ListDueReviewCardsParams
	newLimits []int32
	recorded  []store.RecordReviewParams
	ownerIDs  []string
}

func (f *fakeReviewStore) GetReviewCounts(_ context.Context, arg store.GetReviewCountsParams) (store.GetReviewCountsRow, error) {
	f.ownerIDs = append(f.ownerIDs, arg.OwnerID)
	f.countArgs = arg
	return f.counts, nil
}

func (f *fakeReviewStore) ListDueReviewCards(_ context.Context, arg store.ListDueReviewCardsParams) ([]store.ListDueReviewCardsRow, error) {
	f.ownerIDs = append(f.ownerIDs, arg.OwnerID)
	f.dueArgs = arg
	return f.due, nil
}

func (f *fakeReviewStore) ListNewReviewEntries(_ context.Context, arg store.ListNewReviewEntriesParams) ([]store.VocabularyEntry, error) {
	f.ownerIDs = append(f.ownerIDs, arg.OwnerID)
	f.newLimits = append(f.newLimits, arg.RowLimit)
	return f.fresh[:min(len(f.fresh), int(arg.RowLimit))], nil
}

func (f *fakeReviewStore) GetNextReviewDate(_ context.Context, arg store.GetNextReviewDateParams) (time.Time, error) {
	f.ownerIDs = append(f.ownerIDs, arg.OwnerID)
	if f.nextDate == nil {
		return time.Time{}, pgx.ErrNoRows
	}
	return *f.nextDate, nil
}

func (f *fakeReviewStore) GetVocabularyEntry(_ context.Context, arg store.GetVocabularyEntryParams) (store.VocabularyEntry, error) {
	e, ok := f.entries[arg.ID]
	if !ok || e.OwnerID != arg.OwnerID {
		return store.VocabularyEntry{}, pgx.ErrNoRows
	}
	return e, nil
}

func (f *fakeReviewStore) GetReviewCard(_ context.Context, arg store.GetReviewCardParams) (store.ReviewCard, error) {
	c, ok := f.cards[arg.EntryID]
	if !ok || c.OwnerID != arg.OwnerID {
		return store.ReviewCard{}, pgx.ErrNoRows
	}
	return c, nil
}

func (f *fakeReviewStore) RecordReview(_ context.Context, arg store.RecordReviewParams) (store.RecordReviewRow, error) {
	if f.recordErr != nil {
		return store.RecordReviewRow{}, f.recordErr
	}
	f.recorded = append(f.recorded, arg)
	return store.RecordReviewRow{EntryID: arg.EntryID, OwnerID: arg.OwnerID, DueOn: arg.DueOn}, nil
}

func (f *fakeReviewStore) ListEntrySources(_ context.Context, arg store.ListEntrySourcesParams) ([]store.ListEntrySourcesRow, error) {
	f.ownerIDs = append(f.ownerIDs, arg.OwnerID)
	var rows []store.ListEntrySourcesRow
	for _, id := range arg.EntryIds {
		rows = append(rows, store.ListEntrySourcesRow{
			EntryID: id, AnnotationID: "a1", DocumentID: "d1", DocumentTitle: "Čitanka", PageNumber: 3,
		})
	}
	return rows, nil
}

// newReviews is a service whose clock reads 23 October 2026, 22:30 UTC,
// which is already 24 October in Belgrade.
func newReviews(f *fakeReviewStore) *ReviewService {
	svc := NewReviewService(f)
	svc.now = func() time.Time { return time.Date(2026, 10, 23, 22, 30, 0, 0, time.UTC) }
	return svc
}

func date(s string) time.Time {
	t, err := time.Parse(time.DateOnly, s)
	if err != nil {
		panic(err)
	}
	return t
}

const (
	kucaID = "00000000-0000-7000-8000-000000000101"
	vodaID = "00000000-0000-7000-8000-000000000102"
	hlebID = "00000000-0000-7000-8000-000000000103"
)

func word(id, term string) store.VocabularyEntry {
	return store.VocabularyEntry{ID: id, OwnerID: anaID, Term: term, Translation: term + " (en)"}
}

func TestReviewQueueHasDueWordsThenNewOnes(t *testing.T) {
	sourceID := "00000000-0000-7000-8000-0000000000aa"
	kuca := word(kucaID, "kuća")
	kuca.SourceAnnotationID = &sourceID
	f := &fakeReviewStore{
		counts: store.GetReviewCountsRow{Due: 1, Unseen: 2},
		due: []store.ListDueReviewCardsRow{{
			VocabularyEntry: kuca, DueOn: date("2026-10-22"), IntervalDays: 10, EasePercent: 250, Repetitions: 3,
		}},
		fresh: []store.VocabularyEntry{word(vodaID, "voda"), word(hlebID, "hleb")},
	}
	q, err := newReviews(f).Queue(actorCtx(anaID), "Europe/Belgrade")
	if err != nil {
		t.Fatal(err)
	}

	if q.Today != "2026-10-24" || q.Due != 1 || q.New != 2 {
		t.Errorf("today %s, due %d, new %d", q.Today, q.Due, q.New)
	}
	belgrade, _ := time.LoadLocation("Europe/Belgrade")
	if !f.countArgs.Today.Equal(date("2026-10-24")) || !f.countArgs.DayStart.Equal(time.Date(2026, 10, 24, 0, 0, 0, 0, belgrade)) {
		t.Errorf("counted for %v from %v", f.countArgs.Today, f.countArgs.DayStart)
	}
	if !f.dueArgs.Today.Equal(date("2026-10-24")) || f.dueArgs.RowLimit != reviewBatch {
		t.Errorf("due asked for %v, limit %d", f.dueArgs.Today, f.dueArgs.RowLimit)
	}
	var terms []string
	for _, c := range q.Cards {
		terms = append(terms, c.Entry.Term)
	}
	if len(q.Cards) != 3 || terms[0] != "kuća" || terms[1] != "voda" || terms[2] != "hleb" {
		t.Fatalf("cards = %v", terms)
	}
	if q.Cards[0].New || !q.Cards[1].New {
		t.Errorf("new = %v, %v", q.Cards[0].New, q.Cards[1].New)
	}
	if want := (NextIntervals{Again: 0, Hard: 12, Good: 25, Easy: 33}); q.Cards[0].NextIntervals != want {
		t.Errorf("due word: next %+v, want %+v", q.Cards[0].NextIntervals, want)
	}
	if want := (NextIntervals{Again: 0, Hard: 1, Good: 2, Easy: 4}); q.Cards[1].NextIntervals != want {
		t.Errorf("new word: next %+v, want %+v", q.Cards[1].NextIntervals, want)
	}
	if src := q.Cards[0].Entry.Source; src == nil || src.DocumentTitle != "Čitanka" {
		t.Errorf("source = %+v", src)
	}
	for _, id := range f.ownerIDs {
		if id != anaID {
			t.Errorf("queried owner %s", id)
		}
	}
}

func TestReviewQueueLimitsNewWordsPerDay(t *testing.T) {
	tests := []struct {
		name      string
		counts    store.GetReviewCountsRow
		due       int
		wantNew   int
		wantLimit []int32 // what ListNewReviewEntries was asked for
	}{
		{"fewer words than the limit", store.GetReviewCountsRow{Unseen: 3}, 0, 3, []int32{3}},
		{"more words than the limit", store.GetReviewCountsRow{Unseen: 100}, 0, newWordsPerDay, []int32{newWordsPerDay}},
		{"some started today", store.GetReviewCountsRow{Unseen: 100, StartedToday: 15}, 0, 5, []int32{5}},
		{"limit reached", store.GetReviewCountsRow{Unseen: 100, StartedToday: 25}, 0, 0, nil},
		{"room left in the batch", store.GetReviewCountsRow{Due: 45, Unseen: 100}, 45, newWordsPerDay, []int32{5}},
		{"no room in the batch", store.GetReviewCountsRow{Due: 80, Unseen: 100}, reviewBatch, newWordsPerDay, nil},
	}
	for _, tt := range tests {
		f := &fakeReviewStore{counts: tt.counts, due: make([]store.ListDueReviewCardsRow, tt.due)}
		q, err := newReviews(f).Queue(actorCtx(anaID), "UTC")
		if err != nil {
			t.Fatal(err)
		}
		if q.New != tt.wantNew || len(f.newLimits) != len(tt.wantLimit) || (len(tt.wantLimit) > 0 && f.newLimits[0] != tt.wantLimit[0]) {
			t.Errorf("%s: new = %d, asked for %v; want %d, %v", tt.name, q.New, f.newLimits, tt.wantNew, tt.wantLimit)
		}
	}
}

func TestReviewQueueSaysWhenToComeBack(t *testing.T) {
	later := date("2026-10-30")
	tomorrow := date("2026-10-24")
	tests := []struct {
		name     string
		counts   store.GetReviewCountsRow
		nextDate *time.Time
		want     string // "" for nil
	}{
		{"no words at all", store.GetReviewCountsRow{}, nil, ""},
		{"the next word due", store.GetReviewCountsRow{}, &later, "2026-10-30"},
		{"new words left for tomorrow", store.GetReviewCountsRow{Unseen: 30}, &later, "2026-10-24"},
		{"new words left, nothing scheduled", store.GetReviewCountsRow{Unseen: 30, StartedToday: 20}, nil, "2026-10-24"},
		{"all new words fit today", store.GetReviewCountsRow{Unseen: 3}, &later, "2026-10-30"},
		{"a word due tomorrow anyway", store.GetReviewCountsRow{Unseen: 30}, &tomorrow, "2026-10-24"},
	}
	for _, tt := range tests {
		q, err := newReviews(&fakeReviewStore{counts: tt.counts, nextDate: tt.nextDate}).Queue(actorCtx(anaID), "UTC")
		if err != nil {
			t.Fatal(err)
		}
		got := ""
		if q.NextDueOn != nil {
			got = *q.NextDueOn
		}
		if got != tt.want {
			t.Errorf("%s: next due on %q, want %q", tt.name, got, tt.want)
		}
	}
}

func TestReviewQueueIsEmptyNotNil(t *testing.T) {
	q, err := newReviews(&fakeReviewStore{}).Queue(actorCtx(anaID), "UTC")
	if err != nil {
		t.Fatal(err)
	}
	if q.Cards == nil || len(q.Cards) != 0 {
		t.Errorf("cards = %#v", q.Cards)
	}
}

func TestRecordReviewOfANewWord(t *testing.T) {
	f := &fakeReviewStore{entries: map[string]store.VocabularyEntry{vodaID: word(vodaID, "voda")}}
	res, err := newReviews(f).Record(actorCtx(anaID), vodaID, ReviewInput{Rating: RatingGood, TimeZone: "Europe/Belgrade"})
	if err != nil {
		t.Fatal(err)
	}
	// Today is 24 October in Belgrade; good on a new word is 2 days.
	want := ReviewResult{EntryID: vodaID, Rating: RatingGood, DueOn: "2026-10-26", IntervalDays: 2}
	if res != want {
		t.Errorf("result = %+v, want %+v", res, want)
	}
	if len(f.recorded) != 1 {
		t.Fatalf("recorded %d reviews", len(f.recorded))
	}
	got := f.recorded[0]
	if got.OwnerID != anaID || got.ReadReviewedAt != nil || got.Rating != store.ReviewRatingGood ||
		got.EasePercent != startEase || got.Repetitions != 1 || !got.DueOn.Equal(date("2026-10-26")) {
		t.Errorf("recorded %+v", got)
	}
}

func TestRecordReviewOfAWordSeenBefore(t *testing.T) {
	last := time.Date(2026, 10, 13, 8, 0, 0, 123000, time.UTC)
	f := &fakeReviewStore{
		entries: map[string]store.VocabularyEntry{kucaID: word(kucaID, "kuća")},
		cards: map[string]store.ReviewCard{kucaID: {
			EntryID: kucaID, OwnerID: anaID, IntervalDays: 10, EasePercent: 250, Repetitions: 3, Lapses: 1,
			LastReviewedAt: last,
		}},
	}
	res, err := newReviews(f).Record(actorCtx(anaID), kucaID, ReviewInput{Rating: RatingAgain, TimeZone: "UTC"})
	if err != nil {
		t.Fatal(err)
	}
	if res.DueOn != "2026-10-23" || res.IntervalDays != 0 {
		t.Errorf("again: due %s in %d days, want today", res.DueOn, res.IntervalDays)
	}
	got := f.recorded[0]
	// The schedule is replaced only if it is still the one read.
	if got.ReadReviewedAt == nil || !got.ReadReviewedAt.Equal(last) {
		t.Errorf("read reviewed at %v, want %v", got.ReadReviewedAt, last)
	}
	if got.Lapses != 2 || got.Repetitions != 0 || got.EasePercent != 230 {
		t.Errorf("recorded %+v", got)
	}
}

func TestRecordReviewWhenAnotherAnswerGotThereFirst(t *testing.T) {
	f := &fakeReviewStore{
		entries:   map[string]store.VocabularyEntry{vodaID: word(vodaID, "voda")},
		recordErr: pgx.ErrNoRows,
	}
	_, err := newReviews(f).Record(actorCtx(anaID), vodaID, ReviewInput{Rating: RatingGood})
	if !errors.Is(err, errReviewConflict) {
		t.Errorf("err = %v, want a conflict", err)
	}
}

func TestRecordReviewOfAMissingWord(t *testing.T) {
	markos := word(hlebID, "hleb")
	markos.OwnerID = markoID
	entries := map[string]store.VocabularyEntry{vodaID: word(vodaID, "voda"), hlebID: markos}
	for name, tc := range map[string]struct {
		id        string
		recordErr error
	}{
		"not an id":            {id: "kuća"},
		"never created":        {id: missingID},
		"someone else's":       {id: hlebID},
		"deleted meanwhile":    {id: vodaID, recordErr: &pgconn.PgError{Code: "23503", ConstraintName: reviewCardEntryFkey}},
		"deleted before a log": {id: vodaID, recordErr: &pgconn.PgError{Code: "23503", ConstraintName: reviewEntryFkey}},
	} {
		f := &fakeReviewStore{entries: entries, recordErr: tc.recordErr}
		_, err := newReviews(f).Record(actorCtx(anaID), tc.id, ReviewInput{Rating: RatingGood})
		if !errors.Is(err, errEntryNotFound) {
			t.Errorf("%s: err = %v, want not found", name, err)
		}
	}
}

func TestRecordReviewValidatesTheAnswer(t *testing.T) {
	f := &fakeReviewStore{entries: map[string]store.VocabularyEntry{vodaID: word(vodaID, "voda")}}
	for _, in := range []ReviewInput{
		{Rating: ""},
		{Rating: "perfect"},
		{Rating: RatingGood, TimeZone: "Mars/Olympus"},
	} {
		_, err := newReviews(f).Record(actorCtx(anaID), vodaID, in)
		var appErr *apperr.Error
		if !errors.As(err, &appErr) || appErr.Kind != apperr.Invalid {
			t.Errorf("%+v: err = %v, want a validation error", in, err)
		}
	}
	if len(f.recorded) != 0 {
		t.Error("an invalid answer was recorded")
	}
}

func TestReviewsAreForStudentsOnly(t *testing.T) {
	f := &fakeReviewStore{entries: map[string]store.VocabularyEntry{vodaID: word(vodaID, "voda")}}
	_, queueErr := newReviews(f).Queue(teacherCtx(), "UTC")
	_, recordErr := newReviews(f).Record(teacherCtx(), vodaID, ReviewInput{Rating: RatingGood})
	for _, err := range []error{queueErr, recordErr} {
		var appErr *apperr.Error
		if !errors.As(err, &appErr) || appErr.Kind != apperr.Forbidden {
			t.Errorf("err = %v, want forbidden", err)
		}
	}
	if len(f.ownerIDs) != 0 || len(f.recorded) != 0 {
		t.Error("the store was used")
	}
}
