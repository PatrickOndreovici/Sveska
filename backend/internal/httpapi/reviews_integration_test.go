//go:build integration

package httpapi

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"

	"learning-serbian-platform/backend/internal/store"
)

// These tests run the flashcard queries against PostgreSQL: the queue's
// order and counts, cards both ways round, saving a schedule and its log in
// one statement, and what happens to them when a word is deleted.

// queue fetches the review queue in UTC.
func (b *browser) queue() map[string]any {
	b.t.Helper()
	resp, body := b.send(http.MethodGet, "/api/review-queue?timeZone=UTC", nil)
	if resp.StatusCode != http.StatusOK {
		b.t.Fatalf("queue = %d %v", resp.StatusCode, body)
	}
	return body
}

// answer reviews a word Serbian → English in UTC and returns the review.
func (b *browser) answer(entryID, rating string) map[string]any {
	b.t.Helper()
	return b.answerCard(entryID, "sr_en", rating)
}

// answerCard reviews one of a word's cards in UTC and returns the review.
func (b *browser) answerCard(entryID, direction, rating string) map[string]any {
	b.t.Helper()
	resp, body := b.send(http.MethodPost, "/api/vocabulary/"+entryID+"/reviews", map[string]string{
		"direction": direction, "rating": rating, "timeZone": "UTC",
	})
	if resp.StatusCode != http.StatusCreated {
		b.t.Fatalf("answer %s %s %s = %d %v", entryID, direction, rating, resp.StatusCode, body)
	}
	return body["review"].(map[string]any)
}

// queueCards is the queue's cards as "term direction", new ones starred.
func queueCards(queue map[string]any) []string {
	var out []string
	for _, c := range queue["cards"].([]any) {
		card := c.(map[string]any)
		s := card["entry"].(map[string]any)["term"].(string) + " " + card["direction"].(string)
		if card["new"] == true {
			s += "*"
		}
		out = append(out, s)
	}
	return out
}

func cardTerms(queue map[string]any) []string {
	var out []string
	for _, c := range queue["cards"].([]any) {
		out = append(out, c.(map[string]any)["entry"].(map[string]any)["term"].(string))
	}
	return out
}

func daysFromToday(n int) string {
	return time.Now().UTC().AddDate(0, 0, n).Format(time.DateOnly)
}

func TestReviewingWords(t *testing.T) {
	srv, _ := newStack(t)
	ana := signedUp(t, newBrowser(t, srv), "ana@example.com")
	kuca := ana.addWord(map[string]string{"term": "kuća", "translation": "house"})
	voda := ana.addWord(map[string]string{"term": "voda", "translation": "water"})
	ana.addWord(map[string]string{"term": "hleb", "translation": "bread"})

	// Three new words, oldest first, and nothing scheduled yet.
	q := ana.queue()
	if fmt.Sprint(cardTerms(q)) != "[kuća voda hleb]" || q["due"] != 0.0 || q["new"] != 3.0 || q["nextDueOn"] != nil {
		t.Fatalf("first queue = %v %v", cardTerms(q), q)
	}

	if r := ana.answer(kuca, "good"); r["dueOn"] != daysFromToday(2) || r["intervalDays"] != 2.0 {
		t.Errorf("good = %v, want due in 2 days", r)
	}
	if r := ana.answer(voda, "again"); r["dueOn"] != daysFromToday(0) || r["intervalDays"] != 0.0 {
		t.Errorf("again = %v, want due today", r)
	}

	// voda was forgotten, so it is due again today, before the new hleb;
	// kuća comes back in two days.
	q = ana.queue()
	if fmt.Sprint(cardTerms(q)) != "[voda hleb]" || q["due"] != 1.0 || q["new"] != 1.0 || q["nextDueOn"] != daysFromToday(2) {
		t.Errorf("second queue = %v %v", cardTerms(q), q)
	}
	card := q["cards"].([]any)[0].(map[string]any)
	if card["new"] != false || card["nextIntervals"].(map[string]any)["good"] != 2.0 {
		t.Errorf("voda's card = %v", card)
	}

	// Answering counts on the dashboard, in the activity and the streak.
	_, d := ana.send(http.MethodGet, "/api/dashboard?timeZone=UTC", nil)
	if day := activityOn(t, d, daysFromToday(0)); day["reviews"] != 2.0 {
		t.Errorf("today's activity = %v, want 2 reviews", day)
	}
	if review := d["review"].(map[string]any); review["due"] != 1.0 || review["new"] != 1.0 {
		t.Errorf("dashboard review = %v", review)
	}
}

