package repositories

import (
	"context"
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"

	"opensex/backend/internal/models"
)

type CatalogRepo struct{ base }

type CatalogFilter struct {
	Region        string
	City          string
	Establishment string
	Categories    []string
	PriceMinCents int64
	PriceMaxCents int64
	HeightMin     int
	HeightMax     int
	AgeMin        int
	AgeMax        int
	SkinTones     []string
	Genders       []string
	HairColors    []string
	EyeColors     []string
	BodyTypes     []string
	Languages     []string
	Sort          string
	Today         time.Time
}

type CatalogRow struct {
	ID                uuid.UUID
	Slug              string
	DisplayName       string
	Headline          string
	PriceFromCents    *int64
	PublishedAt       *time.Time
	HeightCm          *int16
	HairColor         *string
	EyeColor          *string
	BodyType          *string
	Languages         models.StringArray `gorm:"type:text[]"`
	EstablishmentID   uuid.UUID
	EstablishmentSlug string
	EstablishmentName string
	CityName          *string
	CitySlug          *string
	RegionName        *string
	RegionSlug        *string
}

// Sorts are mapped from a whitelist, never interpolated from the request.
var catalogSorts = map[string]string{
	"recent":     "p.published_at DESC NULLS LAST, p.id DESC",
	"price_asc":  "p.price_from_cents ASC NULLS LAST, p.id",
	"price_desc": "p.price_from_cents DESC NULLS LAST, p.id",
}

func (r *CatalogRepo) published(ctx context.Context) *gorm.DB {
	return r.db(ctx).Table("professional_profiles p").
		Joins("JOIN establishments e ON e.id = p.establishment_id").
		Joins("LEFT JOIN regions c ON c.id = e.city_id").
		Joins("LEFT JOIN regions rg ON rg.id = e.region_id").
		Where("p.is_published AND p.deleted_at IS NULL")
}

const catalogSelect = `p.id, p.slug, p.display_name, p.headline, p.price_from_cents, p.published_at,
	p.height_cm, p.hair_color, p.eye_color, p.body_type, p.languages,
	e.id AS establishment_id, e.slug AS establishment_slug, e.display_name AS establishment_name,
	c.name AS city_name, c.slug AS city_slug, rg.name AS region_name, rg.slug AS region_slug`

// applyFilters narrows published profiles (catalogue and "Découvrir" deck).
func applyFilters(q *gorm.DB, f CatalogFilter) *gorm.DB {
	if f.Region != "" {
		q = q.Where("rg.slug = ?", f.Region)
	}
	if f.City != "" {
		q = q.Where("c.slug = ?", f.City)
	}
	if f.Establishment != "" {
		q = q.Where("e.slug = ?", f.Establishment)
	}
	if len(f.Categories) > 0 {
		q = q.Where(`EXISTS (SELECT 1 FROM professional_categories pc JOIN categories cat ON cat.id = pc.category_id
			WHERE pc.profile_id = p.id AND cat.slug IN ?)`, f.Categories)
	}
	if f.PriceMinCents > 0 {
		q = q.Where("p.price_from_cents >= ?", f.PriceMinCents)
	}
	if f.PriceMaxCents > 0 {
		q = q.Where("p.price_from_cents <= ?", f.PriceMaxCents)
	}
	if f.HeightMin > 0 {
		q = q.Where("p.height_cm >= ?", f.HeightMin)
	}
	if f.HeightMax > 0 {
		q = q.Where("p.height_cm <= ?", f.HeightMax)
	}
	if len(f.HairColors) > 0 {
		q = q.Where("p.hair_color IN ?", f.HairColors)
	}
	if len(f.EyeColors) > 0 {
		q = q.Where("p.eye_color IN ?", f.EyeColors)
	}
	if len(f.BodyTypes) > 0 {
		q = q.Where("p.body_type IN ?", f.BodyTypes)
	}
	if len(f.Languages) > 0 {
		q = q.Where("p.languages && ?::text[]", models.StringArray(f.Languages))
	}
	if f.AgeMin > 0 {
		q = q.Where("p.age >= ?", f.AgeMin)
	}
	if f.AgeMax > 0 {
		q = q.Where("p.age <= ?", f.AgeMax)
	}
	if len(f.SkinTones) > 0 {
		q = q.Where("p.extra_attributes->>'skin_tone' IN ?", f.SkinTones)
	}
	if len(f.Genders) > 0 {
		q = q.Where("p.extra_attributes->>'gender' IN ?", f.Genders)
	}
	return q
}

func (r *CatalogRepo) Search(ctx context.Context, f CatalogFilter, p Page) ([]CatalogRow, int64, error) {
	q := applyFilters(r.published(ctx), f)
	var total int64
	if err := q.Session(&gorm.Session{}).Count(&total).Error; err != nil {
		return nil, 0, err
	}
	order, ok := catalogSorts[f.Sort]
	if !ok {
		order = catalogSorts["recent"]
	}
	var out []CatalogRow
	err := q.Select(catalogSelect).Order(order).Offset(p.Offset()).Limit(p.PageSize).Scan(&out).Error
	return out, total, err
}

func (r *CatalogRepo) PublishedOfEstablishment(ctx context.Context, establishmentID uuid.UUID, limit int) ([]CatalogRow, error) {
	var out []CatalogRow
	err := r.published(ctx).Select(catalogSelect).Where("p.establishment_id = ?", establishmentID).
		Order(catalogSorts["recent"]).Limit(limit).Scan(&out).Error
	return out, err
}

// CategoriesOf returns the category names of several profiles.
func (r *CatalogRepo) CategoriesOf(ctx context.Context, ids []uuid.UUID) (map[uuid.UUID][]models.Category, error) {
	out := map[uuid.UUID][]models.Category{}
	if len(ids) == 0 {
		return out, nil
	}
	type row struct {
		ProfileID uuid.UUID
		models.Category
	}
	var rows []row
	if err := r.db(ctx).Raw(`SELECT pc.profile_id, c.* FROM professional_categories pc JOIN categories c ON c.id = pc.category_id
		WHERE pc.profile_id IN ? ORDER BY c.position, c.name`, ids).Scan(&rows).Error; err != nil {
		return nil, err
	}
	for _, rw := range rows {
		out[rw.ProfileID] = append(out[rw.ProfileID], rw.Category)
	}
	return out, nil
}
