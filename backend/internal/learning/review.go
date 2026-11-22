package learning

import (
	"context"
	"errors"
	"fmt"
	"math"
	"time"

	"github.com/jackc/pgx/v5"

	"learning-serbian-platform/backend/internal/access"
	"learning-serbian-platform/backend/internal/apperr"
	"learning-serbian-platform/backend/internal/store"
)

// Rating is how well a student remembered a word when its card came up.
type Rating string

const (
	RatingAgain Rating = "again" // forgotten: the word comes back today
	RatingHard  Rating = "hard"
	RatingGood  Rating = "good"
	RatingEasy  Rating = "easy"
)

// Direction is which way round a card goes. Every word has a Serbian →
// English card; once that one is learned, it gets an English → Serbian card
// too, with a schedule of its own.
type Direction string

const (
	SerbianToEnglish Direction = "sr_en" // the Serbian word shown, its meaning recalled
	EnglishToSerbian Direction = "en_sr" // the meaning shown, the Serbian word typed
)

// NextIntervals is how many days away each answer would put a word, shown
// on the answer buttons.
type NextIntervals struct {
	Again int
	Hard  int
	Good  int
	Easy  int
}

// ReviewCard is a word to review, which way round, with what each answer
// would do.
type ReviewCard struct {
	Entry         VocabularyEntry
	Direction     Direction
	New           bool // never reviewed this way round before
	NextIntervals NextIntervals
}

// ReviewQueue is what a student has to review today, in the order to show
// it: cards due first, longest overdue first, then the first English →
// Serbian cards of words learned the other way, then new words, oldest
// first. A word has at most one card in it.
type ReviewQueue struct {
	Today string
	Cards []ReviewCard // at most reviewBatch; ask again once they are answered
	Due   int          // words due today in all, Cards included
	New   int          // new cards to start today in all, of both kinds, Cards included
	// NextDueOn is the first day after today with words to review, or nil
	// when there are no words at all.
	NextDueOn *string
}

// ReviewInput is an answer to one card.
type ReviewInput struct {
	Direction Direction
	Rating    Rating
	TimeZone  string // see DashboardService.Get
}

// ReviewResult is when a card comes back after an answer.
type ReviewResult struct {
	EntryID      string
	Direction    Direction
	Rating       Rating
	DueOn        string
	IntervalDays int
}

const (
	// newWordsPerDay keeps a long list of words from arriving all at once.
	newWordsPerDay = 20
	// recallAfter is how many times in a row a word must be remembered
	// Serbian → English before it gets an English → Serbian card, so that
	// nobody is asked to write a word they have seen once.
	recallAfter = 2
	// newRecallPerDay is the daily limit of new English → Serbian cards, so
	// that words learned before they existed do not all arrive at once.
	newRecallPerDay = 20
	// reviewBatch is how many cards one request returns.
	reviewBatch = 50
)

// ReviewStore is the subset of generated queries flashcards need.
type ReviewStore interface {
	GetReviewCounts(ctx context.Context, arg store.GetReviewCountsParams) (store.GetReviewCountsRow, error)
	ListDueReviewCards(ctx context.Context, arg store.ListDueReviewCardsParams) ([]store.ListDueReviewCardsRow, error)
	ListNewReviewEntries(ctx context.Context, arg store.ListNewReviewEntriesParams) ([]store.VocabularyEntry, error)
	ListNewRecallEntries(ctx context.Context, arg store.ListNewRecallEntriesParams) ([]store.ListNewRecallEntriesRow, error)
	GetNextReviewDate(ctx context.Context, arg store.GetNextReviewDateParams) (time.Time, error)
	GetVocabularyEntry(ctx context.Context, arg store.GetVocabularyEntryParams) (store.VocabularyEntry, error)
	GetReviewCard(ctx context.Context, arg store.GetReviewCardParams) (store.ReviewCard, error)
	RecordReview(ctx context.Context, arg store.RecordReviewParams) (store.RecordReviewRow, error)
	ListEntrySources(ctx context.Context, arg store.ListEntrySourcesParams) ([]store.ListEntrySourcesRow, error)
}