func TestReviewQueueLimitsNewWordsPerDay(t *testing.T) {
	srv, _ := newStack(t)
	ana := signedUp(t, newBrowser(t, srv), "ana@example.com")
	var ids []string
	for i := range 22 {
		ids = append(ids, ana.addWord(map[string]string{"term": fmt.Sprintf("reč %d", i), "translation": "word"}))
	}
	if q := ana.queue(); q["new"] != 20.0 || len(q["cards"].([]any)) != 20 {
		t.Fatalf("queue = new %v, %d cards; want 20", q["new"], len(q["cards"].([]any)))
	}
	for _, id := range ids[:20] {
		ana.answer(id, "easy")
	}
	// The last two words wait for tomorrow.
	q := ana.queue()
	if q["new"] != 0.0 || len(q["cards"].([]any)) != 0 || q["nextDueOn"] != daysFromToday(1) {
		t.Errorf("queue after 20 new words = %v", q)
	}
}

func TestReviewingWordsBothWays(t *testing.T) {
	srv, pool := newStack(t)
	ana := signedUp(t, newBrowser(t, srv), "ana@example.com")
	kuca := ana.addWord(map[string]string{"term": "kuća", "translation": "house"})
	exec := func(sql string) {
		t.Helper()
		if _, err := pool.Exec(context.Background(), sql); err != nil {
			t.Fatal(err)
		}
	}
	// lastReviewed moves kuća's cards' last review back days ago, and due
	// sets how many days ago each way became due.
	lastReviewed := func(days int) {
		exec(fmt.Sprintf("UPDATE review_cards SET last_reviewed_at = now() - interval '%d days'", days))
	}
	due := func(direction string, daysAgo int) {
		exec(fmt.Sprintf("UPDATE review_cards SET due_on = (now() AT TIME ZONE 'UTC')::date - %d WHERE direction = '%s'", daysAgo, direction))
	}

	// Remembered twice in a row today: the English → Serbian card waits for
	// a day without the other card, so it comes tomorrow.
	ana.answer(kuca, "good")
	ana.answer(kuca, "good")
	if q := ana.queue(); len(queueCards(q)) != 0 || q["new"] != 0.0 || q["nextDueOn"] != daysFromToday(1) {
		t.Fatalf("queue after two answers = %v %v", queueCards(q), q)
	}

	// A day later it is there, new.
	lastReviewed(1)
	q := ana.queue()
	if fmt.Sprint(queueCards(q)) != "[kuća en_sr*]" || q["new"] != 1.0 {
		t.Fatalf("queue a day later = %v %v", queueCards(q), q)
	}
	if r := ana.answerCard(kuca, "en_sr", "good"); r["direction"] != "en_sr" || r["intervalDays"] != 2.0 {
		t.Errorf("first English → Serbian answer = %v, want a new card's 2 days", r)
	}

	// Both ways due: the one due first shows, the other waits for tomorrow.
	lastReviewed(3)
	due("sr_en", 2)
	due("en_sr", 1)
	if q := ana.queue(); fmt.Sprint(queueCards(q)) != "[kuća sr_en]" || q["due"] != 1.0 {
		t.Fatalf("both due = %v %v", queueCards(q), q)
	}
	ana.answer(kuca, "good")
	if q := ana.queue(); len(queueCards(q)) != 0 || q["due"] != 0.0 || q["nextDueOn"] != daysFromToday(1) {
		t.Errorf("after one way = %v %v", queueCards(q), q)
	}

	// Forgotten today, a card comes back today, and the other way still waits.
	lastReviewed(3)
	due("sr_en", 2)
	due("en_sr", 1)
	ana.answer(kuca, "again")
	if q := ana.queue(); fmt.Sprint(queueCards(q)) != "[kuća sr_en]" || q["due"] != 1.0 {
		t.Errorf("after again = %v %v", queueCards(q), q)
	}

	var srEn, enSr int
	if err := pool.QueryRow(context.Background(),
		"SELECT count(*) FILTER (WHERE direction = 'sr_en'), count(*) FILTER (WHERE direction = 'en_sr') FROM reviews",
	).Scan(&srEn, &enSr); err != nil {
		t.Fatal(err)
	}
	if srEn != 4 || enSr != 1 {
		t.Errorf("logged %d Serbian → English and %d English → Serbian answers; want 4 and 1", srEn, enSr)
	}
}

