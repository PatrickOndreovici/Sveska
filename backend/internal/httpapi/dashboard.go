package httpapi

import (
	"context"
	"net/http"
	"time"

	"learning-serbian-platform/backend/internal/learning"
)

// DashboardService is the subset of learning.DashboardService the handler needs.
type DashboardService interface {
	Get(ctx context.Context, timeZone string) (learning.Dashboard, error)
}

type totalsResponse struct {
	Words         int `json:"words"`
	WordsThisWeek int `json:"wordsThisWeek"`
	Lessons       int `json:"lessons"`
	Documents     int `json:"documents"`
	Highlights    int `json:"highlights"`
	Notes         int `json:"notes"`
}

type activityDayResponse struct {
	Date       string `json:"date"`
	Words      int    `json:"words"`
	Highlights int    `json:"highlights"`
	Pages      int    `json:"pages"`
}

type recentWordResponse struct {
	ID          string    `json:"id"`
	Term        string    `json:"term"`
	Translation string    `json:"translation"`
	CreatedAt   time.Time `json:"createdAt"`
}

type recentDocumentResponse struct {
	ID         string    `json:"id"`
	Title      string    `json:"title"`
	Lesson     lessonRef `json:"lesson"`
	PageNumber int       `json:"pageNumber"`
	WorkedAt   time.Time `json:"workedAt"`
}

type dashboardResponse struct {
	Today           string                   `json:"today"`
	Totals          totalsResponse           `json:"totals"`
	Streak          int                      `json:"streak"`
	Activity        []activityDayResponse    `json:"activity"`
	RecentWords     []recentWordResponse     `json:"recentWords"`
	RecentDocuments []recentDocumentResponse `json:"recentDocuments"`
}

func (a *api) getDashboard(w http.ResponseWriter, r *http.Request) {
	d, err := a.dashboard.Get(r.Context(), r.URL.Query().Get("timeZone"))
	if err != nil {
		writeServiceError(w, r, err)
		return
	}
	t := d.Totals
	resp := dashboardResponse{
		Today: d.Today,
		Totals: totalsResponse{
			Words: t.Words, WordsThisWeek: t.WordsThisWeek, Lessons: t.Lessons,
			Documents: t.Documents, Highlights: t.Highlights, Notes: t.Notes,
		},
		Streak:          d.Streak,
		Activity:        make([]activityDayResponse, 0, len(d.Activity)),
		RecentWords:     make([]recentWordResponse, 0, len(d.RecentWords)),
		RecentDocuments: make([]recentDocumentResponse, 0, len(d.RecentDocuments)),
	}
	for _, day := range d.Activity {
		resp.Activity = append(resp.Activity, activityDayResponse(day))
	}
	for _, word := range d.RecentWords {
		resp.RecentWords = append(resp.RecentWords, recentWordResponse{
			ID: word.ID, Term: word.Term, Translation: word.Translation, CreatedAt: word.CreatedAt.UTC(),
		})
	}
	for _, doc := range d.RecentDocuments {
		resp.RecentDocuments = append(resp.RecentDocuments, recentDocumentResponse{
			ID:         doc.ID,
			Title:      doc.Title,
			Lesson:     lessonRef{ID: doc.LessonID, Title: doc.LessonTitle},
			PageNumber: doc.PageNumber,
			WorkedAt:   doc.WorkedAt.UTC(),
		})
	}
	writeJSON(w, http.StatusOK, resp)
}