// ReviewService turns a student's vocabulary into flashcards that come back
// on a schedule (see schedule.next). Like VocabularyService, it only ever
// reaches the current student's own words.
type ReviewService struct {
	store ReviewStore
	now   func() time.Time
}

func NewReviewService(s ReviewStore) *ReviewService {
	return &ReviewService{store: s, now: time.Now}
}

var errReviewConflict = apperr.New(apperr.Conflict, "review_conflict",
	"This card was just reviewed somewhere else. Go on with the next one.")

// Queue returns what the student has to review today in timeZone.
func (s *ReviewService) Queue(ctx context.Context, timeZone string) (ReviewQueue, error) {
	actor, err := access.RequireStudent(ctx)
	if err != nil {
		return ReviewQueue{}, err
	}
	day, err := currentDay(timeZone, s.now())
	if err != nil {
		return ReviewQueue{}, err
	}
	ownerID := actor.User.ID
	today := day.date()

	counts, err := s.store.GetReviewCounts(ctx, reviewCountsParams(ownerID, day))
	if err != nil {
		return ReviewQueue{}, fmt.Errorf("get review counts: %w", err)
	}
	newToday, recallToday := newWordsLeft(counts), newRecallLeft(counts)

	dueRows, err := s.store.ListDueReviewCards(ctx, store.ListDueReviewCardsParams{
		OwnerID: ownerID, Today: today, DayStart: day.start, RowLimit: reviewBatch,
	})
	if err != nil {
		return ReviewQueue{}, fmt.Errorf("list due review cards: %w", err)
	}
	room := reviewBatch - len(dueRows)
	// English → Serbian cards come before new words: their words are known.
	var recallRows []store.ListNewRecallEntriesRow
	if n := min(recallToday, room); n > 0 {
		recallRows, err = s.store.ListNewRecallEntries(ctx, store.ListNewRecallEntriesParams{
			OwnerID: ownerID, MinRepetitions: recallAfter, Today: today, DayStart: day.start, RowLimit: int32(n),
		})
		if err != nil {
			return ReviewQueue{}, fmt.Errorf("list new recall entries: %w", err)
		}
		room -= len(recallRows)
	}
	var newRows []store.VocabularyEntry
	if n := min(newToday, room); n > 0 {
		newRows, err = s.store.ListNewReviewEntries(ctx, store.ListNewReviewEntriesParams{
			OwnerID: ownerID, RowLimit: int32(n),
		})
		if err != nil {
			return ReviewQueue{}, fmt.Errorf("list new review entries: %w", err)
		}
	}

	size := len(dueRows) + len(recallRows) + len(newRows)
	entries := make([]VocabularyEntry, 0, size)
	cards := make([]ReviewCard, 0, size) // each card's Entry comes from entries, once sources are added
	for _, r := range dueRows {
		entries = append(entries, entryFromRow(r.VocabularyEntry))
		cards = append(cards, ReviewCard{
			Direction: Direction(r.Direction),
			NextIntervals: schedule{
				IntervalDays: int(r.IntervalDays), EasePercent: int(r.EasePercent),
				Repetitions: int(r.Repetitions), Lapses: int(r.Lapses),
			}.nextIntervals(),
		})
	}
	for _, r := range recallRows {
		entries = append(entries, entryFromRow(r.VocabularyEntry))
		cards = append(cards, ReviewCard{Direction: EnglishToSerbian, New: true, NextIntervals: newCard.nextIntervals()})
	}
	for _, r := range newRows {
		entries = append(entries, entryFromRow(r))
		cards = append(cards, ReviewCard{Direction: SerbianToEnglish, New: true, NextIntervals: newCard.nextIntervals()})
	}
	if err := addSources(ctx, s.store, ownerID, entries); err != nil {
		return ReviewQueue{}, err
	}
	for i := range cards {
		cards[i].Entry = entries[i]
	}

	q := ReviewQueue{
		Today: today.Format(time.DateOnly),
		Cards: cards,
		Due:   int(counts.Due),
		New:   newToday + recallToday,
	}

	tomorrow := today.AddDate(0, 0, 1)
	next, err := s.store.GetNextReviewDate(ctx, store.GetNextReviewDateParams{
		OwnerID: ownerID, Tomorrow: tomorrow, DayStart: day.start,
	})
	switch {
	case errors.Is(err, pgx.ErrNoRows):
	case err != nil:
		return ReviewQueue{}, fmt.Errorf("get next review date: %w", err)
	default:
		date := next.Format(time.DateOnly)
		q.NextDueOn = &date
	}
	// New cards left over today, for lack of room or as the other card of
	// their word was reviewed today, come tomorrow.
	if int(counts.Unseen) > newToday || int(counts.RecallWaiting) > recallToday {
		date := tomorrow.Format(time.DateOnly)
		if q.NextDueOn == nil || date < *q.NextDueOn {
			q.NextDueOn = &date
		}
	}
	return q, nil
}

