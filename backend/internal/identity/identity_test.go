package identity

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"

	"learning-serbian-platform/backend/internal/apperr"
	"learning-serbian-platform/backend/internal/store"
)

// fakeStore keeps users and sessions in memory. It mirrors the database's
// unique email rule (case-insensitive) and session expiry.
type fakeStore struct {
	users    map[string]store.GetUserCredentialsByEmailRow // keyed by lower-case email
	sessions map[string]fakeSession                        // keyed by token hash
	now      time.Time
	nextID   int
}

type fakeSession struct {
	userEmail string
	expiresAt time.Time
}

func newFakeStore() *fakeStore {
	return &fakeStore{
		users:    map[string]store.GetUserCredentialsByEmailRow{},
		sessions: map[string]fakeSession{},
		now:      time.Date(2026, 10, 19, 12, 0, 0, 0, time.UTC),
	}
}

func lower(s string) string {
	b := []byte(s)
	for i, c := range b {
		if 'A' <= c && c <= 'Z' {
			b[i] = c + 32
		}
	}
	return string(b)
}

func (f *fakeStore) CreateUser(_ context.Context, arg store.CreateUserParams) (store.CreateUserRow, error) {
	if _, exists := f.users[lower(arg.Email)]; exists {
		return store.CreateUserRow{}, &pgconn.PgError{Code: "23505", ConstraintName: "users_email_key"}
	}
	f.nextID++
	row := store.GetUserCredentialsByEmailRow{
		ID: string(rune('0' + f.nextID)), Email: arg.Email, PasswordHash: arg.PasswordHash,
		DisplayName: arg.DisplayName, Role: arg.Role, CreatedAt: f.now,
	}
	f.users[lower(arg.Email)] = row
	return store.CreateUserRow{ID: row.ID, Email: row.Email, DisplayName: row.DisplayName, Role: row.Role, CreatedAt: row.CreatedAt}, nil
}

func (f *fakeStore) GetUserCredentialsByEmail(_ context.Context, email string) (store.GetUserCredentialsByEmailRow, error) {
	row, ok := f.users[lower(email)]
	if !ok {
		return store.GetUserCredentialsByEmailRow{}, pgx.ErrNoRows
	}
	return row, nil
}

func (f *fakeStore) CreateSession(_ context.Context, arg store.CreateSessionParams) error {
	for email, u := range f.users {
		if u.ID == arg.UserID {
			f.sessions[string(arg.TokenHash)] = fakeSession{userEmail: email, expiresAt: arg.ExpiresAt}
			return nil
		}
	}
	return errors.New("foreign key violation")
}

func (f *fakeStore) GetSessionUser(_ context.Context, tokenHash []byte) (store.GetSessionUserRow, error) {
	s, ok := f.sessions[string(tokenHash)]
	if !ok || !s.expiresAt.After(f.now) {
		return store.GetSessionUserRow{}, pgx.ErrNoRows
	}
	u := f.users[s.userEmail]
	return store.GetSessionUserRow{ID: u.ID, Email: u.Email, DisplayName: u.DisplayName, Role: u.Role, CreatedAt: u.CreatedAt}, nil
}

func (f *fakeStore) DeleteSession(_ context.Context, tokenHash []byte) error {
	delete(f.sessions, string(tokenHash))
	return nil
}

func newTestService() (*Service, *fakeStore) {
	fs := newFakeStore()
	svc := NewService(fs, 24*time.Hour)
	svc.now = func() time.Time { return fs.now }
	return svc, fs
}

var validSignUp = SignUpInput{Email: "ana@example.com", Password: "dobar-dan-123", DisplayName: "Ana", Role: RoleStudent}

func requireAppErr(t *testing.T, err error, kind apperr.Kind, code string) *apperr.Error {
	t.Helper()
	var appErr *apperr.Error
	if !errors.As(err, &appErr) {
		t.Fatalf("error = %v, want *apperr.Error", err)
	}
	if appErr.Kind != kind || appErr.Code != code {
		t.Fatalf("error = %s/%s, want %s/%s", appErr.Kind, appErr.Code, kind, code)
	}
	return appErr
}

