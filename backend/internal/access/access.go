// Package access decides what the current caller may do.
//
// Every service method that touches user data receives an Actor and checks
// it here, so authorization rules live in one place. Phase 1 has a single
// rule: protected operations require an authenticated actor. Resource-level
// rules (owner, class membership) are added with the features they protect.
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
