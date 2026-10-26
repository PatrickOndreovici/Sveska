package httpapi

import (
	"context"
	"encoding/json"
	"net/http"
	"testing"

	"learning-serbian-platform/backend/internal/apperr"
	"learning-serbian-platform/backend/internal/learning"
)

// fakeDashboard answers with a fixed dashboard, or err when set.
type fakeDashboard struct {
	err          error
	calls        int
	lastTimeZone string
}

func (f *fakeDashboard) Get(_ context.Context, timeZone string) (learning.Dashboard, error) {
	f.calls++
	f.lastTimeZone = timeZone
	return learning.Dashboard{
		Today:    "2026-10-23",
		Totals:   learning.Totals{Words: 40, WordsThisWeek: 5, Lessons: 3, Documents: 5, Highlights: 12, Notes: 4},
		Review:   learning.ReviewCounts{Due: 7, New: 5},
		Streak:   2,
		Activity: []learning.ActivityDay{{Date: "2026-10-22", Words: 1}, {Date: "2026-10-23", Highlights: 2, Pages: 1, Reviews: 9}},
		RecentWords: []learning.RecentWord{
			{ID: entryID, Term: "kuća", Translation: "house", CreatedAt: testTime},
		},
		RecentDocuments: []learning.RecentDocument{{
			ID: documentID, Title: "Čitanka", LessonID: lessonID, LessonTitle: "Kuća", PageNumber: 4, WorkedAt: testTime,
		}},
	}, f.err
}

func newDashboardRouter(d *fakeDashboard) http.Handler {
	return NewRouter(Dependencies{DB: fakePinger{}, Auth: &fakeAuth{}, Dashboard: d, AppOrigin: testOrigin, CookieSecure: true})
}

func TestDashboardRequiresSession(t *testing.T) {
	d := &fakeDashboard{}
	rec := do(t, newDashboardRouter(d), http.MethodGet, "/api/dashboard", requestOpts{})
	if rec.Code != http.StatusUnauthorized || d.calls != 0 {
		t.Errorf("status = %d, service calls = %d; want 401 and 0", rec.Code, d.calls)
	}
}

func TestDashboardResponse(t *testing.T) {
	d := &fakeDashboard{}
	rec := do(t, newDashboardRouter(d), http.MethodGet, "/api/dashboard?timeZone=Europe/Belgrade", loggedIn(""))

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", rec.Code, rec.Body)
	}
	if d.lastTimeZone != "Europe/Belgrade" {
		t.Errorf("time zone = %q", d.lastTimeZone)
	}
	var got map[string]any
	if err := json.NewDecoder(rec.Body).Decode(&got); err != nil {
		t.Fatal(err)
	}
	want := map[string]any{
		"today": "2026-10-23",
		"totals": map[string]any{
			"words": 40.0, "wordsThisWeek": 5.0, "lessons": 3.0, "documents": 5.0, "highlights": 12.0, "notes": 4.0,
		},
		"review": map[string]any{"due": 7.0, "new": 5.0},
		"streak": 2.0,
		"activity": []any{
			map[string]any{"date": "2026-10-22", "words": 1.0, "highlights": 0.0, "pages": 0.0, "reviews": 0.0},
			map[string]any{"date": "2026-10-23", "words": 0.0, "highlights": 2.0, "pages": 1.0, "reviews": 9.0},
		},
		"recentWords": []any{
			map[string]any{"id": entryID, "term": "kuća", "translation": "house", "createdAt": "2026-10-23T12:00:00Z"},
		},
		"recentDocuments": []any{map[string]any{
			"id": documentID, "title": "Čitanka", "lesson": map[string]any{"id": lessonID, "title": "Kuća"},
			"pageNumber": 4.0, "workedAt": "2026-10-23T12:00:00Z",
		}},
	}
	gotJSON, _ := json.Marshal(got)
	wantJSON, _ := json.Marshal(want)
	if string(gotJSON) != string(wantJSON) {
		t.Errorf("body =\n%s\nwant\n%s", gotJSON, wantJSON)
	}
}

func TestDashboardErrors(t *testing.T) {
	tests := []struct {
		err    error
		status int
	}{
		{apperr.Validation(map[string]string{"timeZone": "Use a time zone name such as Europe/Belgrade."}), http.StatusUnprocessableEntity},
		{apperr.New(apperr.Forbidden, "students_only", "This is only available to student accounts."), http.StatusForbidden},
	}
	for _, tt := range tests {
		rec := do(t, newDashboardRouter(&fakeDashboard{err: tt.err}), http.MethodGet, "/api/dashboard?timeZone=x", loggedIn(""))
		if rec.Code != tt.status {
			t.Errorf("%v: status = %d, want %d", tt.err, rec.Code, tt.status)
		}
	}
}
