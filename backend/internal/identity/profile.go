package identity

import (
	"context"
	"errors"
	"fmt"
	"io"
	"time"

	"github.com/jackc/pgx/v5"

	"learning-serbian-platform/backend/internal/apperr"
	"learning-serbian-platform/backend/internal/store"
)

// Avatar is a user's profile picture, a JPEG.
type Avatar struct {
	Image     []byte
	UpdatedAt time.Time
}

// ProfileStore is the subset of generated queries the profile needs.
type ProfileStore interface {
	UpdateDisplayName(ctx context.Context, arg store.UpdateDisplayNameParams) (store.UpdateDisplayNameRow, error)
	SaveAvatar(ctx context.Context, arg store.SaveAvatarParams) (time.Time, error)
	GetAvatar(ctx context.Context, userID string) (store.GetAvatarRow, error)
	DeleteAvatar(ctx context.Context, userID string) error
}

// ProfileService changes how a user appears: their name and picture. Its
// methods act on the user they are given; the caller passes the logged-in
// user, so no one can change another account.
type ProfileService struct {
	store ProfileStore
}

func NewProfileService(s ProfileStore) *ProfileService {
	return &ProfileService{store: s}
}

var (
	// ErrAvatarTooLarge is also the answer when the upload's body is cut off
	// at the HTTP layer's limit.
	ErrAvatarTooLarge = apperr.Validation(map[string]string{
		"file": fmt.Sprintf("Choose a picture of at most %d MB.", MaxAvatarBytes>>20),
	})
	errNotAPicture   = apperr.Validation(map[string]string{"file": "Choose a JPEG, PNG, GIF or WebP picture."})
	errPictureTooBig = apperr.Validation(map[string]string{
		"file": fmt.Sprintf("This picture is too big. Choose one of at most %d megapixels.", maxAvatarPixels/1_000_000),
	})
	errNoAvatar = apperr.New(apperr.NotFound, "avatar_not_found", "There is no profile picture.")
)

// UpdateDisplayName changes the name shown for the user, with the same rules
// as at sign-up, and returns the user as now stored.
func (s *ProfileService) UpdateDisplayName(ctx context.Context, userID, displayName string) (User, error) {
	displayName = normalizeDisplayName(displayName)
	if problem := validateDisplayName(displayName); problem != "" {
		return User{}, apperr.Validation(map[string]string{"displayName": problem})
	}
	row, err := s.store.UpdateDisplayName(ctx, store.UpdateDisplayNameParams{ID: userID, DisplayName: displayName})
	if errors.Is(err, pgx.ErrNoRows) {
		return User{}, errNoSession
	}
	if err != nil {
		return User{}, fmt.Errorf("update display name: %w", err)
	}
	return User{
		ID: row.ID, Email: row.Email, DisplayName: row.DisplayName, Role: Role(row.Role), CreatedAt: row.CreatedAt,
		AvatarUpdatedAt: row.AvatarUpdatedAt,
	}, nil
}

// SaveAvatar makes the user's picture from an uploaded image (see
// makeAvatar), replacing any earlier one, and returns the user with it.
func (s *ProfileService) SaveAvatar(ctx context.Context, user User, body io.Reader) (User, error) {
	data, err := io.ReadAll(io.LimitReader(body, MaxAvatarBytes+1))
	if err != nil {
		return User{}, err
	}
	if len(data) > MaxAvatarBytes {
		return User{}, ErrAvatarTooLarge
	}
	picture, err := makeAvatar(data)
	if err != nil {
		return User{}, err
	}
	updatedAt, err := s.store.SaveAvatar(ctx, store.SaveAvatarParams{UserID: user.ID, Image: picture})
	if err != nil {
		return User{}, fmt.Errorf("save avatar: %w", err)
	}
	user.AvatarUpdatedAt = &updatedAt
	return user, nil
}

// Avatar returns the user's picture.
func (s *ProfileService) Avatar(ctx context.Context, userID string) (Avatar, error) {
	row, err := s.store.GetAvatar(ctx, userID)
	if errors.Is(err, pgx.ErrNoRows) {
		return Avatar{}, errNoAvatar
	}
	if err != nil {
		return Avatar{}, fmt.Errorf("get avatar: %w", err)
	}
	return Avatar{Image: row.Image, UpdatedAt: row.UpdatedAt}, nil
}

// DeleteAvatar removes the user's picture. Removing none is not an error.
func (s *ProfileService) DeleteAvatar(ctx context.Context, userID string) error {
	if err := s.store.DeleteAvatar(ctx, userID); err != nil {
		return fmt.Errorf("delete avatar: %w", err)
	}
	return nil
}
