package httpapi

import (
	"context"
	"errors"
	"log/slog"
	"net/http"

	"learning-serbian-platform/backend/internal/access"
	"learning-serbian-platform/backend/internal/apperr"
	"learning-serbian-platform/backend/internal/identity"
)

// securityHeaders sets conservative headers on every API response.
// The API serves JSON only, so nothing may be framed, sniffed or cached.
func securityHeaders(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		h := w.Header()
		h.Set("X-Content-Type-Options", "nosniff")
		h.Set("X-Frame-Options", "DENY")
		h.Set("Content-Security-Policy", "default-src 'none'; frame-ancestors 'none'")
		h.Set("Referrer-Policy", "no-referrer")
		h.Set("Cache-Control", "no-store")
		next.ServeHTTP(w, r)
	})
}

// requireSameOrigin blocks cross-site state-changing requests (CSRF).
// Browsers always send Origin on cross-origin POST/PUT/PATCH/DELETE, so a
// mismatching Origin is rejected. Requests without Origin come from
// non-browser clients, which cannot carry a victim's cookies, and are allowed.
// SameSite=Lax on the session cookie is the second layer.
func requireSameOrigin(allowedOrigin string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			switch r.Method {
			case http.MethodGet, http.MethodHead, http.MethodOptions:
				next.ServeHTTP(w, r)
				return
			}
			if origin := r.Header.Get("Origin"); origin != "" && origin != allowedOrigin {
				writeError(w, http.StatusForbidden, "forbidden_origin", "This request must come from the Sveska app.")
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}

// loadSession attaches the actor for a valid session cookie. Requests without
// a valid session continue anonymously; a stale cookie is cleared.
func (a *api) loadSession(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		cookie, err := r.Cookie(sessionCookieName)
		if err != nil {
			next.ServeHTTP(w, r)
			return
		}
		user, err := a.auth.Authenticate(r.Context(), cookie.Value)
		var appErr *apperr.Error
		switch {
		case err == nil:
			r = r.WithContext(access.WithActor(r.Context(), access.Actor{User: user}))
		case errors.As(err, &appErr) && appErr.Kind == apperr.Unauthenticated:
			a.clearSessionCookie(w)
		default:
			slog.ErrorContext(r.Context(), "load session", "error", err)
			writeError(w, http.StatusInternalServerError, "internal_error", "Something went wrong on our side. Try again.")
			return
		}
		next.ServeHTTP(w, r)
	})
}

// requireActor rejects requests without an authenticated actor with 401.
func requireActor(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if _, err := access.RequireActor(r.Context()); err != nil {
			writeServiceError(w, r, err)
			return
		}
		next.ServeHTTP(w, r)
	})
}

// AuthService is what the HTTP layer needs from identity.
type AuthService interface {
	SignUp(ctx context.Context, in identity.SignUpInput) (identity.User, identity.Session, error)
	LogIn(ctx context.Context, in identity.LogInInput) (identity.User, identity.Session, error)
	LogOut(ctx context.Context, token string) error
	Authenticate(ctx context.Context, token string) (identity.User, error)
}
