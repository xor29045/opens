package repositories

import (
	"context"
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm/clause"

	"opensex/backend/internal/models"
)

type FavoriteRepo struct{ base }

// Add is idempotent.
func (r *FavoriteRepo) Add(ctx context.Context, clientID, profileID uuid.UUID, at time.Time) error {
	return r.db(ctx).Clauses(clause.OnConflict{DoNothing: true}).
		Create(&models.Favorite{ClientUserID: clientID, ProfileID: profileID, CreatedAt: at}).Error
}

func (r *FavoriteRepo) Remove(ctx context.Context, clientID, profileID uuid.UUID) (bool, error) {
	res := r.db(ctx).Where("client_user_id = ? AND profile_id = ?", clientID, profileID).Delete(&models.Favorite{})
	return res.RowsAffected > 0, res.Error
}

type FavoriteRow struct {
	ProfileID         uuid.UUID
	CreatedAt         time.Time
	Slug              string
	DisplayName       string
	Headline          string
	IsPublished       bool
	PriceFromCents    *int64
	BirthDateAttested *time.Time
	EstablishmentName string
	CityName          *string
}

func (r *FavoriteRepo) List(ctx context.Context, clientID uuid.UUID, p Page) ([]FavoriteRow, int64, error) {
	var total int64
	if err := r.db(ctx).Model(&models.Favorite{}).Where("client_user_id = ?", clientID).Count(&total).Error; err != nil {
		return nil, 0, err
	}
	var out []FavoriteRow
	err := r.db(ctx).Raw(`SELECT f.profile_id, f.created_at, p.slug, p.display_name, p.headline,
			(p.is_published AND p.deleted_at IS NULL) AS is_published, p.price_from_cents, p.birth_date_attested,
			e.display_name AS establishment_name, c.name AS city_name
		FROM favorites f
		JOIN professional_profiles p ON p.id = f.profile_id
		JOIN establishments e ON e.id = p.establishment_id
		LEFT JOIN regions c ON c.id = e.city_id
		WHERE f.client_user_id = ?
		ORDER BY f.created_at DESC OFFSET ? LIMIT ?`, clientID, p.Offset(), p.PageSize).Scan(&out).Error
	return out, total, err
}

// ---- "Découvrir" deck: dislikes and candidates

func (r *FavoriteRepo) AddDislike(ctx context.Context, clientID, profileID uuid.UUID, at time.Time) error {
	return r.db(ctx).Clauses(clause.OnConflict{DoNothing: true}).
		Create(&models.ClientDislike{ClientUserID: clientID, ProfileID: profileID, CreatedAt: at}).Error
}

func (r *FavoriteRepo) RemoveDislike(ctx context.Context, clientID, profileID uuid.UUID) (bool, error) {
	res := r.db(ctx).Where("client_user_id = ? AND profile_id = ?", clientID, profileID).Delete(&models.ClientDislike{})
	return res.RowsAffected > 0, res.Error
}

func (r *FavoriteRepo) ClearDislikes(ctx context.Context, clientID uuid.UUID) (int64, error) {
	res := r.db(ctx).Where("client_user_id = ?", clientID).Delete(&models.ClientDislike{})
	return res.RowsAffected, res.Error
}

type DiscoverRow struct {
	ID                uuid.UUID
	Slug              string
	DisplayName       string
	Headline          string
	BirthDateAttested *time.Time
	PriceFromCents    *int64
	Languages         models.StringArray `gorm:"type:text[]"`
	EstablishmentSlug string
	EstablishmentName string
	CityName          *string
}

// discoverWhere selects published profiles the client has neither liked
// (favorite) nor passed on (dislike).
const discoverWhere = `p.is_published AND p.deleted_at IS NULL
	AND NOT EXISTS (SELECT 1 FROM favorites f WHERE f.client_user_id = @client AND f.profile_id = p.id)
	AND NOT EXISTS (SELECT 1 FROM client_dislikes d WHERE d.client_user_id = @client AND d.profile_id = p.id)`

// Discover returns up to limit candidates in random order and the number of
// candidates left in total.
func (r *FavoriteRepo) Discover(ctx context.Context, clientID uuid.UUID, limit int) ([]DiscoverRow, int64, error) {
	args := map[string]any{"client": clientID, "limit": limit}
	var total int64
	if err := r.db(ctx).Raw(`SELECT count(*) FROM professional_profiles p WHERE `+discoverWhere, args).Scan(&total).Error; err != nil {
		return nil, 0, err
	}
	var out []DiscoverRow
	err := r.db(ctx).Raw(`SELECT p.id, p.slug, p.display_name, p.headline, p.birth_date_attested, p.price_from_cents, p.languages,
			e.slug AS establishment_slug, e.display_name AS establishment_name, c.name AS city_name
		FROM professional_profiles p
		JOIN establishments e ON e.id = p.establishment_id
		LEFT JOIN regions c ON c.id = e.city_id
		WHERE `+discoverWhere+`
		ORDER BY random() LIMIT @limit`, args).Scan(&out).Error
	return out, total, err
}
