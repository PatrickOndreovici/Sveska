package httpapi

import (
	"fmt"
	"net/http"
	"time"

	"learning-serbian-platform/backend/internal/access"
	"learning-serbian-platform/backend/internal/identity"
)

const sessionCookieName = "sveska_session"

type userResponse struct {
	ID          string `json:"id"`
	Email       string `json:"email"`
	DisplayName string `json:"displayName"`
	Role        string `json:"role"`
	// AvatarURL is null without a profile picture.
	AvatarURL *string   `json:"avatarUrl"`
	CreatedAt time.Time `json:"createdAt"`
}

type userEnvelope struct {
	User userResponse `json:"user"`
}

// toUserEnvelope describes the logged-in user, who is the only one this
// API returns, so their picture is at /api/me/avatar. The URL carries the
// picture's version, so a browser may keep it and a new picture gets a
// new URL.
func toUserEnvelope(u identity.User) userEnvelope {
	var avatarURL *string
	if u.AvatarUpdatedAt != nil {
		url := fmt.Sprintf("/api/me/avatar?v=%d", u.AvatarUpdatedAt.UnixMilli())
		avatarURL = &url
	}
	return userEnvelope{User: userResponse{
		ID: u.ID, Email: u.Email, DisplayName: u.DisplayName, Role: string(u.Role), AvatarURL: avatarURL,
		CreatedAt: u.CreatedAt.UTC(),
	}}
}

type signUpRequest struct {
	Email       string `json:"email"`
	Password    string `json:"password"`
	DisplayName string `json:"displayName"`
	Role        string `json:"role"`
}

func (a *api) signUp(w http.ResponseWriter, r *http.Request) {
	var req signUpRequest
	if err := decodeJSON(w, r, &req); err != nil {
		writeServiceError(w, r, err)
		return
	}
	user, session, err := a.auth.SignUp(r.Context(), identity.SignUpInput{
		Email: req.Email, Password: req.Password, DisplayName: req.DisplayName, Role: identity.Role(req.Role),
	})
	if err != nil {
		writeServiceError(w, r, err)
		return
	}
	a.replaceSession(w, r, session)
	writeJSON(w, http.StatusCreated, toUserEnvelope(user))
}

type logInRequest struct {
	Email    string `json:"email"`
	Password string `json:"password"`
}

func (a *api) logIn(w http.ResponseWriter, r *http.Request) {
	var req logInRequest
	if err := decodeJSON(w, r, &req); err != nil {
		writeServiceError(w, r, err)
		return
	}
	user, session, err := a.auth.LogIn(r.Context(), identity.LogInInput{Email: req.Email, Password: req.Password})
	if err != nil {
		writeServiceError(w, r, err)
		return
	}
	a.replaceSession(w, r, session)
	writeJSON(w, http.StatusOK, toUserEnvelope(user))
}

func (a *api) logOut(w http.ResponseWriter, r *http.Request) {
	if cookie, err := r.Cookie(sessionCookieName); err == nil {
		if err := a.auth.LogOut(r.Context(), cookie.Value); err != nil {
			writeServiceError(w, r, err)
			return
		}
	}
	a.clearSessionCookie(w)
	w.WriteHeader(http.StatusNoContent)
}

func (a *api) me(w http.ResponseWriter, r *http.Request) {
	actor, err := access.RequireActor(r.Context())
	if err != nil {
		writeServiceError(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, toUserEnvelope(actor.User))
}

// replaceSession ends any session the request already carried, then sets the
// new cookie. This prevents an old session from lingering after re-login.
func (a *api) replaceSession(w http.ResponseWriter, r *http.Request, session identity.Session) {
	if old, err := r.Cookie(sessionCookieName); err == nil && old.Value != session.Token {
		_ = a.auth.LogOut(r.Context(), old.Value) // best effort; the old session expires anyway
	}
	http.SetCookie(w, &http.Cookie{
		Name:     sessionCookieName,
		Value:    session.Token,
		Path:     "/",
		Expires:  session.ExpiresAt,
		MaxAge:   int(time.Until(session.ExpiresAt).Seconds()),
		HttpOnly: true,
		Secure:   a.cookieSecure,
		SameSite: http.SameSiteLaxMode,
	})
}

func (a *api) clearSessionCookie(w http.ResponseWriter) {
	http.SetCookie(w, &http.Cookie{
		Name:     sessionCookieName,
		Value:    "",
		Path:     "/",
		MaxAge:   -1,
		HttpOnly: true,
		Secure:   a.cookieSecure,
		SameSite: http.SameSiteLaxMode,
	})
}
