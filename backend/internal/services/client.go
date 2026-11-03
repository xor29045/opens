package services

import (
	"context"
	"sort"

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
	return s.Me(ctx, actor)
}

// ---------------------------------------------------------------- "Découvrir" deck

// DiscoverCard is a profile proposed in the like / dislike deck. It carries
// no contact details (those stay on the profile page, RM-40).
type DiscoverCard struct {
	ID             uuid.UUID `json:"id"`
	Slug           string    `json:"slug"`
	DisplayName    string    `json:"display_name"`
	Headline       string    `json:"headline"`
	Age            *int      `json:"age"`
	PriceFromCents *int64    `json:"price_from_cents"`
	Languages      []string  `json:"languages"`
	Photos         []string  `json:"photos"`
	Categories     []string  `json:"categories"`
	Establishment  string    `json:"establishment_name"`
	City           *string   `json:"city"`
}

type DiscoverResult struct {
	Items     []DiscoverCard `json:"items"`
	Remaining int64          `json:"remaining"`
}

const maxDeckPhotos = 6

// Discover returns a batch of published profiles the client has not rated yet.
func (s *ClientService) Discover(ctx context.Context, actor *rbac.Actor, limit int) (*DiscoverResult, error) {
	if limit < 1 || limit > 30 {
		limit = 15
	}
	rows, remaining, err := s.Repos.Favorites.Discover(ctx, actor.UserID, limit)
	if err != nil {
		return nil, apperr.Internal(err)
	}
	ids := make([]uuid.UUID, len(rows))
	for i, r := range rows {
		ids[i] = r.ID
	}
	photos, err := s.Repos.Profiles.PhotosUnscoped(ctx, ids)
	if err != nil {
		return nil, apperr.Internal(err)
	}
	sort.Slice(photos, func(i, j int) bool {
		if photos[i].IsPrimary != photos[j].IsPrimary {
			return photos[i].IsPrimary
		}
		return photos[i].Position < photos[j].Position
	})
	byProfile := map[uuid.UUID][]string{}
	for _, ph := range photos {
		if len(byProfile[ph.ProfileID]) < maxDeckPhotos {
			byProfile[ph.ProfileID] = append(byProfile[ph.ProfileID], s.Profiles.photoURL(ctx, ph.StorageKey, true))
		}
	}
	cats, err := s.Repos.Catalog.CategoriesOf(ctx, ids)
	if err != nil {
		return nil, apperr.Internal(err)
	}
	now := s.now()
	out := &DiscoverResult{Items: make([]DiscoverCard, 0, len(rows)), Remaining: remaining}
	for _, r := range rows {
		card := DiscoverCard{ID: r.ID, Slug: r.Slug, DisplayName: r.DisplayName, Headline: r.Headline, Age: ageOf(r.BirthDateAttested, now),
			PriceFromCents: r.PriceFromCents, Languages: []string(r.Languages), Photos: byProfile[r.ID], Categories: []string{},
			Establishment: r.EstablishmentName, City: r.CityName}
		if card.Languages == nil {
			card.Languages = []string{}
		}
		if card.Photos == nil {
			card.Photos = []string{}
		}
		for _, c := range cats[r.ID] {
			card.Categories = append(card.Categories, c.Name)
		}
		out.Items = append(out.Items, card)
	}
	return out, nil
}

// Dislike hides a published profile from the deck (and from the favorites).
func (s *ClientService) Dislike(ctx context.Context, actor *rbac.Actor, profileID uuid.UUID) error {
	p, err := s.Repos.Profiles.ByID(ctx, profileID)
	if err != nil || !p.IsPublished {
		if err == nil || database.IsNotFound(err) {
			return apperr.NotFound("profile")
		}
		return apperr.Internal(err)
	}
	return s.tx(ctx, func(ctx context.Context) error {
		if _, err := s.Repos.Favorites.Remove(ctx, actor.UserID, profileID); err != nil {
			return apperr.Internal(err)
		}
		if err := s.Repos.Favorites.AddDislike(ctx, actor.UserID, profileID, s.now()); err != nil {
			return apperr.Internal(err)
		}
		return nil
	})
}

// Undislike cancels a dislike (the profile comes back in the deck).
func (s *ClientService) Undislike(ctx context.Context, actor *rbac.Actor, profileID uuid.UUID) error {
	ok, err := s.Repos.Favorites.RemoveDislike(ctx, actor.UserID, profileID)
	if err != nil {
		return apperr.Internal(err)
	}
	if !ok {
		return apperr.NotFound("dislike")
	}
	return nil
}

// ClearDislikes brings every passed profile back in the deck.
func (s *ClientService) ClearDislikes(ctx context.Context, actor *rbac.Actor) (int64, error) {
	n, err := s.Repos.Favorites.ClearDislikes(ctx, actor.UserID)
	if err != nil {
		return 0, apperr.Internal(err)
	}
	return n, nil
}
