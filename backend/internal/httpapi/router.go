// Package httpapi maps HTTP requests to application services.
// Handlers decode input, call a service, and encode output. Business rules and
// authorization decisions live in the service packages, not here.
package httpapi

import (
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
)

// Dependencies are the collaborators the router needs.
type Dependencies struct {
	DB Pinger
}

// NewRouter builds the HTTP handler for the API.
func NewRouter(deps Dependencies) http.Handler {
	r := chi.NewRouter()
	r.Use(middleware.RequestID)
	r.Use(middleware.RealIP)
	r.Use(middleware.Logger)
	r.Use(middleware.Recoverer)

	r.NotFound(func(w http.ResponseWriter, _ *http.Request) {
		writeError(w, http.StatusNotFound, "not_found", "The requested resource does not exist.")
	})
	r.MethodNotAllowed(func(w http.ResponseWriter, _ *http.Request) {
		writeError(w, http.StatusMethodNotAllowed, "method_not_allowed", "This method is not allowed for the resource.")
	})

	r.Route("/api", func(r chi.Router) {
		r.Get("/health", healthHandler(deps.DB))
	})
	return r
}
