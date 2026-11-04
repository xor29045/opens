package repositories

import (
	"context"
	"strings"

	"github.com/google/uuid"
	"gorm.io/gorm"

	"opensex/backend/internal/models"
)

// FavoriteRepo serves the anonymous "Découvrir" deck. Likes and passes are
// kept in the visitor's browser (ADR-023): the server only receives the ids
// to leave out or to describe.
type FavoriteRepo struct{ base }

type DiscoverRow struct {
	ID                uuid.UUID
	Slug              string
	DisplayName       string
	Headline          string
	PriceFromCents    *int64
	Age               *int16
	Languages         models.StringArray `gorm:"type:text[]"`
	EstablishmentSlug string
	EstablishmentName string
	CityName          *string
}

func (r *FavoriteRepo) published(ctx context.Context) *gorm.DB {
	return r.db(ctx).Table("professional_profiles p").
		Joins("JOIN establishments e ON e.id = p.establishment_id").
		Joins("LEFT JOIN regions c ON c.id = e.city_id").
		Joins("LEFT JOIN regions rg ON rg.id = e.region_id").
		Where("p.is_published AND p.deleted_at IS NULL")
}

const discoverColumns = `p.id, p.slug, p.display_name, p.headline, p.price_from_cents, p.languages, p.age,
	e.slug AS establishment_slug, e.display_name AS establishment_name, c.name AS city_name`

// Discover returns up to limit published profiles matching f and not in
// exclude, in random order, and the number of such profiles in total.
func (r *FavoriteRepo) Discover(ctx context.Context, f CatalogFilter, exclude []uuid.UUID, limit int) ([]DiscoverRow, int64, error) {
	q := applyFilters(r.published(ctx), f)
	if len(exclude) > 0 {
		q = q.Where("NOT (p.id = ANY(CAST(? AS uuid[])))", uuidArray(exclude))
	}
	var total int64
	if err := q.Session(&gorm.Session{}).Count(&total).Error; err != nil {
		return nil, 0, err
	}
	var out []DiscoverRow
	err := q.Select(discoverColumns).Order("random()").Limit(limit).Scan(&out).Error
	return out, total, err
}

// Published returns the published profiles among ids (favorites page).
func (r *FavoriteRepo) Published(ctx context.Context, ids []uuid.UUID) ([]DiscoverRow, error) {
	var out []DiscoverRow
	err := r.published(ctx).Select(discoverColumns).Where("p.id = ANY(CAST(? AS uuid[]))", uuidArray(ids)).
		Order("p.display_name").Scan(&out).Error
	return out, err
}

// uuidArray renders ids as a Postgres array literal: GORM would otherwise
// expand a slice into a parenthesised list.
func uuidArray(ids []uuid.UUID) string {
	parts := make([]string, len(ids))
	for i, id := range ids {
		parts[i] = id.String()
	}
	return "{" + strings.Join(parts, ",") + "}"
}
