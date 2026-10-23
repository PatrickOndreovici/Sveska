// Package httpapi maps HTTP requests to application services.
// Handlers decode input, call a service, and encode output. Business rules and
// authorization decisions live in the service packages, not here.
package httpapi

import (
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
)

// Dependencies are the collaborators and settings the router needs.
type Dependencies struct {
	DB           Pinger
	Auth         AuthService
	Profile      ProfileService
	Vocabulary   VocabularyService
	Lessons      LessonService
	Annotations  AnnotationService
	Drawings     DrawingService
	AppOrigin    string
	CookieSecure bool
}

type api struct {
	auth         AuthService
	profile      ProfileService
	vocabulary   VocabularyService
	lessons      LessonService
	annotations  AnnotationService
	drawings     DrawingService
	cookieSecure bool
}

// NewRouter builds the HTTP handler for the API.
func NewRouter(deps Dependencies) http.Handler {
	a := &api{
		auth:         deps.Auth,
		profile:      deps.Profile,
		vocabulary:   deps.Vocabulary,
		lessons:      deps.Lessons,
		annotations:  deps.Annotations,
		drawings:     deps.Drawings,
		cookieSecure: deps.CookieSecure,
	}

	r := chi.NewRouter()
	r.Use(middleware.RequestID)
	r.Use(middleware.RealIP)
	r.Use(middleware.Logger)
	r.Use(middleware.Recoverer)
	r.Use(securityHeaders)

	r.NotFound(func(w http.ResponseWriter, _ *http.Request) {
		writeError(w, http.StatusNotFound, "not_found", "The requested resource does not exist.")
	})
	r.MethodNotAllowed(func(w http.ResponseWriter, _ *http.Request) {
		writeError(w, http.StatusMethodNotAllowed, "method_not_allowed", "This method is not allowed for the resource.")
	})

	r.Route("/api", func(r chi.Router) {
		r.Get("/health", healthHandler(deps.DB))

		r.Group(func(r chi.Router) {
			r.Use(requireSameOrigin(deps.AppOrigin))
			r.Use(a.loadSession)

			r.Post("/auth/signup", a.signUp)
			r.Post("/auth/login", a.logIn)
			r.Post("/auth/logout", a.logOut)

			// Everything in this group requires a logged-in user.
			r.Group(func(r chi.Router) {
				r.Use(requireActor)
				r.Get("/me", a.me)
				r.Put("/me", a.updateMe)
				r.Get("/me/avatar", a.getAvatar)
				r.Put("/me/avatar", a.uploadAvatar)
				r.Delete("/me/avatar", a.deleteAvatar)

				r.Get("/vocabulary", a.listVocabulary)
				r.Post("/vocabulary", a.createVocabularyEntry)
				r.Put("/vocabulary/{id}", a.updateVocabularyEntry)
				r.Delete("/vocabulary/{id}", a.deleteVocabularyEntry)

				r.Get("/lessons", a.listLessons)
				r.Post("/lessons", a.createLesson)
				r.Get("/lessons/{id}", a.getLesson)
				r.Put("/lessons/{id}", a.updateLesson)
				r.Delete("/lessons/{id}", a.deleteLesson)
				r.Post("/lessons/{id}/documents", a.uploadDocument)

				r.Get("/documents/{id}", a.getDocument)
				r.Get("/documents/{id}/file", a.downloadDocument)
				r.Delete("/documents/{id}", a.deleteDocument)
				r.Get("/documents/{id}/annotations", a.listAnnotations)
				r.Post("/documents/{id}/annotations", a.createAnnotation)
				r.Get("/documents/{id}/strokes", a.listStrokes)
				r.Post("/documents/{id}/strokes", a.createStroke)
				r.Get("/documents/{id}/text-boxes", a.listTextBoxes)
				r.Post("/documents/{id}/text-boxes", a.createTextBox)

				r.Put("/annotations/{id}", a.updateAnnotation)
				r.Delete("/annotations/{id}", a.deleteAnnotation)

				r.Delete("/strokes/{id}", a.deleteStroke)
				r.Put("/text-boxes/{id}", a.updateTextBox)
				r.Delete("/text-boxes/{id}", a.deleteTextBox)
			})
		})
	})
	return r
}
