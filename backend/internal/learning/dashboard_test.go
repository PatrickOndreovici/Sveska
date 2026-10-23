package learning

import (
	"context"
	"errors"
	"testing"
	"time"

	"learning-serbian-platform/backend/internal/apperr"
	"learning-serbian-platform/backend/internal/store"
)

// fakeDashboardStore answers with fixed rows and remembers what it was asked.
type fakeDashboardStore struct {
	totals    store.GetDashboardTotalsRow
	days      []store.ListActivityDaysRow
	words     []store.ListRecentVocabularyEntriesRow
	documents []store.ListRecentDocumentsRow

	ownerIDs     []string
	activityArgs store.ListActivityDaysParams
	wordLimit    int32
	docLimit     int32
}

func (f *fakeDashboardStore) GetDashboardTotals(_ context.Context, ownerID string) (store.GetDashboardTotalsRow, error) {
	f.ownerIDs = append(f.ownerIDs, ownerID)
	return f.totals, nil
}

func (f *fakeDashboardStore) ListActivityDays(_ context.Context, arg store.ListActivityDaysParams) ([]store.ListActivityDaysRow, error) {
	f.ownerIDs = append(f.ownerIDs, arg.OwnerID)
	f.activityArgs = arg
	return f.days, nil
}

func (f *fakeDashboardStore) ListRecentVocabularyEntries(_ context.Context, arg store.ListRecentVocabularyEntriesParams) ([]store.ListRecentVocabularyEntriesRow, error) {
	f.ownerIDs = append(f.ownerIDs, arg.OwnerID)
	f.wordLimit = arg.RowLimit
	return f.words, nil
}

func (f *fakeDashboardStore) ListRecentDocuments(_ context.Context, arg store.ListRecentDocumentsParams) ([]store.ListRecentDocumentsRow, error) {
	f.ownerIDs = append(f.ownerIDs, arg.OwnerID)
	f.docLimit = arg.RowLimit
	return f.documents, nil
}

// newDashboard is a service whose clock reads 23 October 2026, 22:30 UTC,
// which is already 24 October in Belgrade (UTC+2 in October).
func newDashboard(f *fakeDashboardStore) *DashboardService {
	svc := NewDashboardService(f)
	svc.now = func() time.Time { return time.Date(2026, 10, 23, 22, 30, 0, 0, time.UTC) }
	return svc
}

func TestDashboardCountsDaysInTheStudentsTimeZone(t *testing.T) {
	f := &fakeDashboardStore{}
	d, err := newDashboard(f).Get(actorCtx(anaID), "Europe/Belgrade")
	if err != nil {
		t.Fatal(err)
	}
	if d.Today != "2026-10-24" {
		t.Errorf("today = %s, want 2026-10-24", d.Today)
	}
	if f.activityArgs.TimeZone != "Europe/Belgrade" {
		t.Errorf("time zone = %q", f.activityArgs.TimeZone)
	}
	// 366 days back from Belgrade's midnight on 24 October.
	belgrade, _ := time.LoadLocation("Europe/Belgrade")
	if want := time.Date(2025, 10, 24, 0, 0, 0, 0, belgrade); !f.activityArgs.Since.Equal(want) {
		t.Errorf("since = %v, want %v", f.activityArgs.Since, want)
	}
	for _, id := range f.ownerIDs {
		if id != anaID {
			t.Errorf("queried owner %s, want %s", id, anaID)
		}
	}
}

func TestDashboardWithoutATimeZoneUsesUTC(t *testing.T) {
	f := &fakeDashboardStore{}
	d, err := newDashboard(f).Get(actorCtx(anaID), "")
	if err != nil {
		t.Fatal(err)
	}
	if d.Today != "2026-10-23" || f.activityArgs.TimeZone != "UTC" {
		t.Errorf("today = %s in %q, want 2026-10-23 in UTC", d.Today, f.activityArgs.TimeZone)
	}
}

func TestDashboardRejectsUnknownTimeZones(t *testing.T) {
	for _, tz := range []string{"Mars/Olympus", "Local", "../../etc/passwd", "Europe/Belgrade'; --", "UTC+3"} {
		_, err := newDashboard(&fakeDashboardStore{}).Get(actorCtx(anaID), tz)
		var appErr *apperr.Error
		if !errors.As(err, &appErr) || appErr.Fields["timeZone"] == "" {
			t.Errorf("%q: err = %v, want a timeZone problem", tz, err)
		}
	}
}

func TestDashboardFillsEveryDayOfTheChart(t *testing.T) {
	f := &fakeDashboardStore{days: []store.ListActivityDaysRow{
		{Day: "2026-07-31", Words: 9}, // before the chart: only the streak looks this far
		{Day: "2026-10-20", Words: 2, Highlights: 1},
		{Day: "2026-10-23", Pages: 3},
	}}
	d, err := newDashboard(f).Get(actorCtx(anaID), "UTC")
	if err != nil {
		t.Fatal(err)
	}
	if len(d.Activity) != ActivityDays {
		t.Fatalf("%d days, want %d", len(d.Activity), ActivityDays)
	}
	if first, last := d.Activity[0].Date, d.Activity[ActivityDays-1].Date; first != "2026-08-01" || last != "2026-10-23" {
		t.Errorf("chart runs %s to %s, want 2026-08-01 to 2026-10-23", first, last)
	}
	want := map[string]ActivityDay{
		"2026-08-01": {Date: "2026-08-01"},
		"2026-10-20": {Date: "2026-10-20", Words: 2, Highlights: 1},
		"2026-10-22": {Date: "2026-10-22"},
		"2026-10-23": {Date: "2026-10-23", Pages: 3},
	}
	for _, day := range d.Activity {
		if w, ok := want[day.Date]; ok && day != w {
			t.Errorf("%s = %+v, want %+v", day.Date, day, w)
		}
	}
}

