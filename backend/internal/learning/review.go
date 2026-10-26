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

// NextIntervals is how many days away each answer would put a word, shown
// on the answer buttons.
type NextIntervals struct {
	Again int
	Hard  int
	Good  int
	Easy  int
}

// ReviewCard is a word to review with what each answer would do.
type ReviewCard struct {
	Entry         VocabularyEntry
	New           bool // never reviewed before
	NextIntervals NextIntervals
}

// ReviewQueue is what a student has to review today, in the order to show
// it: words due first, longest overdue first, then new words, oldest first.
type ReviewQueue struct {
	Today string
	Cards []ReviewCard // at most reviewBatch; ask again once they are answered
	Due   int          // words due today in all, Cards included
	New   int          // new words to learn today in all, Cards included
	// NextDueOn is the first day after today with words to review, or nil
	// when there are no words at all.
	NextDueOn *string
}

// ReviewInput is an answer to one card.
type ReviewInput struct {
	Rating   Rating
	TimeZone string // see DashboardService.Get
}

// ReviewResult is when a word comes back after an answer.
type ReviewResult struct {
	EntryID      string
	Rating       Rating
	DueOn        string
	IntervalDays int
}

const (
	// newWordsPerDay keeps a long list of words from arriving all at once.
	newWordsPerDay = 20
	// reviewBatch is how many cards one request returns.
	reviewBatch = 50
)

// ReviewStore is the subset of generated queries flashcards need.
type ReviewStore interface {
	GetReviewCounts(ctx context.Context, arg store.GetReviewCountsParams) (store.GetReviewCountsRow, error)
	ListDueReviewCards(ctx context.Context, arg store.ListDueReviewCardsParams) ([]store.ListDueReviewCardsRow, error)
	ListNewReviewEntries(ctx context.Context, arg store.ListNewReviewEntriesParams) ([]store.VocabularyEntry, error)
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
	"This word was just reviewed somewhere else. Go on with the next one.")

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

	counts, err := s.store.GetReviewCounts(ctx, store.GetReviewCountsParams{
		OwnerID: ownerID, Today: today, DayStart: day.start,
	})
	if err != nil {
		return ReviewQueue{}, fmt.Errorf("get review counts: %w", err)
	}
	newToday := newWordsLeft(counts)

	dueRows, err := s.store.ListDueReviewCards(ctx, store.ListDueReviewCardsParams{
		OwnerID: ownerID, Today: today, RowLimit: reviewBatch,
	})
	if err != nil {
		return ReviewQueue{}, fmt.Errorf("list due review cards: %w", err)
	}
	var newRows []store.VocabularyEntry
	if room := min(newToday, reviewBatch-len(dueRows)); room > 0 {
		newRows, err = s.store.ListNewReviewEntries(ctx, store.ListNewReviewEntriesParams{
			OwnerID: ownerID, RowLimit: int32(room),
		})
		if err != nil {
			return ReviewQueue{}, fmt.Errorf("list new review entries: %w", err)
		}
	}

	entries := make([]VocabularyEntry, 0, len(dueRows)+len(newRows))
	schedules := make([]schedule, 0, cap(entries))
	for _, r := range dueRows {
		entries = append(entries, entryFromRow(r.VocabularyEntry))
		schedules = append(schedules, schedule{
			IntervalDays: int(r.IntervalDays), EasePercent: int(r.EasePercent),
			Repetitions: int(r.Repetitions), Lapses: int(r.Lapses),
		})
	}
	for _, r := range newRows {
		entries = append(entries, entryFromRow(r))
		schedules = append(schedules, newCard)
	}
	if err := addSources(ctx, s.store, ownerID, entries); err != nil {
		return ReviewQueue{}, err
	}

	q := ReviewQueue{
		Today: today.Format(time.DateOnly),
		Cards: make([]ReviewCard, len(entries)),
		Due:   int(counts.Due),
		New:   newToday,
	}
	for i, e := range entries {
		q.Cards[i] = ReviewCard{Entry: e, New: i >= len(dueRows), NextIntervals: schedules[i].nextIntervals()}
	}

	next, err := s.store.GetNextReviewDate(ctx, store.GetNextReviewDateParams{OwnerID: ownerID, Today: today})
	switch {
	case errors.Is(err, pgx.ErrNoRows):
	case err != nil:
		return ReviewQueue{}, fmt.Errorf("get next review date: %w", err)
	default:
		date := next.Format(time.DateOnly)
		q.NextDueOn = &date
	}
	// New words left over for lack of room today come tomorrow.
	if int(counts.Unseen) > newToday {
		tomorrow := today.AddDate(0, 0, 1).Format(time.DateOnly)
		if q.NextDueOn == nil || tomorrow < *q.NextDueOn {
			q.NextDueOn = &tomorrow
		}
	}
	return q, nil
}

// newWordsLeft is how many new words can still be learned today.
func newWordsLeft(c store.GetReviewCountsRow) int {
	return max(0, min(newWordsPerDay-int(c.StartedToday), int(c.Unseen)))
}

// Record saves the answer to one of the current student's words and
// schedules the word's next review.
func (s *ReviewService) Record(ctx context.Context, entryID string, in ReviewInput) (ReviewResult, error) {
	actor, err := access.RequireStudent(ctx)
	if err != nil {
		return ReviewResult{}, err
	}
	if !isUUID(entryID) {
		return ReviewResult{}, errEntryNotFound
	}
	if !in.Rating.valid() {
		return ReviewResult{}, apperr.Validation(map[string]string{"rating": "Choose again, hard, good or easy."})
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
	card, err := s.store.GetReviewCard(ctx, store.GetReviewCardParams{EntryID: entryID, OwnerID: ownerID})
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
		Rating:       in.Rating,
		DueOn:        dueOn.Format(time.DateOnly),
		IntervalDays: next.IntervalDays,
	}, nil
}

const (
	reviewCardEntryFkey = "review_cards_entry_fkey"
	reviewEntryFkey     = "reviews_entry_fkey"
)

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
