package access

import (
	"context"
	"errors"
	"testing"

	"learning-serbian-platform/backend/internal/apperr"
	"learning-serbian-platform/backend/internal/identity"
)

func TestRequireStudent(t *testing.T) {
	as := func(role identity.Role) context.Context {
		return WithActor(context.Background(), Actor{User: identity.User{ID: "u1", Role: role}})
	}
	cases := []struct {
		name string
		ctx  context.Context
		kind apperr.Kind // empty when the actor is allowed
	}{
		{"student", as(identity.RoleStudent), ""},
		{"teacher", as(identity.RoleTeacher), apperr.Forbidden},
		{"anonymous", context.Background(), apperr.Unauthenticated},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			actor, err := RequireStudent(c.ctx)
			if c.kind == "" {
				if err != nil || actor.User.ID != "u1" {
					t.Fatalf("RequireStudent = %+v, %v; want the student", actor, err)
				}
				return
			}
			var appErr *apperr.Error
			if !errors.As(err, &appErr) || appErr.Kind != c.kind {
				t.Fatalf("err = %v, want kind %s", err, c.kind)
			}
		})
	}
}
