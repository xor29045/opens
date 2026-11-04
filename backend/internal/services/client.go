package services

import (
	"context"
	"sort"

	"github.com/google/uuid"

	"opensex/backend/internal/apperr"
	"opensex/backend/internal/models"
	"opensex/backend/internal/repositories"
	"opensex/backend/internal/validators"
)

// ClientService serves the anonymous "Découvrir" deck and the account
// deletion helper shared by the other services. There are no client accounts
// anymore (ADR-023).
type ClientService struct{ *Services }

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

// ---------------------------------------------------------------- "Découvrir" deck

// DiscoverCard is a profile proposed in the like / dislike deck. It carries
// no contact details (those stay on the profile page, RM-40).
type DiscoverCard struct {
	ID             uuid.UUID `json:"id"`
	Slug           string    `json:"slug"`
	DisplayName    string    `json:"display_name"`
	Headline       string    `json:"headline"`
	PriceFromCents *int64    `json:"price_from_cents"`
	Age            *int16    `json:"age"`
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

// Limits of the anonymous deck requests.
const (
	MaxDeckExclude = 1000
	MaxDeckIDs     = 200
)

// Discover returns a batch of published profiles the visitor has not rated
// yet; exclude holds the likes and passes kept in the browser (ADR-023).
func (s *ClientService) Discover(ctx context.Context, f repositories.CatalogFilter, exclude []uuid.UUID, limit int) (*DiscoverResult, error) {
	if limit < 1 || limit > 30 {
		limit = 15
	}
	if len(exclude) > MaxDeckExclude {
		return nil, apperr.Invalid("exclude", "too_many_items")
	}
	if err := validateDeckFilter(f); err != nil {
		return nil, err
	}
	rows, remaining, err := s.Repos.Favorites.Discover(ctx, f, exclude, limit)
	if err != nil {
		return nil, apperr.Internal(err)
	}
	items, err := s.cards(ctx, rows)
	if err != nil {
		return nil, err
	}
	return &DiscoverResult{Items: items, Remaining: remaining}, nil
}

// Cards describes the liked profiles still published (favorites page); the
// others are simply absent from the answer.
func (s *ClientService) Cards(ctx context.Context, ids []uuid.UUID) ([]DiscoverCard, error) {
	if len(ids) > MaxDeckIDs {
		return nil, apperr.Invalid("ids", "too_many_items")
	}
	if len(ids) == 0 {
		return []DiscoverCard{}, nil
	}
	rows, err := s.Repos.Favorites.Published(ctx, ids)
	if err != nil {
		return nil, apperr.Internal(err)
	}
	return s.cards(ctx, rows)
}

func (s *ClientService) cards(ctx context.Context, rows []repositories.DiscoverRow) ([]DiscoverCard, error) {
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
	out := make([]DiscoverCard, 0, len(rows))
	for _, r := range rows {
		card := DiscoverCard{ID: r.ID, Slug: r.Slug, DisplayName: r.DisplayName, Headline: r.Headline,
			PriceFromCents: r.PriceFromCents, Age: r.Age, Languages: []string(r.Languages), Photos: byProfile[r.ID], Categories: []string{},
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
		out = append(out, card)
	}
	return out, nil
}

// validateDeckFilter checks the "Découvrir" filters against the closed
// vocabularies (GET /attributes).
func validateDeckFilter(f repositories.CatalogFilter) error {
	var c apperr.Collector
	if f.City != "" && !validators.IsSlug(f.City) {
		c.Add("filters.city", "invalid_slug")
	}
	lists := []struct {
		field   string
		values  []string
		allowed []string
	}{
		{"filters.hair_color", f.HairColors, validators.HairColors},
		{"filters.eye_color", f.EyeColors, validators.EyeColors},
		{"filters.skin_tone", f.SkinTones, validators.ExtraAttributes["skin_tone"]},
		{"filters.gender", f.Genders, validators.ExtraAttributes["gender"]},
	}
	for _, l := range lists {
		if len(l.values) > len(l.allowed) {
			c.Add(l.field, "too_many_items")
			continue
		}
		for _, v := range l.values {
			if !validators.OneOf(v, l.allowed) {
				c.Add(l.field, "invalid_value")
				break
			}
		}
	}
	ranges := []struct {
		field    string
		min, max int
		lo, hi   int
	}{
		{"filters.height", f.HeightMin, f.HeightMax, 120, 230},
		{"filters.age", f.AgeMin, f.AgeMax, 18, 99},
		{"filters.price", int(f.PriceMinCents / 100), int(f.PriceMaxCents / 100), 0, 100000},
	}
	for _, r := range ranges {
		if r.min < 0 || r.max < 0 || (r.min > 0 && (r.min < r.lo || r.min > r.hi)) || (r.max > 0 && (r.max < r.lo || r.max > r.hi)) ||
			(r.min > 0 && r.max > 0 && r.min > r.max) {
			c.Add(r.field, "out_of_range")
		}
	}
	return c.Err()
}