// reviewCountsParams asks for the student's counts on day.
func reviewCountsParams(ownerID string, day localDay) store.GetReviewCountsParams {
	return store.GetReviewCountsParams{
		OwnerID: ownerID, Today: day.date(), DayStart: day.start, MinRepetitions: recallAfter,
	}
}

// newWordsLeft is how many new words can still be learned today.
func newWordsLeft(c store.GetReviewCountsRow) int {
	return max(0, min(newWordsPerDay-int(c.StartedToday), int(c.Unseen)))
}

// newRecallLeft is how many words can still get their first English →
// Serbian card today.
func newRecallLeft(c store.GetReviewCountsRow) int {
	return max(0, min(newRecallPerDay-int(c.RecallStartedToday), int(c.RecallReady)))
}

// Record saves the answer to one of the current student's cards and
// schedules the card's next review.
func (s *ReviewService) Record(ctx context.Context, entryID string, in ReviewInput) (ReviewResult, error) {
	actor, err := access.RequireStudent(ctx)
	if err != nil {
		return ReviewResult{}, err
	}
	if !isUUID(entryID) {
		return ReviewResult{}, errEntryNotFound
	}
	fields := map[string]string{}
	if !in.Direction.valid() {
		fields["direction"] = "Choose sr_en or en_sr."
	}
	if !in.Rating.valid() {
		fields["rating"] = "Choose again, hard, good or easy."
	}
	if len(fields) > 0 {
		return ReviewResult{}, apperr.Validation(fields)
	}
	day, err := currentDay(in.TimeZone, s.now())
	if err != nil {
		return ReviewResult{}, err
	}
	ownerID := actor.User.ID

	_, err = s.store.GetVocabularyEntry(ctx, store.GetVocabularyEntryParams{ID: entryID, OwnerID: ownerID})
	if errors.Is(err, pgx.ErrNoRows) {
		return ReviewResult{}, errEntryNotFound
	}
	if err != nil {
		return ReviewResult{}, fmt.Errorf("get vocabulary entry: %w", err)
	}

	current := newCard
	var readReviewedAt *time.Time
	card, err := s.store.GetReviewCard(ctx, store.GetReviewCardParams{
		EntryID: entryID, Direction: store.ReviewDirection(in.Direction), OwnerID: ownerID,
	})
	switch {
	case errors.Is(err, pgx.ErrNoRows):
	case err != nil:
		return ReviewResult{}, fmt.Errorf("get review card: %w", err)
	default:
		current = schedule{
			IntervalDays: int(card.IntervalDays), EasePercent: int(card.EasePercent),
			Repetitions: int(card.Repetitions), Lapses: int(card.Lapses),
		}
		readReviewedAt = &card.LastReviewedAt
	}

	next := current.next(in.Rating)
	dueOn := day.date().AddDate(0, 0, next.IntervalDays)
	_, err = s.store.RecordReview(ctx, store.RecordReviewParams{
		EntryID:        entryID,
		Direction:      store.ReviewDirection(in.Direction),
		OwnerID:        ownerID,
		DueOn:          dueOn,
		IntervalDays:   int32(next.IntervalDays),
		EasePercent:    int32(next.EasePercent),
		Repetitions:    int32(next.Repetitions),
		Lapses:         int32(next.Lapses),
		ReadReviewedAt: readReviewedAt,
		Rating:         store.ReviewRating(in.Rating),
	})
	switch {
	case errors.Is(err, pgx.ErrNoRows):
		return ReviewResult{}, errReviewConflict
	// Deleted since it was read.
	case isForeignKeyViolation(err, reviewCardEntryFkey), isForeignKeyViolation(err, reviewEntryFkey):
		return ReviewResult{}, errEntryNotFound
	case err != nil:
		return ReviewResult{}, fmt.Errorf("record review: %w", err)
	}
	return ReviewResult{
		EntryID:      entryID,
		Direction:    in.Direction,
		Rating:       in.Rating,
		DueOn:        dueOn.Format(time.DateOnly),
		IntervalDays: next.IntervalDays,
	}, nil
}

