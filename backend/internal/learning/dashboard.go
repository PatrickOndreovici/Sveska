package learning

import (
	"context"
	"fmt"
	"time"

	"learning-serbian-platform/backend/internal/access"
	"learning-serbian-platform/backend/internal/store"
)

// Dashboard is a student's overview: what they have, what they did on each
// recent day and where to pick up. Days are dates in the student's own time
// zone, as YYYY-MM-DD.
type Dashboard struct {
	Today           string
	Totals          Totals
	Review          ReviewCounts
	Streak          int              // days in a row with something done, see streak
	Activity        []ActivityDay    // the last ActivityDays days, oldest first, today last
	RecentWords     []RecentWord     // newest first
	RecentDocuments []RecentDocument // most recently worked on first
}

type Totals struct {
	Words         int
	WordsThisWeek int // added in the last 7 days, today included
	Lessons       int
	Documents     int
	Highlights    int
	Notes         int // highlights with a note
}

// ReviewCounts is what the student has to review today, as in ReviewQueue.
type ReviewCounts struct {
	Due int
	New int
}

// ActivityDay is what a student did on one day. Pages counts the PDF pages
// they drew or wrote on, each once however much they did on it; Reviews
// counts flashcard answers.
type ActivityDay struct {
	Date       string
	Words      int
	Highlights int
	Pages      int
	Reviews    int
}

// total is everything done on the day.
func (d ActivityDay) total() int {
	return d.Words + d.Highlights + d.Pages + d.Reviews
}

type RecentWord struct {
	ID          string
	Term        string
	Translation string
	CreatedAt   time.Time
}

// RecentDocument is a PDF with when the student last worked on it (uploaded,
// highlighted, drew or wrote on it) and the page of that change.
type RecentDocument struct {
	ID          string
	Title       string
	LessonID    string
	LessonTitle string
	PageNumber  int
	WorkedAt    time.Time
}

const (
	// ActivityDays is the chart's span: twelve weeks.
	ActivityDays = 84
	// streakDays is how far back a streak is counted.
	streakDays      = 366
	recentWords     = 5
	recentDocuments = 3
)

// DashboardStore is the subset of generated queries the dashboard needs.
type DashboardStore interface {
	GetDashboardTotals(ctx context.Context, ownerID string) (store.GetDashboardTotalsRow, error)
	ListActivityDays(ctx context.Context, arg store.ListActivityDaysParams) ([]store.ListActivityDaysRow, error)
	ListRecentVocabularyEntries(ctx context.Context, arg store.ListRecentVocabularyEntriesParams) ([]store.ListRecentVocabularyEntriesRow, error)
	ListRecentDocuments(ctx context.Context, arg store.ListRecentDocumentsParams) ([]store.ListRecentDocumentsRow, error)
	GetReviewCounts(ctx context.Context, arg store.GetReviewCountsParams) (store.GetReviewCountsRow, error)
}

// DashboardService builds a student's dashboard.
type DashboardService struct {
	store DashboardStore
	now   func() time.Time
}

func NewDashboardService(s DashboardStore) *DashboardService {
	return &DashboardService{store: s, now: time.Now}
}

// Get returns the student's dashboard, counting days in timeZone (an IANA
// name; empty means UTC), so that a day starts at the student's midnight.
func (s *DashboardService) Get(ctx context.Context, timeZone string) (Dashboard, error) {
	actor, err := access.RequireStudent(ctx)
	if err != nil {
		return Dashboard{}, err
	}
	day, err := currentDay(timeZone, s.now())
	if err != nil {
		return Dashboard{}, err
	}
	ownerID := actor.User.ID
	today := day.start

	totals, err := s.store.GetDashboardTotals(ctx, ownerID)
	if err != nil {
		return Dashboard{}, fmt.Errorf("get dashboard totals: %w", err)
	}
	dayRows, err := s.store.ListActivityDays(ctx, store.ListActivityDaysParams{
		OwnerID:  ownerID,
		Since:    today.AddDate(0, 0, -(streakDays - 1)),
		TimeZone: day.timeZone,
	})
	if err != nil {
		return Dashboard{}, fmt.Errorf("list activity days: %w", err)
	}
	wordRows, err := s.store.ListRecentVocabularyEntries(ctx, store.ListRecentVocabularyEntriesParams{
		OwnerID:  ownerID,
		RowLimit: recentWords,
	})
	if err != nil {
		return Dashboard{}, fmt.Errorf("list recent vocabulary entries: %w", err)
	}
	documentRows, err := s.store.ListRecentDocuments(ctx, store.ListRecentDocumentsParams{
		OwnerID:  ownerID,
		RowLimit: recentDocuments,
	})
	if err != nil {
		return Dashboard{}, fmt.Errorf("list recent documents: %w", err)
	}
	reviewCounts, err := s.store.GetReviewCounts(ctx, store.GetReviewCountsParams{
		OwnerID: ownerID, Today: day.date(), DayStart: day.start,
	})
	if err != nil {
		return Dashboard{}, fmt.Errorf("get review counts: %w", err)
	}

	byDate := make(map[string]ActivityDay, len(dayRows))
	for _, r := range dayRows {
		byDate[r.Day] = ActivityDay{
			Date: r.Day, Words: int(r.Words), Highlights: int(r.Highlights), Pages: int(r.Pages), Reviews: int(r.Reviews),
		}
	}
	activity := make([]ActivityDay, ActivityDays)
	for i := range activity {
		date := today.AddDate(0, 0, i-(ActivityDays-1)).Format(time.DateOnly)
		activity[i] = byDate[date]
		activity[i].Date = date
	}
	wordsThisWeek := 0
	for _, d := range activity[ActivityDays-7:] {
		wordsThisWeek += d.Words
	}

	d := Dashboard{
		Today: today.Format(time.DateOnly),
		Totals: Totals{
			Words:         int(totals.Words),
			WordsThisWeek: wordsThisWeek,
			Lessons:       int(totals.Lessons),
			Documents:     int(totals.Documents),
			Highlights:    int(totals.Highlights),
			Notes:         int(totals.Notes),
		},
		Review:          ReviewCounts{Due: int(reviewCounts.Due), New: newWordsLeft(reviewCounts)},
		Streak:          streak(byDate, today),
		Activity:        activity,
		RecentWords:     make([]RecentWord, 0, len(wordRows)),
		RecentDocuments: make([]RecentDocument, 0, len(documentRows)),
	}
	for _, r := range wordRows {
		d.RecentWords = append(d.RecentWords, RecentWord{
			ID: r.ID, Term: r.Term, Translation: r.Translation, CreatedAt: r.CreatedAt,
		})
	}
	for _, r := range documentRows {
		d.RecentDocuments = append(d.RecentDocuments, RecentDocument{
			ID: r.ID, Title: r.Title, LessonID: r.LessonID, LessonTitle: r.LessonTitle,
			PageNumber: int(r.PageNumber), WorkedAt: r.WorkedAt,
		})
	}
	return d, nil
}

// streak counts the days in a row on which something was done, up to today.
// A streak that reached yesterday still counts while nothing has been done
// today: the day is not over yet.
func streak(days map[string]ActivityDay, today time.Time) int {
	active := func(t time.Time) bool {
		return days[t.Format(time.DateOnly)].total() > 0
	}
	day := today
	if !active(day) {
		day = day.AddDate(0, 0, -1)
	}
	n := 0
	for active(day) {
		n++
		day = day.AddDate(0, 0, -1)
	}
	return n
}
