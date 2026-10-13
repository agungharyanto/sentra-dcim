// Package principal carries the authenticated caller through the request context (ADR-003). The type field makes
// room for machine tokens (v0.6) next to users.
package principal

import (
	"context"

	"github.com/google/uuid"
)

// Types of principals.
const (
	TypeUser = "user"
)

// Principal is who is making the request.
type Principal struct {
	Type      string    `json:"type"`
	ID        uuid.UUID `json:"id"`
	Username  string    `json:"username"`
	SessionID uuid.UUID `json:"-"` // session that authenticated the request (users only)
}

type ctxKey struct{}

// With returns ctx carrying p.
func With(ctx context.Context, p Principal) context.Context {
	return context.WithValue(ctx, ctxKey{}, p)
}

// From returns the principal of ctx, if any.
func From(ctx context.Context) (Principal, bool) {
	p, ok := ctx.Value(ctxKey{}).(Principal)
	return p, ok
}
