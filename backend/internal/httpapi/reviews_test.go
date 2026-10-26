package httpapi

import (
	"context"
	"encoding/json"
	"net/http"
	"testing"

	"learning-serbian-platform/backend/internal/apperr"
	"learning-serbian-platform/backend/internal/learning"
)

// fakeReviews answers with a fixed queue and result, or err when set.
type fakeReviews struct {
	err          error
	calls        int
	lastTimeZone string
	lastEntryID  string
	lastInput    learning.ReviewInput
}

func (f *fakeReviews) Queue(_ context.Context, timeZone string) (learning.ReviewQueue, error) {
	f.calls++
	f.lastTimeZone = timeZone
	next := "2026-10-24"
	return learning.ReviewQueue{
		Today: "2026-10-23",
		Cards: []learning.ReviewCard{{
			Entry: learning.VocabularyEntry{
				ID:          entryID,
				EntryFields: learning.EntryFields{Term: "kuća", Translation: "house", PartOfSpeech: "noun", Gender: "feminine"},
				CreatedAt:   testTime,
				UpdatedAt:   testTime,
			},
			New:           true,
			NextIntervals: learning.NextIntervals{Again: 0, Hard: 1, Good: 2, Easy: 4},
		}},
		Due:       0,
		New:       1,
		NextDueOn: &next,
	}, f.err
}

func (f *fakeReviews) Record(_ context.Context, id string, in learning.ReviewInput) (learning.ReviewResult, error) {
	f.calls++
	f.lastEntryID = id
	f.lastInput = in
	return learning.ReviewResult{EntryID: id, Rating: in.Rating, DueOn: "2026-10-25", IntervalDays: 2}, f.err
}

func newReviewRouter(r *fakeReviews) http.Handler {
	return NewRouter(Dependencies{DB: fakePinger{}, Auth: &fakeAuth{}, Reviews: r, AppOrigin: testOrigin, CookieSecure: true})
}

func TestReviewsRequireSession(t *testing.T) {
	r := &fakeReviews{}
	h := newReviewRouter(r)
	queue := do(t, h, http.MethodGet, "/api/review-queue", requestOpts{})
	record := do(t, h, http.MethodPost, "/api/vocabulary/"+entryID+"/reviews", requestOpts{body: `{"rating":"good"}`, origin: testOrigin})
	if queue.Code != http.StatusUnauthorized || record.Code != http.StatusUnauthorized || r.calls != 0 {
		t.Errorf("status = %d, %d, service calls = %d; want 401 and 0", queue.Code, record.Code, r.calls)
	}
}

func TestReviewQueueResponse(t *testing.T) {
	r := &fakeReviews{}
	rec := do(t, newReviewRouter(r), http.MethodGet, "/api/review-queue?timeZone=Europe/Belgrade", loggedIn(""))
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", rec.Code, rec.Body)
	}
	if r.lastTimeZone != "Europe/Belgrade" {
		t.Errorf("time zone = %q", r.lastTimeZone)
	}
	var got map[string]any
	if err := json.NewDecoder(rec.Body).Decode(&got); err != nil {
		t.Fatal(err)
	}
	want := map[string]any{
		"today": "2026-10-23",
		"cards": []any{map[string]any{
			"entry": map[string]any{
				"id": entryID, "term": "kuća", "translation": "house", "notes": "", "example": "",
				"partOfSpeech": "noun", "gender": "feminine", "plural": "", "aspect": "", "aspectPair": "",
				"feminineForm": "", "neuterForm": "", "source": nil,
				"createdAt": "2026-10-23T12:00:00Z", "updatedAt": "2026-10-23T12:00:00Z",
			},
			"new":           true,
			"nextIntervals": map[string]any{"again": 0.0, "hard": 1.0, "good": 2.0, "easy": 4.0},
		}},
		"due":       0.0,
		"new":       1.0,
		"nextDueOn": "2026-10-24",
	}
	gotJSON, _ := json.Marshal(got)
	wantJSON, _ := json.Marshal(want)
	if string(gotJSON) != string(wantJSON) {
		t.Errorf("body =\n%s\nwant\n%s", gotJSON, wantJSON)
	}
}

func TestCreateReview(t *testing.T) {
	r := &fakeReviews{}
	o := loggedIn(`{"rating":"good","timeZone":"Europe/Belgrade"}`)
	rec := do(t, newReviewRouter(r), http.MethodPost, "/api/vocabulary/"+entryID+"/reviews", o)
	if rec.Code != http.StatusCreated {
		t.Fatalf("status = %d, body = %s", rec.Code, rec.Body)
	}
	if want := (learning.ReviewInput{Rating: learning.RatingGood, TimeZone: "Europe/Belgrade"}); r.lastEntryID != entryID || r.lastInput != want {
		t.Errorf("recorded %s %+v", r.lastEntryID, r.lastInput)
	}
	var got reviewEnvelope
	if err := json.NewDecoder(rec.Body).Decode(&got); err != nil {
		t.Fatal(err)
	}
	if want := (reviewResponse{EntryID: entryID, Rating: "good", DueOn: "2026-10-25", IntervalDays: 2}); got.Review != want {
		t.Errorf("review = %+v, want %+v", got.Review, want)
	}
}

func TestCreateReviewErrors(t *testing.T) {
	tests := []struct {
		err    error
		status int
	}{
		{apperr.Validation(map[string]string{"rating": "Choose again, hard, good or easy."}), http.StatusUnprocessableEntity},
		{apperr.New(apperr.NotFound, "entry_not_found", "This word is not in your vocabulary."), http.StatusNotFound},
		{apperr.New(apperr.Conflict, "review_conflict", "This word was just reviewed somewhere else."), http.StatusConflict},
		{apperr.New(apperr.Forbidden, "students_only", "This is only available to student accounts."), http.StatusForbidden},
	}
	for _, tt := range tests {
		rec := do(t, newReviewRouter(&fakeReviews{err: tt.err}), http.MethodPost, "/api/vocabulary/"+entryID+"/reviews",
			loggedIn(`{"rating":"good"}`))
		if rec.Code != tt.status {
			t.Errorf("%v: status = %d, want %d", tt.err, rec.Code, tt.status)
		}
	}
}
