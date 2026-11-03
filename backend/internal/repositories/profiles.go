package repositories

import (
	"context"
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"

	"opensex/backend/internal/models"
)

type ProfileRepo struct{ base }

func (r *ProfileRepo) Create(ctx context.Context, p *models.Profile) error {
	return r.db(ctx).Create(p).Error
}

func (r *ProfileRepo) ByID(ctx context.Context, id uuid.UUID) (*models.Profile, error) {
	var p models.Profile
	err := r.db(ctx).Where("id = ?", id).Take(&p).Error
	return &p, err
}

// ScopedByID enforces the ownership chain profile → establishment (§5.1).
func (r *ProfileRepo) ScopedByID(ctx context.Context, establishmentID, id uuid.UUID) (*models.Profile, error) {
	var p models.Profile
	err := r.db(ctx).Where("id = ? AND establishment_id = ?", id, establishmentID).Take(&p).Error
	return &p, err
}

func (r *ProfileRepo) ScopedByIDForUpdate(ctx context.Context, establishmentID, id uuid.UUID) (*models.Profile, error) {
	var p models.Profile
	err := r.db(ctx).Raw(`SELECT * FROM professional_profiles WHERE id = ? AND establishment_id = ? AND deleted_at IS NULL FOR UPDATE`, id, establishmentID).Take(&p).Error
	return &p, err
}

// ExistsAnywhere reports whether a profile id exists (used to detect IDOR attempts).
func (r *ProfileRepo) ExistsAnywhere(ctx context.Context, id uuid.UUID) bool {
	var n int64
	r.db(ctx).Model(&models.Profile{}).Where("id = ?", id).Count(&n)
	return n > 0
}

func (r *ProfileRepo) ByUser(ctx context.Context, userID uuid.UUID) (*models.Profile, error) {
	var p models.Profile
	err := r.db(ctx).Where("user_id = ?", userID).Take(&p).Error
	return &p, err
}

func (r *ProfileRepo) BySlug(ctx context.Context, slug string) (*models.Profile, error) {
	var p models.Profile
	err := r.db(ctx).Where("slug = ?", slug).Take(&p).Error
	return &p, err
}

func (r *ProfileRepo) SlugTaken(ctx context.Context, slug string) (bool, error) {
	var n int64
	err := r.db(ctx).Unscoped().Model(&models.Profile{}).Where("slug = ?", slug).Count(&n).Error
	return n > 0, err
}

func (r *ProfileRepo) Update(ctx context.Context, id uuid.UUID, cols map[string]any) error {
	cols["updated_at"] = time.Now().UTC()
	return r.db(ctx).Model(&models.Profile{}).Where("id = ?", id).Updates(cols).Error
}

func (r *ProfileRepo) SoftDelete(ctx context.Context, id uuid.UUID, at time.Time) error {
	return r.db(ctx).Model(&models.Profile{}).Where("id = ?", id).
		Updates(map[string]any{"deleted_at": at, "is_published": false, "published_at": nil, "updated_at": at}).Error
}

func (r *ProfileRepo) HardDelete(ctx context.Context, id uuid.UUID) error {
	return r.db(ctx).Unscoped().Delete(&models.Profile{}, "id = ?", id).Error
}

type ProfileFilter struct {
	EstablishmentID *uuid.UUID
	Status          string
	Published       *bool
	Linked          *bool
	Hidden          *bool
	Query           string
}

func (r *ProfileRepo) List(ctx context.Context, f ProfileFilter, p Page) ([]models.Profile, int64, error) {
	q := r.db(ctx).Model(&models.Profile{})
	if f.EstablishmentID != nil {
		q = q.Where("establishment_id = ?", *f.EstablishmentID)
	}
	if f.Status != "" {
		q = q.Where("status = ?", f.Status)
	}
	if f.Published != nil {
		q = q.Where("is_published = ?", *f.Published)
	}
	if f.Linked != nil {
		if *f.Linked {
			q = q.Where("user_id IS NOT NULL")
		} else {
			q = q.Where("user_id IS NULL")
		}
	}
	if f.Hidden != nil {
		q = q.Where("is_hidden = ?", *f.Hidden)
	}
	if f.Query != "" {
		q = q.Where("display_name ILIKE ?", "%"+escapeLike(f.Query)+"%")
	}
	var total int64
	if err := q.Session(&gorm.Session{}).Count(&total).Error; err != nil {
		return nil, 0, err
	}
	var out []models.Profile
	err := q.Order("created_at DESC").Offset(p.Offset()).Limit(p.PageSize).Find(&out).Error
	return out, total, err
}