const (
	reviewCardEntryFkey = "review_cards_entry_fkey"
	reviewEntryFkey     = "reviews_entry_fkey"
)

func (d Direction) valid() bool {
	return d == SerbianToEnglish || d == EnglishToSerbian
}

func (r Rating) valid() bool {
	switch r {
	case RatingAgain, RatingHard, RatingGood, RatingEasy:
		return true
	}
	return false
}

// schedule is where a word stands in its reviews.
type schedule struct {
	IntervalDays int // days until the word comes back
	EasePercent  int // how much a "good" answer stretches the interval
	Repetitions  int // answers in a row other than "again"
	Lapses       int // times the word was forgotten after being learned
}

const (
	startEase = 250
	minEase   = 130
	// maxInterval matches review_cards_interval_range: every word comes back
	// at least once a year.
	maxInterval = 365
)

// newCard is a word never reviewed.
var newCard = schedule{EasePercent: startEase}

// next is the schedule after an answer, a simplified SM-2 as Anki uses it,
// counted in whole days:
//
//   - again: the word was forgotten and comes back today; its interval
//     starts over.
//   - A word not yet learned (new, or forgotten last time) comes back in
//     1, 2 or 4 days for hard, good or easy.
//   - Once learned, hard stretches the interval by 1.2, good by the ease
//     (2.5 to start with) and easy by the ease and 1.3, each at least a day
//     more than the one before, so the buttons never tie.
//   - Again and hard on a learned word lower its ease and easy raises it,
//     so a word that keeps being hard comes back more often.
func (c schedule) next(r Rating) schedule {
	learned := c.Repetitions > 0
	if r == RatingAgain {
		if learned {
			c.Lapses++
			c.EasePercent = max(minEase, c.EasePercent-20)
		}
		c.Repetitions = 0
		c.IntervalDays = 0
		return c
	}

	var interval int
	if !learned {
		interval = map[Rating]int{RatingHard: 1, RatingGood: 2, RatingEasy: 4}[r]
	} else {
		prev := float64(c.IntervalDays)
		ease := float64(c.EasePercent) / 100
		hard := max(c.IntervalDays+1, int(math.Round(prev*1.2)))
		good := max(hard+1, int(math.Round(prev*ease)))
		easy := max(good+1, int(math.Round(prev*ease*1.3)))
		switch r {
		case RatingHard:
			interval = hard
			c.EasePercent = max(minEase, c.EasePercent-15)
		case RatingGood:
			interval = good
		case RatingEasy:
			interval = easy
			c.EasePercent += 15
		}
	}
	c.Repetitions++
	c.IntervalDays = min(interval, maxInterval)
	return c
}

func (c schedule) nextIntervals() NextIntervals {
	return NextIntervals{
		Again: c.next(RatingAgain).IntervalDays,
		Hard:  c.next(RatingHard).IntervalDays,
		Good:  c.next(RatingGood).IntervalDays,
		Easy:  c.next(RatingEasy).IntervalDays,
	}
}
