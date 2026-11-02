package services

import (
	"context"

	"github.com/google/uuid"

	"opensex/backend/internal/apperr"
	"opensex/backend/internal/database"
	"opensex/backend/internal/models"
	"opensex/backend/internal/rbac"
	"opensex/backend/internal/repositories"
	"opensex/backend/internal/validators"
)

// ClientService manages the CLIENT space: account and favourites (RM-50, RM-51).
type ClientService struct{ *Services }

func (s *ClientService) Me(ctx context.Context, actor *rbac.Actor) (*models.User, error) {
	u, err := s.Repos.Users.ByID(ctx, actor.UserID)
	if err != nil {
		return nil, apperr.Internal(err)
	}
	return u, nil
}

// DeleteAccount soft-deletes the client account after password confirmation;
// the data is purged after 30 days.
func (s *ClientService) DeleteAccount(ctx context.Context, actor *rbac.Actor, pwd string) error {
	u, err := s.Repos.Users.ByID(ctx, actor.UserID)
	if err != nil {
		return apperr.Internal(err)
	}
	if ok, _, _ := s.Hasher.Verify(ctx, pwd, u.PasswordHash); !ok {
		return apperr.New(apperr.KindForbidden, "invalid_password", "password is incorrect")
	}
	err = s.tx(ctx, func(ctx context.Context) error {
		if err := s.deleteAccount(ctx, u.ID); err != nil {
			return err
		}
		s.Audit.Record(ctx, AuditEvent{Action: "account.deleted", Category: models.AuditAuth, TargetType: "user", TargetID: u.ID.String()})
		return nil
	})
	if err == nil {
		_ = s.Denylist.DenyUser(ctx, u.ID)
	}
	return err
}

// deleteAccount anonymises and soft-deletes a user and revokes its sessions.
func (s *ClientService) deleteAccount(ctx context.Context, userID uuid.UUID) error {
	now := s.now()
	if err := s.Repos.Users.Update(ctx, userID, map[string]any{
		"status": models.UserDeleted, "deleted_at": now, "username": "deleted-" + userID.String(),
	}); err != nil {
		return apperr.Internal(err)
	}
	if err := s.DB.Conn(ctx).Where("user_id = ?", userID).Delete(&models.RecoveryCode{}).Error; err != nil {
		return apperr.Internal(err)
	}
	return s.Auth.RevokeAllSessions(ctx, userID, nil, models.RevokeDeleted)
}

func (s *ClientService) Favorites(ctx context.Context, actor *rbac.Actor, page repositories.Page) ([]repositories.FavoriteRow, int64, error) {
	rows, total, err := s.Repos.Favorites.List(ctx, actor.UserID, page)
	if err != nil {
		return nil, 0, apperr.Internal(err)
	}
	return rows, total, nil
}

// AddFavorite is idempotent; only published profiles can be added.
func (s *ClientService) AddFavorite(ctx context.Context, actor *rbac.Actor, profileID uuid.UUID) error {
	p, err := s.Repos.Profiles.ByID(ctx, profileID)
	if err != nil || !p.IsPublished {
		if err == nil || database.IsNotFound(err) {
			return apperr.NotFound("profile")
		}
		return apperr.Internal(err)
	}
	if err := s.Repos.Favorites.Add(ctx, actor.UserID, profileID, s.now()); err != nil {
		return apperr.Internal(err)
	}
	return nil
}

func (s *ClientService) RemoveFavorite(ctx context.Context, actor *rbac.Actor, profileID uuid.UUID) error {
	ok, err := s.Repos.Favorites.Remove(ctx, actor.UserID, profileID)
	if err != nil {
		return apperr.Internal(err)
	}
	if !ok {
		return apperr.NotFound("favorite")
	}
	return nil
}

// Rename changes the client's username (RM-03 rules apply).
func (s *ClientService) Rename(ctx context.Context, actor *rbac.Actor, username string) (*models.User, error) {
	if code := validators.Username(username); code != "" {
		return nil, apperr.Invalid("username", code)
	}
	if err := s.Repos.Users.Update(ctx, actor.UserID, map[string]any{"username": username}); err != nil {
		if database.IsUniqueViolation(err) {
			return nil, apperr.Conflict("username_taken", "username already taken")
		}
		return nil, apperr.Internal(err)
	}
	s.Audit.Record(ctx, AuditEvent{Action: "account.username_changed", Category: models.AuditAuth, TargetType: "user", TargetID: actor.UserID.String()})
	return s.Me(ctx, actor)
}