func TestReviewsArePrivate(t *testing.T) {
	srv, _ := newStack(t)
	ana := signedUp(t, newBrowser(t, srv), "ana@example.com")
	marko := signedUp(t, newBrowser(t, srv), "marko@example.com")
	kuca := ana.addWord(map[string]string{"term": "kuća", "translation": "house"})

	if q := marko.queue(); len(q["cards"].([]any)) != 0 || q["new"] != 0.0 || q["nextDueOn"] != nil {
		t.Errorf("Marko's queue = %v", q)
	}
	resp, body := marko.send(http.MethodPost, "/api/vocabulary/"+kuca+"/reviews", map[string]string{"direction": "sr_en", "rating": "good"})
	if resp.StatusCode != http.StatusNotFound || errorCode(body) != "entry_not_found" {
		t.Errorf("Marko reviewing Ana's word = %d %v", resp.StatusCode, body)
	}
	resp, _ = ana.send(http.MethodPost, "/api/vocabulary/not-an-id/reviews", map[string]string{"direction": "sr_en", "rating": "good"})
	if resp.StatusCode != http.StatusNotFound {
		t.Errorf("not an id = %d, want 404", resp.StatusCode)
	}
	resp, body = ana.send(http.MethodPost, "/api/vocabulary/"+kuca+"/reviews", map[string]string{"direction": "sr_en", "rating": "perfect"})
	if resp.StatusCode != http.StatusUnprocessableEntity {
		t.Errorf("unknown rating = %d %v", resp.StatusCode, body)
	}
	resp, body = ana.send(http.MethodPost, "/api/vocabulary/"+kuca+"/reviews", map[string]string{"rating": "good"})
	if resp.StatusCode != http.StatusUnprocessableEntity {
		t.Errorf("no direction = %d %v", resp.StatusCode, body)
	}

	teacher := newBrowser(t, srv)
	teacher.send(http.MethodPost, "/api/auth/signup", signUpPayload("teacher@example.com", "teacher"))
	if resp, _ := teacher.send(http.MethodGet, "/api/review-queue", nil); resp.StatusCode != http.StatusForbidden {
		t.Errorf("teacher = %d, want 403", resp.StatusCode)
	}
}

func TestStaleReviewSavesNothing(t *testing.T) {
	srv, pool := newStack(t)
	ana := signedUp(t, newBrowser(t, srv), "ana@example.com")
	kuca := ana.addWord(map[string]string{"term": "kuća", "translation": "house"})
	ana.answer(kuca, "good")

	// A second answer that read the word while it was still new, as from
	// another tab: neither the schedule nor the log changes.
	var ownerID string
	if err := pool.QueryRow(context.Background(), "SELECT owner_id FROM vocabulary_entries WHERE id = $1", kuca).Scan(&ownerID); err != nil {
		t.Fatal(err)
	}
	_, err := store.New(pool).RecordReview(context.Background(), store.RecordReviewParams{
		EntryID: kuca, Direction: store.ReviewDirectionSrEn, OwnerID: ownerID, DueOn: time.Now(), IntervalDays: 0, EasePercent: 250,
		ReadReviewedAt: nil, Rating: store.ReviewRatingAgain,
	})
	if !errors.Is(err, pgx.ErrNoRows) {
		t.Fatalf("stale review: err = %v, want no rows", err)
	}
	var reviews, interval int
	if err := pool.QueryRow(context.Background(),
		"SELECT (SELECT count(*) FROM reviews), (SELECT interval_days FROM review_cards WHERE entry_id = $1)", kuca,
	).Scan(&reviews, &interval); err != nil {
		t.Fatal(err)
	}
	if reviews != 1 || interval != 2 {
		t.Errorf("%d reviews, interval %d; want 1 and the first answer's 2", reviews, interval)
	}
}

func TestDeletingAWordKeepsItsReviewsOnTheDashboard(t *testing.T) {
	srv, pool := newStack(t)
	ana := signedUp(t, newBrowser(t, srv), "ana@example.com")
	kuca := ana.addWord(map[string]string{"term": "kuća", "translation": "house"})
	ana.answer(kuca, "again")
	ana.answer(kuca, "good")

	if resp, _ := ana.send(http.MethodDelete, "/api/vocabulary/"+kuca, nil); resp.StatusCode != http.StatusNoContent {
		t.Fatalf("delete = %d", resp.StatusCode)
	}
	var cards, reviews int
	if err := pool.QueryRow(context.Background(),
		"SELECT (SELECT count(*) FROM review_cards), (SELECT count(*) FROM reviews WHERE entry_id IS NULL)",
	).Scan(&cards, &reviews); err != nil {
		t.Fatal(err)
	}
	if cards != 0 || reviews != 2 {
		t.Errorf("%d cards, %d reviews without a word; want 0 and 2", cards, reviews)
	}
	_, d := ana.send(http.MethodGet, "/api/dashboard?timeZone=UTC", nil)
	if day := activityOn(t, d, daysFromToday(0)); day["reviews"] != 2.0 {
		t.Errorf("today's activity = %v, want 2 reviews", day)
	}
}
