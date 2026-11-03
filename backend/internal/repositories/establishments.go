package repositories

import (
	"context"
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"

	"opensex/backend/internal/models"
)

type EstablishmentRepo struct{ base }

func (r *EstablishmentRepo) Create(ctx context.Context, e *models.Establishment) error {
	return r.db(ctx).Create(e).Error
}

func (r *EstablishmentRepo) ByID(ctx context.Context, id uuid.UUID) (*models.Establishment, error) {
	var e models.Establishment
	err := r.db(ctx).Where("id = ?", id).Take(&e).Error
	return &e, err
}

func (r *EstablishmentRepo) ByIDForUpdate(ctx context.Context, id uuid.UUID) (*models.Establishment, error) {
	var e models.Establishment
	err := r.db(ctx).Raw("SELECT * FROM establishments WHERE id = ? AND deleted_at IS NULL FOR UPDATE", id).Take(&e).Error
	return &e, err
}

func (r *EstablishmentRepo) ByManager(ctx context.Context, managerID uuid.UUID) (*models.Establishment, error) {
	var e models.Establishment
	err := r.db(ctx).Where("manager_user_id = ?", managerID).Take(&e).Error
	return &e, err
}

// ByManagerUnscoped also returns soft-deleted establishments.
func (r *EstablishmentRepo) ByManagerUnscoped(ctx context.Context, managerID uuid.UUID) (*models.Establishment, error) {
	var e models.Establishment
	err := r.db(ctx).Unscoped().Where("manager_user_id = ?", managerID).Take(&e).Error
	return &e, err
}

func (r *EstablishmentRepo) BySlug(ctx context.Context, slug string) (*models.Establishment, error) {
	var e models.Establishment
	err := r.db(ctx).Where("slug = ?", slug).Take(&e).Error
	return &e, err
}

func (r *EstablishmentRepo) SlugTaken(ctx context.Context, slug string, except uuid.UUID) (bool, error) {
	var n int64
	err := r.db(ctx).Unscoped().Model(&models.Establishment{}).Where("slug = ? AND id <> ?", slug, except).Count(&n).Error
	return n > 0, err
}

func (r *EstablishmentRepo) Save(ctx context.Context, e *models.Establishment) error {
	e.UpdatedAt = time.Now().UTC()
	return r.db(ctx).Save(e).Error
}

func (r *EstablishmentRepo) Update(ctx context.Context, id uuid.UUID, cols map[string]any) error {
	cols["updated_at"] = time.Now().UTC()
	return r.db(ctx).Model(&models.Establishment{}).Where("id = ?", id).Updates(cols).Error
}

func (r *EstablishmentRepo) Contacts(ctx context.Context, id uuid.UUID) ([]models.EstablishmentContact, error) {
	var out []models.EstablishmentContact
	err := r.db(ctx).Where("establishment_id = ?", id).Order("position, id").Find(&out).Error
	return out, err
}

func (r *EstablishmentRepo) ReplaceContacts(ctx context.Context, id uuid.UUID, contacts []models.EstablishmentContact) error {
	if err := r.db(ctx).Where("establishment_id = ?", id).Delete(&models.EstablishmentContact{}).Error; err != nil {
		return err
	}
	if len(contacts) == 0 {
		return nil
	}
	return r.db(ctx).Create(&contacts).Error
}

// Selectable lists the houses a professional can choose at registration:
// named, not hidden, manager active.
func (r *EstablishmentRepo) Selectable(ctx context.Context) ([]EstablishmentSummary, error) {
	var out []EstablishmentSummary
	err := r.db(ctx).Raw(`SELECT e.id, e.slug, e.display_name, c.name AS city_name, rg.name AS region_name
		FROM establishments e
		JOIN users m ON m.id = e.manager_user_id AND m.status = 'ACTIVE' AND m.deleted_at IS NULL
		LEFT JOIN regions c ON c.id = e.city_id
		LEFT JOIN regions rg ON rg.id = e.region_id
		WHERE e.deleted_at IS NULL AND e.display_name <> '' AND NOT e.is_hidden
		ORDER BY e.display_name`).Scan(&out).Error
	return out, err
}

type EstablishmentSummary struct {
	ID          uuid.UUID
	Slug        string
	DisplayName string
	CityName    *string
	RegionName  *string
}

type EstablishmentFilter struct {
	SubscriptionStatus string
	Query              string
}

type EstablishmentRow struct {
	models.Establishment
	ManagerUsername    string
	ManagerStatus      string
	SubscriptionStatus *string
	ProfilesCount      int64
	PublishedCount     int64
}

func (r *EstablishmentRepo) AdminList(ctx context.Context, f EstablishmentFilter, p Page) ([]EstablishmentRow, int64, error) {
	q := r.db(ctx).Table("establishments e").
		Joins("JOIN users m ON m.id = e.manager_user_id").
		Joins(`LEFT JOIN LATERAL (SELECT s.status FROM subscriptions s WHERE s.establishment_id = e.id ORDER BY s.created_at DESC LIMIT 1) sub ON true`).
		Where("e.deleted_at IS NULL")
	if f.SubscriptionStatus != "" {
		if f.SubscriptionStatus == "NONE" {
			q = q.Where("sub.status IS NULL")
		} else {
			q = q.Where("sub.status = ?", f.SubscriptionStatus)
		}
	}
	if f.Query != "" {
		like := "%" + escapeLike(f.Query) + "%"
		q = q.Where("(e.display_name ILIKE ? OR e.legal_name ILIKE ? OR m.username::text ILIKE ?)", like, like, like)
	}
	var total int64
	if err := q.Session(&gorm.Session{}).Count(&total).Error; err != nil {
		return nil, 0, err
	}
	var out []EstablishmentRow
	err := q.Select(`e.*, m.username AS manager_username, m.status AS manager_status, sub.status AS subscription_status,
		(SELECT count(*) FROM professional_profiles p WHERE p.establishment_id = e.id AND p.deleted_at IS NULL) AS profiles_count,
		(SELECT count(*) FROM professional_profiles p WHERE p.establishment_id = e.id AND p.is_published) AS published_count`).
		Order("e.created_at DESC").Offset(p.Offset()).Limit(p.PageSize).Scan(&out).Error
	return out, total, err
}

func (r *EstablishmentRepo) Region(ctx context.Context, id *uuid.UUID) (*models.Region, error) {
	if id == nil {
		return nil, nil
	}
	var reg models.Region
	if err := r.db(ctx).Where("id = ?", *id).Take(&reg).Error; err != nil {
		return nil, err
	}
	return &reg, nil
}