func (r *ProfileRepo) AllOfEstablishment(ctx context.Context, establishmentID uuid.UUID) ([]models.Profile, error) {
	var out []models.Profile
	err := r.db(ctx).Where("establishment_id = ?", establishmentID).Order("created_at").Find(&out).Error
	return out, err
}

func (r *ProfileRepo) CountPending(ctx context.Context, establishmentID uuid.UUID) (int64, error) {
	var n int64
	err := r.db(ctx).Model(&models.Profile{}).Where("establishment_id = ? AND status = ?", establishmentID, models.ProfilePendingApproval).Count(&n).Error
	return n, err
}

// LinkedUserIDs returns the professional accounts attached to an establishment.
func (r *ProfileRepo) LinkedUserIDs(ctx context.Context, establishmentID uuid.UUID) ([]uuid.UUID, error) {
	var ids []uuid.UUID
	err := r.db(ctx).Unscoped().Model(&models.Profile{}).Where("establishment_id = ? AND user_id IS NOT NULL", establishmentID).Pluck("user_id", &ids).Error
	return ids, err
}

func (r *ProfileRepo) PurgeDeleted(ctx context.Context, cutoff time.Time) ([]uuid.UUID, error) {
	var ids []uuid.UUID
	if err := r.db(ctx).Unscoped().Model(&models.Profile{}).Where("deleted_at IS NOT NULL AND deleted_at < ?", cutoff).Pluck("id", &ids).Error; err != nil {
		return nil, err
	}
	return ids, nil
}

func (r *ProfileRepo) CountPublished(ctx context.Context) (int64, error) {
	var n int64
	err := r.db(ctx).Model(&models.Profile{}).Where("is_published").Count(&n).Error
	return n, err
}

// ---- photos

func (r *ProfileRepo) Photos(ctx context.Context, profileID uuid.UUID) ([]models.ProfilePhoto, error) {
	var out []models.ProfilePhoto
	err := r.db(ctx).Where("profile_id = ?", profileID).Order("is_primary DESC, position, created_at").Find(&out).Error
	return out, err
}

func (r *ProfileRepo) PhotosUnscoped(ctx context.Context, profileIDs []uuid.UUID) ([]models.ProfilePhoto, error) {
	var out []models.ProfilePhoto
	if len(profileIDs) == 0 {
		return out, nil
	}
	err := r.db(ctx).Where("profile_id IN ?", profileIDs).Find(&out).Error
	return out, err
}

func (r *ProfileRepo) PrimaryPhotos(ctx context.Context, profileIDs []uuid.UUID) (map[uuid.UUID]models.ProfilePhoto, error) {
	out := map[uuid.UUID]models.ProfilePhoto{}
	if len(profileIDs) == 0 {
		return out, nil
	}
	var rows []models.ProfilePhoto
	if err := r.db(ctx).Where("profile_id IN ? AND is_primary", profileIDs).Find(&rows).Error; err != nil {
		return nil, err
	}
	for _, p := range rows {
		out[p.ProfileID] = p
	}
	return out, nil
}

func (r *ProfileRepo) PhotoScoped(ctx context.Context, profileID, photoID uuid.UUID) (*models.ProfilePhoto, error) {
	var p models.ProfilePhoto
	err := r.db(ctx).Where("id = ? AND profile_id = ?", photoID, profileID).Take(&p).Error
	return &p, err
}

func (r *ProfileRepo) CountPhotos(ctx context.Context, profileID uuid.UUID) (int64, error) {
	var n int64
	err := r.db(ctx).Model(&models.ProfilePhoto{}).Where("profile_id = ?", profileID).Count(&n).Error
	return n, err
}

func (r *ProfileRepo) CreatePhoto(ctx context.Context, p *models.ProfilePhoto) error {
	return r.db(ctx).Create(p).Error
}

func (r *ProfileRepo) DeletePhoto(ctx context.Context, id uuid.UUID) error {
	return r.db(ctx).Delete(&models.ProfilePhoto{}, "id = ?", id).Error
}

