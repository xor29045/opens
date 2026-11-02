package repositories

import (
	"context"

	"github.com/google/uuid"

	"opensex/backend/internal/models"
)

type TaxonomyRepo struct{ base }

func (r *TaxonomyRepo) Regions(ctx context.Context, activeOnly bool) ([]models.Region, error) {
	var out []models.Region
	q := r.db(ctx).Order("kind DESC, name")
	if activeOnly {
		q = q.Where("is_active")
	}
	return out, q.Find(&out).Error
}

func (r *TaxonomyRepo) RegionByID(ctx context.Context, id uuid.UUID) (*models.Region, error) {
	var reg models.Region
	err := r.db(ctx).Where("id = ?", id).Take(&reg).Error
	return &reg, err
}

func (r *TaxonomyRepo) RegionsByIDs(ctx context.Context, ids []uuid.UUID) (map[uuid.UUID]models.Region, error) {
	out := map[uuid.UUID]models.Region{}
	if len(ids) == 0 {
		return out, nil
	}
	var rows []models.Region
	if err := r.db(ctx).Where("id IN ?", ids).Find(&rows).Error; err != nil {
		return nil, err
	}
	for _, rg := range rows {
		out[rg.ID] = rg
	}
	return out, nil
}

func (r *TaxonomyRepo) SaveRegion(ctx context.Context, reg *models.Region) error {
	return r.db(ctx).Save(reg).Error
}

func (r *TaxonomyRepo) CreateRegion(ctx context.Context, reg *models.Region) error {
	return r.db(ctx).Create(reg).Error
}

func (r *TaxonomyRepo) DeleteRegion(ctx context.Context, id uuid.UUID) error {
	return r.db(ctx).Delete(&models.Region{}, "id = ?", id).Error
}

func (r *TaxonomyRepo) RegionInUse(ctx context.Context, id uuid.UUID) (bool, error) {
	var n int64
	err := r.db(ctx).Raw(`SELECT (SELECT count(*) FROM establishments WHERE region_id = @id OR city_id = @id)
		+ (SELECT count(*) FROM regions WHERE parent_id = @id)`, map[string]any{"id": id}).Scan(&n).Error
	return n > 0, err
}

func (r *TaxonomyRepo) Categories(ctx context.Context, activeOnly bool) ([]models.Category, error) {
	var out []models.Category
	q := r.db(ctx).Order("position, name")
	if activeOnly {
		q = q.Where("is_active")
	}
	return out, q.Find(&out).Error
}

func (r *TaxonomyRepo) CategoryByID(ctx context.Context, id uuid.UUID) (*models.Category, error) {
	var c models.Category
	err := r.db(ctx).Where("id = ?", id).Take(&c).Error
	return &c, err
}

// SelectableCategories returns the active service categories among ids
// (children of an active domain).
func (r *TaxonomyRepo) SelectableCategories(ctx context.Context, ids []uuid.UUID) ([]models.Category, error) {
	var out []models.Category
	if len(ids) == 0 {
		return out, nil
	}
	err := r.db(ctx).Table("categories c").Select("c.*").
		Joins("JOIN categories parent ON parent.id = c.parent_id AND parent.is_active").
		Where("c.id IN ? AND c.is_active", ids).Scan(&out).Error
	return out, err
}

func (r *TaxonomyRepo) CreateCategory(ctx context.Context, c *models.Category) error {
	return r.db(ctx).Create(c).Error
}

func (r *TaxonomyRepo) SaveCategory(ctx context.Context, c *models.Category) error {
	return r.db(ctx).Save(c).Error
}

func (r *TaxonomyRepo) DeleteCategory(ctx context.Context, id uuid.UUID) error {
	return r.db(ctx).Delete(&models.Category{}, "id = ?", id).Error
}

func (r *TaxonomyRepo) CategoryInUse(ctx context.Context, id uuid.UUID) (bool, error) {
	var n int64
	err := r.db(ctx).Raw(`SELECT (SELECT count(*) FROM professional_categories WHERE category_id = @id)
		+ (SELECT count(*) FROM categories WHERE parent_id = @id)`, map[string]any{"id": id}).Scan(&n).Error
	return n > 0, err
}
