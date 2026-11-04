// Package rbac carries the authenticated Actor and request metadata through
// context.Context to the services (which never see Gin types).
package rbac

import (
	"context"

	"github.com/google/uuid"

	"opensex/backend/internal/models"
)

// Actor is built by the auth middleware from a verified token. Its scope
// (establishment, profile) is always derived server-side.
type Actor struct {
	UserID          uuid.UUID
	Role            string
	SessionID       uuid.UUID
	EstablishmentID uuid.UUID // MANAGER: managed establishment; PROFESSIONAL: employer
	ProfileID       uuid.UUID // PROFESSIONAL: own profile
}

func (a *Actor) Is(role string) bool { return a != nil && a.Role == role }

func (a *Actor) IsAdmin() bool { return a.Is(models.RoleAdmin) }

var AllRoles = map[string]bool{
	models.RoleAdmin: true, models.RoleManager: true, models.RoleProfessional: true,
}

type RequestMeta struct {
	RequestID string
	IP        string
	IPHash    string
	UserAgent string
}

type metaKey struct{}
type actorKey struct{}

func WithMeta(ctx context.Context, m RequestMeta) context.Context {
	return context.WithValue(ctx, metaKey{}, m)
}

func MetaFrom(ctx context.Context) RequestMeta {
	m, _ := ctx.Value(metaKey{}).(RequestMeta)
	return m
}

func WithActor(ctx context.Context, a *Actor) context.Context {
	return context.WithValue(ctx, actorKey{}, a)
}

// ActorFrom returns the actor or nil for anonymous requests.
func ActorFrom(ctx context.Context) *Actor {
	a, _ := ctx.Value(actorKey{}).(*Actor)
	return a
}
