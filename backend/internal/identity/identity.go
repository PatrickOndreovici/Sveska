// Package identity owns accounts, passwords and sessions.
package identity

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"

	"learning-serbian-platform/backend/internal/apperr"
	"learning-serbian-platform/backend/internal/store"
)

// Role is fixed at sign-up. Accounts are either students or teachers.
type Role string

const (
	RoleStudent Role = "student"
	RoleTeacher Role = "teacher"
)

// User is the public view of an account. It never carries the password hash.
type User struct {
	ID          string
	Email       string
	DisplayName string
	Role        Role
	CreatedAt   time.Time
}

// Session is a newly issued login. Token goes into the cookie and is not stored.
type Session struct {
	Token     string
	ExpiresAt time.Time
}

type SignUpInput struct {
	Email       string
	Password    string
	DisplayName string
	Role        Role
}

type LogInInput struct {
	Email    string
	Password string
}

// Store is the subset of generated queries this package needs.
// *store.Queries satisfies it; tests may substitute a fake.
type Store interface {
	CreateUser(ctx context.Context, arg store.CreateUserParams) (store.CreateUserRow, error)
	GetUserCredentialsByEmail(ctx context.Context, email string) (store.GetUserCredentialsByEmailRow, error)
	CreateSession(ctx context.Context, arg store.CreateSessionParams) error
	GetSessionUser(ctx context.Context, tokenHash []byte) (store.GetSessionUserRow, error)
	DeleteSession(ctx context.Context, tokenHash []byte) error
}

// Service implements sign-up, log-in, log-out and session lookup.
type Service struct {
	store      Store
	sessionTTL time.Duration
	now        func() time.Time
}

func NewService(s Store, sessionTTL time.Duration) *Service {
	return &Service{store: s, sessionTTL: sessionTTL, now: time.Now}
}

var (
	errInvalidCredentials = apperr.New(apperr.Unauthenticated, "invalid_credentials", "Email or password is incorrect.")
	errNoSession          = apperr.New(apperr.Unauthenticated, "unauthenticated", "Log in to continue.")
	errEmailTaken         = apperr.New(apperr.Conflict, "email_taken", "An account with this email already exists.")
)

// SignUp creates an account and logs it in.
func (s *Service) SignUp(ctx context.Context, in SignUpInput) (User, Session, error) {
	in.Email = normalizeEmail(in.Email)
	in.DisplayName = normalizeDisplayName(in.DisplayName)

	fields := map[string]string{}
	addProblem(fields, "email", validateEmail(in.Email))
	addProblem(fields, "password", validatePassword(in.Password))
	addProblem(fields, "displayName", validateDisplayName(in.DisplayName))
	addProblem(fields, "role", validateRole(in.Role))
	if len(fields) > 0 {
		return User{}, Session{}, apperr.Validation(fields)
	}

	hash, err := hashPassword(in.Password)
	if err != nil {
		return User{}, Session{}, err
	}
	row, err := s.store.CreateUser(ctx, store.CreateUserParams{
		Email:        in.Email,
		PasswordHash: hash,
		DisplayName:  in.DisplayName,
		Role:         store.UserRole(in.Role),
	})
	if isUniqueViolation(err, "users_email_key") {
		return User{}, Session{}, errEmailTaken
	}
	if err != nil {
		return User{}, Session{}, fmt.Errorf("create user: %w", err)
	}

	user := User{ID: row.ID, Email: row.Email, DisplayName: row.DisplayName, Role: Role(row.Role), CreatedAt: row.CreatedAt}
	session, err := s.startSession(ctx, user.ID)
	if err != nil {
		return User{}, Session{}, err
	}
	return user, session, nil
}

// LogIn checks credentials and starts a new session.
func (s *Service) LogIn(ctx context.Context, in LogInInput) (User, Session, error) {
	in.Email = normalizeEmail(in.Email)

	fields := map[string]string{}
	if in.Email == "" {
		fields["email"] = "Enter your email address."
	}
	if in.Password == "" {
		fields["password"] = "Enter your password."
	}
	if len(fields) > 0 {
		return User{}, Session{}, apperr.Validation(fields)
	}
	// Overlong passwords can never match a stored one; skip the expensive hash.
	if len(in.Password) > 4*maxPasswordLen {
		return User{}, Session{}, errInvalidCredentials
	}

	row, err := s.store.GetUserCredentialsByEmail(ctx, in.Email)
	if errors.Is(err, pgx.ErrNoRows) {
		// Spend the same time as a real check so response timing does not
		// reveal which emails have accounts.
		_, _ = verifyPassword(in.Password, dummyHash())
		return User{}, Session{}, errInvalidCredentials
	}
	if err != nil {
		return User{}, Session{}, fmt.Errorf("get user by email: %w", err)
	}

	ok, err := verifyPassword(in.Password, row.PasswordHash)
	if err != nil {
		return User{}, Session{}, fmt.Errorf("verify password for user %s: %w", row.ID, err)
	}
	if !ok {
		return User{}, Session{}, errInvalidCredentials
	}

	user := User{ID: row.ID, Email: row.Email, DisplayName: row.DisplayName, Role: Role(row.Role), CreatedAt: row.CreatedAt}
	session, err := s.startSession(ctx, user.ID)
	if err != nil {
		return User{}, Session{}, err
	}
	return user, session, nil
}

// LogOut ends the session for token. Unknown or malformed tokens are ignored.
func (s *Service) LogOut(ctx context.Context, token string) error {
	if !looksLikeToken(token) {
		return nil
	}
	if err := s.store.DeleteSession(ctx, hashToken(token)); err != nil {
		return fmt.Errorf("delete session: %w", err)
	}
	return nil
}

// Authenticate returns the user behind a valid, unexpired session token.
func (s *Service) Authenticate(ctx context.Context, token string) (User, error) {
	if !looksLikeToken(token) {
		return User{}, errNoSession
	}
	row, err := s.store.GetSessionUser(ctx, hashToken(token))
	if errors.Is(err, pgx.ErrNoRows) {
		return User{}, errNoSession
	}
	if err != nil {
		return User{}, fmt.Errorf("get session user: %w", err)
	}
	return User{ID: row.ID, Email: row.Email, DisplayName: row.DisplayName, Role: Role(row.Role), CreatedAt: row.CreatedAt}, nil
}

func (s *Service) startSession(ctx context.Context, userID string) (Session, error) {
	token, hash, err := newSessionToken()
	if err != nil {
		return Session{}, err
	}
	expiresAt := s.now().Add(s.sessionTTL).UTC().Truncate(time.Second)
	if err := s.store.CreateSession(ctx, store.CreateSessionParams{UserID: userID, TokenHash: hash, ExpiresAt: expiresAt}); err != nil {
		return Session{}, fmt.Errorf("create session: %w", err)
	}
	return Session{Token: token, ExpiresAt: expiresAt}, nil
}

func addProblem(fields map[string]string, field, problem string) {
	if problem != "" {
		fields[field] = problem
	}
}

func isUniqueViolation(err error, constraint string) bool {
	var pgErr *pgconn.PgError
	return errors.As(err, &pgErr) && pgErr.Code == "23505" && pgErr.ConstraintName == constraint
}
