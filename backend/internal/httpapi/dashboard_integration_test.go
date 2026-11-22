//go:build integration

package httpapi

import (
	"context"
	"net/http"
	"testing"
	"time"
)

// These tests run the dashboard's queries against PostgreSQL: the counts,
// the days in the student's time zone and the order of recent work.

// activityOn returns the chart's entry for date, failing if there is none.
func activityOn(t *testing.T, dashboard map[string]any, date string) map[string]any {
	t.Helper()
	for _, d := range dashboard["activity"].([]any) {
		if day := d.(map[string]any); day["date"] == date {
			return day
		}
	}
	t.Fatalf("no activity entry for %s", date)
	return nil
}

func TestDashboardCountsAStudentsWork(t *testing.T) {
	srv, pool := newStack(t)
	ana := signedUp(t, newBrowser(t, srv), "ana@example.com")
	lessonID := ana.createLesson("Lekcija 1")
	reader := ana.uploadPDF(lessonID, "Čitanka.pdf")
	workbook := ana.uploadPDF(lessonID, "Radna sveska.pdf")

	kuca := ana.addWord(map[string]string{"term": "kuća", "translation": "house"})
	ana.addWord(map[string]string{"term": "voda", "translation": "water"})
	ana.annotate(reader, highlight(1, "Dobar dan"))
	noted := ana.annotate(reader, highlight(2, "Zdravo"))
	if resp, _ := ana.send(http.MethodPut, "/api/annotations/"+noted, map[string]string{"note": "hello"}); resp.StatusCode != http.StatusOK {
		t.Fatalf("add note = %d", resp.StatusCode)
	}
	// Two pages drawn on: the workbook's page 1, then three changes on the reader's page 3.
	ana.create("/api/documents/"+workbook+"/strokes", "stroke", stroke(1, "red"))
	ana.create("/api/documents/"+reader+"/strokes", "stroke", stroke(3, "blue"))
	ana.create("/api/documents/"+reader+"/strokes", "stroke", stroke(3, "blue"))
	ana.create("/api/documents/"+reader+"/text-boxes", "textBox", textBox(3, "kuća"))

	// kuća was added half an hour after midnight in Belgrade, three days ago:
	// still the previous day in UTC.
	belgrade, err := time.LoadLocation("Europe/Belgrade")
	if err != nil {
		t.Fatal(err)
	}
	now := time.Now().In(belgrade)
	addedAt := time.Date(now.Year(), now.Month(), now.Day()-3, 0, 30, 0, 0, belgrade)
	if _, err := pool.Exec(context.Background(), "UPDATE vocabulary_entries SET created_at = $1 WHERE id = $2", addedAt, kuca); err != nil {
		t.Fatal(err)
	}

	resp, d := ana.send(http.MethodGet, "/api/dashboard?timeZone=Europe/Belgrade", nil)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("dashboard = %d %v", resp.StatusCode, d)
	}
	today := now.Format(time.DateOnly)
	if d["today"] != today {
		t.Errorf("today = %v, want %s", d["today"], today)
	}
	totals := d["totals"].(map[string]any)
	wantTotals := map[string]float64{"words": 2, "wordsThisWeek": 2, "lessons": 1, "documents": 2, "highlights": 2, "notes": 1}
	for k, want := range wantTotals {
		if totals[k] != want {
			t.Errorf("totals.%s = %v, want %v", k, totals[k], want)
		}
	}
	if days := d["activity"].([]any); len(days) != 84 {
		t.Errorf("%d days of activity, want 84", len(days))
	}
	todays := activityOn(t, d, today)
	if todays["words"] != 1.0 || todays["highlights"] != 2.0 || todays["pages"] != 2.0 {
		t.Errorf("today's activity = %v, want 1 word, 2 highlights, 2 pages", todays)
	}
	if day := activityOn(t, d, addedAt.Format(time.DateOnly)); day["words"] != 1.0 {
		t.Errorf("activity in Belgrade on %s = %v, want kuća", addedAt.Format(time.DateOnly), day)
	}
	// Nothing was done yesterday, so the streak is today alone.
	if d["streak"] != 1.0 {
		t.Errorf("streak = %v, want 1", d["streak"])
	}

	words := d["recentWords"].([]any)
	if len(words) != 2 || words[0].(map[string]any)["term"] != "voda" || words[1].(map[string]any)["term"] != "kuća" {
		t.Errorf("recent words = %v, want voda then kuća", words)
	}
	docs := d["recentDocuments"].([]any)
	if len(docs) != 2 {
		t.Fatalf("recent documents = %v", docs)
	}
	first, second := docs[0].(map[string]any), docs[1].(map[string]any)
	if first["id"] != reader || first["pageNumber"] != 3.0 || first["lesson"].(map[string]any)["title"] != "Lekcija 1" {
		t.Errorf("first recent document = %v, want the reader at page 3", first)
	}
	if second["id"] != workbook || second["pageNumber"] != 1.0 {
		t.Errorf("second recent document = %v, want the workbook at page 1", second)
	}

	// The same work in UTC puts kuća on the UTC date of that moment.
	_, d = ana.send(http.MethodGet, "/api/dashboard?timeZone=UTC", nil)
	utcDate := addedAt.UTC().Format(time.DateOnly)
	if utcDate == addedAt.Format(time.DateOnly) {
		t.Fatalf("test setup: %v is the same date in UTC and Belgrade", addedAt)
	}
	if day := activityOn(t, d, utcDate); day["words"] != 1.0 {
		t.Errorf("activity in UTC on %s = %v, want kuća", utcDate, day)
	}
}

func TestDashboardIsPrivate(t *testing.T) {
	srv, _ := newStack(t)
	ana := signedUp(t, newBrowser(t, srv), "ana@example.com")
	marko := signedUp(t, newBrowser(t, srv), "marko@example.com")
	reader := ana.uploadPDF(ana.createLesson("Lekcija 1"), "Čitanka.pdf")
	ana.addWord(map[string]string{"term": "kuća", "translation": "house"})
	ana.annotate(reader, highlight(1, "Dobar dan"))

	resp, d := marko.send(http.MethodGet, "/api/dashboard?timeZone=Europe/Belgrade", nil)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("dashboard = %d %v", resp.StatusCode, d)
	}
	for k, v := range d["totals"].(map[string]any) {
		if v != 0.0 {
			t.Errorf("Marko's totals.%s = %v", k, v)
		}
	}
	if len(d["recentWords"].([]any)) != 0 || len(d["recentDocuments"].([]any)) != 0 || d["streak"] != 0.0 {
		t.Errorf("Marko's dashboard = %v", d)
	}
}

func TestDashboardRejectsUnknownTimeZonesAndTeachers(t *testing.T) {
	srv, _ := newStack(t)
	ana := signedUp(t, newBrowser(t, srv), "ana@example.com")
	if resp, body := ana.send(http.MethodGet, "/api/dashboard?timeZone=Mars/Olympus", nil); resp.StatusCode != http.StatusUnprocessableEntity {
		t.Errorf("unknown time zone = %d %v", resp.StatusCode, body)
	}

	teacher := newBrowser(t, srv)
	teacher.send(http.MethodPost, "/api/auth/signup", signUpPayload("teacher@example.com", "teacher"))
	if resp, _ := teacher.send(http.MethodGet, "/api/dashboard", nil); resp.StatusCode != http.StatusForbidden {
		t.Errorf("teacher = %d, want 403", resp.StatusCode)
	}
}