func TestSignUpCreatesUserAndSession(t *testing.T) {
	svc, fs := newTestService()
	in := validSignUp
	in.Email = "  ana@example.com  "
	in.DisplayName = "  Ana  "

	user, session, err := svc.SignUp(context.Background(), in)
	if err != nil {
		t.Fatalf("SignUp: %v", err)
	}
	if user.Email != "ana@example.com" || user.DisplayName != "Ana" || user.Role != RoleStudent {
		t.Errorf("user = %+v", user)
	}
	if fs.users["ana@example.com"].PasswordHash == in.Password {
		t.Error("password stored in plain text")
	}
	if want := fs.now.Add(24 * time.Hour); !session.ExpiresAt.Equal(want) {
		t.Errorf("ExpiresAt = %v, want %v", session.ExpiresAt, want)
	}
	if got, err := svc.Authenticate(context.Background(), session.Token); err != nil || got.ID != user.ID {
		t.Errorf("Authenticate(new session) = %+v, %v", got, err)
	}
}

func TestSignUpReportsEveryInvalidField(t *testing.T) {
	svc, _ := newTestService()
	_, _, err := svc.SignUp(context.Background(), SignUpInput{Email: "nope", Password: "short", DisplayName: " ", Role: "admin"})
	appErr := requireAppErr(t, err, apperr.Invalid, "validation_failed")
	for _, field := range []string{"email", "password", "displayName", "role"} {
		if appErr.Fields[field] == "" {
			t.Errorf("missing problem for field %q", field)
		}
	}
}

func TestSignUpRejectsTakenEmailCaseInsensitively(t *testing.T) {
	svc, _ := newTestService()
	if _, _, err := svc.SignUp(context.Background(), validSignUp); err != nil {
		t.Fatalf("first SignUp: %v", err)
	}
	again := validSignUp
	again.Email = "ANA@example.com"
	again.Role = RoleTeacher
	_, _, err := svc.SignUp(context.Background(), again)
	requireAppErr(t, err, apperr.Conflict, "email_taken")
}

func TestLogIn(t *testing.T) {
	svc, _ := newTestService()
	created, _, err := svc.SignUp(context.Background(), validSignUp)
	if err != nil {
		t.Fatalf("SignUp: %v", err)
	}

	t.Run("correct credentials", func(t *testing.T) {
		user, session, err := svc.LogIn(context.Background(), LogInInput{Email: "Ana@Example.com", Password: validSignUp.Password})
		if err != nil {
			t.Fatalf("LogIn: %v", err)
		}
		if user.ID != created.ID || session.Token == "" {
			t.Errorf("user = %+v, token empty = %v", user, session.Token == "")
		}
	})
	t.Run("wrong password", func(t *testing.T) {
		_, _, err := svc.LogIn(context.Background(), LogInInput{Email: validSignUp.Email, Password: "wrong-password-1"})
		requireAppErr(t, err, apperr.Unauthenticated, "invalid_credentials")
	})
	t.Run("unknown email gets the same error as a wrong password", func(t *testing.T) {
		_, _, err := svc.LogIn(context.Background(), LogInInput{Email: "nobody@example.com", Password: "whatever-123"})
		requireAppErr(t, err, apperr.Unauthenticated, "invalid_credentials")
	})
	t.Run("missing fields", func(t *testing.T) {
		_, _, err := svc.LogIn(context.Background(), LogInInput{})
		appErr := requireAppErr(t, err, apperr.Invalid, "validation_failed")
		if appErr.Fields["email"] == "" || appErr.Fields["password"] == "" {
			t.Errorf("fields = %v", appErr.Fields)
		}
	})
}

func TestAuthenticate(t *testing.T) {
	svc, fs := newTestService()
	_, session, err := svc.SignUp(context.Background(), validSignUp)
	if err != nil {
		t.Fatalf("SignUp: %v", err)
	}

	t.Run("malformed token", func(t *testing.T) {
		_, err := svc.Authenticate(context.Background(), "not-a-token")
		requireAppErr(t, err, apperr.Unauthenticated, "unauthenticated")
	})
	t.Run("unknown token", func(t *testing.T) {
		token, _, _ := newSessionToken()
		_, err := svc.Authenticate(context.Background(), token)
		requireAppErr(t, err, apperr.Unauthenticated, "unauthenticated")
	})
	t.Run("expired session", func(t *testing.T) {
		fs.now = fs.now.Add(25 * time.Hour)
		defer func() { fs.now = fs.now.Add(-25 * time.Hour) }()
		_, err := svc.Authenticate(context.Background(), session.Token)
		requireAppErr(t, err, apperr.Unauthenticated, "unauthenticated")
	})
	t.Run("logged out session", func(t *testing.T) {
		if err := svc.LogOut(context.Background(), session.Token); err != nil {
			t.Fatalf("LogOut: %v", err)
		}
		_, err := svc.Authenticate(context.Background(), session.Token)
		requireAppErr(t, err, apperr.Unauthenticated, "unauthenticated")
	})
}