func TestDashboardTotals(t *testing.T) {
	f := &fakeDashboardStore{
		totals: store.GetDashboardTotalsRow{Words: 40, Lessons: 3, Documents: 5, Highlights: 12, Notes: 4},
		days: []store.ListActivityDaysRow{
			{Day: "2026-10-16", Words: 10}, // 8 days ago: not this week
			{Day: "2026-10-17", Words: 3},  // 7 days ago counting today as the first: this week
			{Day: "2026-10-23", Words: 2},
		},
	}
	d, err := newDashboard(f).Get(actorCtx(anaID), "UTC")
	if err != nil {
		t.Fatal(err)
	}
	want := Totals{Words: 40, WordsThisWeek: 5, Lessons: 3, Documents: 5, Highlights: 12, Notes: 4}
	if d.Totals != want {
		t.Errorf("totals = %+v, want %+v", d.Totals, want)
	}
}

func TestStreak(t *testing.T) {
	today := time.Date(2026, 10, 23, 0, 0, 0, 0, time.UTC)
	days := func(dates ...string) map[string]ActivityDay {
		m := map[string]ActivityDay{}
		for _, d := range dates {
			m[d] = ActivityDay{Date: d, Highlights: 1}
		}
		return m
	}
	tests := []struct {
		name string
		days map[string]ActivityDay
		want int
	}{
		{"nothing done", days(), 0},
		{"only today", days("2026-10-23"), 1},
		{"up to today", days("2026-10-21", "2026-10-22", "2026-10-23"), 3},
		{"up to yesterday, today not done yet", days("2026-10-21", "2026-10-22"), 2},
		{"broken two days ago", days("2026-10-20", "2026-10-21"), 0},
		{"a gap stops it", days("2026-10-19", "2026-10-21", "2026-10-22", "2026-10-23"), 3},
		{"across a month", days("2026-09-30", "2026-10-01"), 0},
		{"an empty row is not activity", map[string]ActivityDay{"2026-10-23": {Date: "2026-10-23"}}, 0},
	}
	for _, tt := range tests {
		if got := streak(tt.days, today); got != tt.want {
			t.Errorf("%s: streak = %d, want %d", tt.name, got, tt.want)
		}
	}

	longRun := map[string]ActivityDay{}
	for i := range 40 {
		date := today.AddDate(0, 0, -i).Format(time.DateOnly)
		longRun[date] = ActivityDay{Date: date, Words: 1}
	}
	if got := streak(longRun, today); got != 40 {
		t.Errorf("40 days: streak = %d", got)
	}
}

func TestDashboardStreakUsesTheWholeYear(t *testing.T) {
	// 100 days in a row, longer than the chart.
	f := &fakeDashboardStore{}
	today := time.Date(2026, 10, 23, 0, 0, 0, 0, time.UTC)
	for i := range 100 {
		f.days = append(f.days, store.ListActivityDaysRow{Day: today.AddDate(0, 0, -i).Format(time.DateOnly), Pages: 1})
	}
	d, err := newDashboard(f).Get(actorCtx(anaID), "UTC")
	if err != nil {
		t.Fatal(err)
	}
	if d.Streak != 100 {
		t.Errorf("streak = %d, want 100", d.Streak)
	}
}

func TestDashboardRecentWordsAndDocuments(t *testing.T) {
	at := time.Date(2026, 10, 22, 9, 0, 0, 0, time.UTC)
	f := &fakeDashboardStore{
		words: []store.ListRecentVocabularyEntriesRow{{ID: "w1", Term: "kuća", Translation: "house", CreatedAt: at}},
		documents: []store.ListRecentDocumentsRow{{
			ID: "d1", Title: "Lekcija 3", LessonID: "l1", LessonTitle: "Kuća", PageNumber: 4, WorkedAt: at,
		}},
	}
	d, err := newDashboard(f).Get(actorCtx(anaID), "UTC")
	if err != nil {
		t.Fatal(err)
	}
	if f.wordLimit != recentWords || f.docLimit != recentDocuments {
		t.Errorf("limits = %d words, %d documents", f.wordLimit, f.docLimit)
	}
	if len(d.RecentWords) != 1 || d.RecentWords[0] != (RecentWord{ID: "w1", Term: "kuća", Translation: "house", CreatedAt: at}) {
		t.Errorf("words = %+v", d.RecentWords)
	}
	wantDoc := RecentDocument{ID: "d1", Title: "Lekcija 3", LessonID: "l1", LessonTitle: "Kuća", PageNumber: 4, WorkedAt: at}
	if len(d.RecentDocuments) != 1 || d.RecentDocuments[0] != wantDoc {
		t.Errorf("documents = %+v", d.RecentDocuments)
	}
}

func TestDashboardIsEmptyNotNilForANewStudent(t *testing.T) {
	d, err := newDashboard(&fakeDashboardStore{}).Get(actorCtx(anaID), "UTC")
	if err != nil {
		t.Fatal(err)
	}
	if d.RecentWords == nil || d.RecentDocuments == nil || d.Streak != 0 {
		t.Errorf("dashboard = %+v", d)
	}
}

func TestDashboardIsForStudentsOnly(t *testing.T) {
	f := &fakeDashboardStore{}
	_, err := newDashboard(f).Get(teacherCtx(), "UTC")
	var appErr *apperr.Error
	if !errors.As(err, &appErr) || appErr.Kind != apperr.Forbidden {
		t.Errorf("err = %v, want forbidden", err)
	}
	if _, err := newDashboard(f).Get(context.Background(), "UTC"); err == nil {
		t.Error("no actor: want an error")
	}
	if len(f.ownerIDs) != 0 {
		t.Error("the store was queried")
	}
}