func (r *ProfileRepo) SetPrimaryPhoto(ctx context.Context, profileID, photoID uuid.UUID) error {
	if err := r.db(ctx).Model(&models.ProfilePhoto{}).Where("profile_id = ? AND is_primary", profileID).Update("is_primary", false).Error; err != nil {
		return err
	}
	return r.db(ctx).Model(&models.ProfilePhoto{}).Where("id = ? AND profile_id = ?", photoID, profileID).Update("is_primary", true).Error
}

func (r *ProfileRepo) SetPhotoPosition(ctx context.Context, profileID, photoID uuid.UUID, pos int) error {
	return r.db(ctx).Model(&models.ProfilePhoto{}).Where("id = ? AND profile_id = ?", photoID, profileID).Update("position", pos).Error
}

func (r *ProfileRepo) HasPrimaryPhoto(ctx context.Context, profileID uuid.UUID) (bool, error) {
	var n int64
	err := r.db(ctx).Model(&models.ProfilePhoto{}).Where("profile_id = ? AND is_primary", profileID).Count(&n).Error
	return n > 0, err
}

// ---- contacts, rates, categories

func (r *ProfileRepo) Contacts(ctx context.Context, profileID uuid.UUID) ([]models.ProfileContact, error) {
	var out []models.ProfileContact
	err := r.db(ctx).Where("profile_id = ?", profileID).Order("position, id").Find(&out).Error
	return out, err
}

func (r *ProfileRepo) ReplaceContacts(ctx context.Context, profileID uuid.UUID, cs []models.ProfileContact) error {
	if err := r.db(ctx).Where("profile_id = ?", profileID).Delete(&models.ProfileContact{}).Error; err != nil {
		return err
	}
	if len(cs) == 0 {
		return nil
	}
	return r.db(ctx).Create(&cs).Error
}

func (r *ProfileRepo) Rates(ctx context.Context, profileID uuid.UUID) ([]models.ProfileRate, error) {
	var out []models.ProfileRate
	err := r.db(ctx).Where("profile_id = ?", profileID).Order("position, id").Find(&out).Error
	return out, err
}

func (r *ProfileRepo) ReplaceRates(ctx context.Context, profileID uuid.UUID, rs []models.ProfileRate) error {
	if err := r.db(ctx).Where("profile_id = ?", profileID).Delete(&models.ProfileRate{}).Error; err != nil {
		return err
	}
	if len(rs) == 0 {
		return nil
	}
	return r.db(ctx).Create(&rs).Error
}

func (r *ProfileRepo) Categories(ctx context.Context, profileID uuid.UUID) ([]models.Category, error) {
	var out []models.Category
	err := r.db(ctx).Table("categories c").Select("c.*").
		Joins("JOIN professional_categories pc ON pc.category_id = c.id").
		Where("pc.profile_id = ?", profileID).Order("c.position, c.name").Scan(&out).Error
	return out, err
}

func (r *ProfileRepo) ReplaceCategories(ctx context.Context, profileID uuid.UUID, ids []uuid.UUID) error {
	if err := r.db(ctx).Where("profile_id = ?", profileID).Delete(&models.ProfessionalCategory{}).Error; err != nil {
		return err
	}
	if len(ids) == 0 {
		return nil
	}
	rows := make([]models.ProfessionalCategory, len(ids))
	for i, id := range ids {
		rows[i] = models.ProfessionalCategory{ProfileID: profileID, CategoryID: id}
	}
	return r.db(ctx).Create(&rows).Error
}

// ActivateAttestedDrafts puts in service every attested DRAFT profile of an
// establishment (used when its subscription gets paid). Pending
// registrations (not attested) and archived profiles are left untouched.
func (r *ProfileRepo) ActivateAttestedDrafts(ctx context.Context, establishmentID uuid.UUID, at time.Time) ([]uuid.UUID, error) {
	var ids []uuid.UUID
	err := r.db(ctx).Raw(`UPDATE professional_profiles SET status = 'ACTIVE', updated_at = ?
		WHERE establishment_id = ? AND status = 'DRAFT' AND attested_at IS NOT NULL AND deleted_at IS NULL
		RETURNING id`, at, establishmentID).Scan(&ids).Error
	return ids, err
}
