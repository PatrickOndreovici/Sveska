package httpapi

import (
	"context"
	"net/http"

	"github.com/go-chi/chi/v5"

	"learning-serbian-platform/backend/internal/learning"
)

// ReviewService is the subset of learning.ReviewService the handlers need.
type ReviewService interface {
	Queue(ctx context.Context, timeZone string) (learning.ReviewQueue, error)
	Record(ctx context.Context, entryID string, in learning.ReviewInput) (learning.ReviewResult, error)
}

type nextIntervalsResponse struct {
	Again int `json:"again"`
	Hard  int `json:"hard"`
	Good  int `json:"good"`
	Easy  int `json:"easy"`
}

type reviewCardResponse struct {
	Entry         entryResponse         `json:"entry"`
	New           bool                  `json:"new"`
	NextIntervals nextIntervalsResponse `json:"nextIntervals"`
}

type reviewQueueResponse struct {
	Today     string               `json:"today"`
	Cards     []reviewCardResponse `json:"cards"`
	Due       int                  `json:"due"`
	New       int                  `json:"new"`
	NextDueOn *string              `json:"nextDueOn"`
}

// reviewRequest is the body of POST /api/vocabulary/{id}/reviews.
type reviewRequest struct {
	Rating   string `json:"rating"`
	TimeZone string `json:"timeZone"`
}

type reviewResponse struct {
	EntryID      string `json:"entryId"`
	Rating       string `json:"rating"`
	DueOn        string `json:"dueOn"`
	IntervalDays int    `json:"intervalDays"`
}

type reviewEnvelope struct {
	Review reviewResponse `json:"review"`
}

func (a *api) getReviewQueue(w http.ResponseWriter, r *http.Request) {
	q, err := a.reviews.Queue(r.Context(), r.URL.Query().Get("timeZone"))
	if err != nil {
		writeServiceError(w, r, err)
		return
	}
	resp := reviewQueueResponse{
		Today:     q.Today,
		Cards:     make([]reviewCardResponse, 0, len(q.Cards)),
		Due:       q.Due,
		New:       q.New,
		NextDueOn: q.NextDueOn,
	}
	for _, c := range q.Cards {
		resp.Cards = append(resp.Cards, reviewCardResponse{
			Entry:         toEntryResponse(c.Entry),
			New:           c.New,
			NextIntervals: nextIntervalsResponse(c.NextIntervals),
		})
	}
	writeJSON(w, http.StatusOK, resp)
}

func (a *api) createReview(w http.ResponseWriter, r *http.Request) {
	var req reviewRequest
	if err := decodeJSON(w, r, &req); err != nil {
		writeServiceError(w, r, err)
		return
	}
	res, err := a.reviews.Record(r.Context(), chi.URLParam(r, "id"), learning.ReviewInput{
		Rating:   learning.Rating(req.Rating),
		TimeZone: req.TimeZone,
	})
	if err != nil {
		writeServiceError(w, r, err)
		return
	}
	writeJSON(w, http.StatusCreated, reviewEnvelope{Review: reviewResponse{
		EntryID:      res.EntryID,
		Rating:       string(res.Rating),
		DueOn:        res.DueOn,
		IntervalDays: res.IntervalDays,
	}})
}
