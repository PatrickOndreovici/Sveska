package httpapi

import (
	"context"
	"log/slog"
	"net/http"
	"time"
)

// Pinger reports whether a dependency is reachable. *pgxpool.Pool satisfies it.
type Pinger interface {
	Ping(ctx context.Context) error
}

type healthResponse struct {
	Status   string `json:"status"`
	Database string `json:"database"`
}

const healthCheckTimeout = 2 * time.Second

func healthHandler(db Pinger) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ctx, cancel := context.WithTimeout(r.Context(), healthCheckTimeout)
		defer cancel()

		if err := db.Ping(ctx); err != nil {
			slog.Warn("health check: database unreachable", "error", err)
			writeJSON(w, http.StatusServiceUnavailable, healthResponse{Status: "unavailable", Database: "unreachable"})
			return
		}
		writeJSON(w, http.StatusOK, healthResponse{Status: "ok", Database: "ok"})
	}
}
