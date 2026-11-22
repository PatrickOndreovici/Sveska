// Package access decides what the current caller may do.
//
// Every service method that touches user data receives an Actor and checks
// it here, so authorization rules live in one place: protected operations
// require an authenticated actor, and a student's own study material (lessons,
// documents, vocabulary) requires a student. Resource-level rules (owner,
// class membership) are added with the features they protect.
package access

import (
	"context"

	"learning-serbian-platform/backend/internal/apperr"
	"learning-serbian-platform/backend/internal/identity"
)

// Actor is the authenticated user making the request.
type Actor struct {
	User identity.User
}

type actorKey struct{}

// WithActor returns a context carrying the actor.
func WithActor(ctx context.Context, actor Actor) context.Context {
	return context.WithValue(ctx, actorKey{}, actor)
}

// RequireActor returns the authenticated actor, or an Unauthenticated error.
func RequireActor(ctx context.Context) (Actor, error) {
	actor, ok := ctx.Value(actorKey{}).(Actor)
	if !ok {
		return Actor{}, apperr.New(apperr.Unauthenticated, "unauthenticated", "Log in to continue.")
	}
	return actor, nil
}

// RequireStudent returns the actor if they are a student. Lessons, documents
// and vocabulary are a student's own study material; teachers will share
// material through classes and assignments instead.
func RequireStudent(ctx context.Context) (Actor, error) {
	actor, err := RequireActor(ctx)
	if err != nil {
		return Actor{}, err
	}
	if actor.User.Role != identity.RoleStudent {
		return Actor{}, apperr.New(apperr.Forbidden, "students_only", "This is only available to student accounts.")
	}
	return actor, nil
}
