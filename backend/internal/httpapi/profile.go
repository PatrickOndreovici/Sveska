package httpapi

import (
	"bytes"
	"context"
	"io"
	"net/http"

	"learning-serbian-platform/backend/internal/access"
	"learning-serbian-platform/backend/internal/identity"
)

// ProfileService is the subset of identity.ProfileService the handlers need.
type ProfileService interface {
	UpdateDisplayName(ctx context.Context, userID, displayName string) (identity.User, error)
	SaveAvatar(ctx context.Context, user identity.User, body io.Reader) (identity.User, error)
	Avatar(ctx context.Context, userID string) (identity.Avatar, error)
	DeleteAvatar(ctx context.Context, userID string) error
}

type profileRequest struct {
	DisplayName string `json:"displayName"`
}

// The profile handlers act on the logged-in user only; the URL names no
// user, so there is no one else's profile to reach.

func (a *api) updateMe(w http.ResponseWriter, r *http.Request) {
	actor, err := access.RequireActor(r.Context())
	if err != nil {
		writeServiceError(w, r, err)
		return
	}
	var req profileRequest
	if err := decodeJSON(w, r, &req); err != nil {
		writeServiceError(w, r, err)
		return
	}
	user, err := a.profile.UpdateDisplayName(r.Context(), actor.User.ID, req.DisplayName)
	if err != nil {
		writeServiceError(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, toUserEnvelope(user))
}

func (a *api) getAvatar(w http.ResponseWriter, r *http.Request) {
	actor, err := access.RequireActor(r.Context())
	if err != nil {
		writeServiceError(w, r, err)
		return
	}
	avatar, err := a.profile.Avatar(r.Context(), actor.User.ID)
	if err != nil {
		writeServiceError(w, r, err)
		return
	}
	h := w.Header()
	h.Set("Content-Type", "image/jpeg")
	// The page asks for /api/me/avatar?v=<version>, so the browser may keep
	// a picture for good: a new one has a new URL. private keeps shared
	// caches from storing one user's picture.
	h.Set("Cache-Control", "private, max-age=31536000, immutable")
	http.ServeContent(w, r, "", avatar.UpdatedAt, bytes.NewReader(avatar.Image))
}

func (a *api) uploadAvatar(w http.ResponseWriter, r *http.Request) {
	actor, err := access.RequireActor(r.Context())
	if err != nil {
		writeServiceError(w, r, err)
		return
	}
	r.Body = http.MaxBytesReader(w, r.Body, identity.MaxAvatarBytes+maxMultipartOverhead)
	part, err := filePart(r, "Choose a picture to upload.")
	if err != nil {
		writeServiceError(w, r, uploadError(err, identity.ErrAvatarTooLarge))
		return
	}
	defer part.Close()

	user, err := a.profile.SaveAvatar(r.Context(), actor.User, part)
	if err != nil {
		writeServiceError(w, r, uploadError(err, identity.ErrAvatarTooLarge))
		return
	}
	writeJSON(w, http.StatusOK, toUserEnvelope(user))
}

func (a *api) deleteAvatar(w http.ResponseWriter, r *http.Request) {
	actor, err := access.RequireActor(r.Context())
	if err != nil {
		writeServiceError(w, r, err)
		return
	}
	if err := a.profile.DeleteAvatar(r.Context(), actor.User.ID); err != nil {
		writeServiceError(w, r, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
