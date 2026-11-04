package repositories

import (
	"context"
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"

	"opensex/backend/internal/models"
)

type UserRepo struct{ base }

const userSelect = "users.*, roles.code AS role"

func (r *UserRepo) withRole(ctx context.Context) *gorm.DB {
	return r.db(ctx).Model(&models.User{}).Select(userSelect).
		Joins("LEFT JOIN user_roles ON user_roles.user_id = users.id").
		Joins("LEFT JOIN roles ON roles.id = user_roles.role_id")
}

func (r *UserRepo) Create(ctx context.Context, u *models.User, role string) error {
	if err := r.db(ctx).Omit("Role").Create(u).Error; err != nil {
		return err
	}
	u.Role = role
	return r.db(ctx).Create(&models.UserRole{UserID: u.ID, RoleID: models.RoleIDs[role], GrantedAt: u.CreatedAt}).Error
}

func (r *UserRepo) ByID(ctx context.Context, id uuid.UUID) (*models.User, error) {
	var u models.User
	err := r.withRole(ctx).Where("users.id = ?", id).Take(&u).Error
	return &u, err
}

func (r *UserRepo) ByUsername(ctx context.Context, username string) (*models.User, error) {
	var u models.User
	err := r.withRole(ctx).Where("users.username = ?", username).Take(&u).Error
	return &u, err
}

func (r *UserRepo) Exists(ctx context.Context, username string) (bool, error) {
	var n int64
	err := r.db(ctx).Unscoped().Model(&models.User{}).Where("username = ?", username).Count(&n).Error
	return n > 0, err
}

// Update writes the given columns.
func (r *UserRepo) Update(ctx context.Context, id uuid.UUID, cols map[string]any) error {
	cols["updated_at"] = time.Now().UTC()
	return r.db(ctx).Model(&models.User{}).Where("id = ?", id).Updates(cols).Error
}

type UserFilter struct {
	Role   string
	Status string
	Query  string
}

func (r *UserRepo) List(ctx context.Context, f UserFilter, p Page) ([]models.User, int64, error) {
	q := r.withRole(ctx).Unscoped()
	if f.Role != "" {
		q = q.Where("roles.code = ?", f.Role)
	}
	if f.Status != "" {
		q = q.Where("users.status = ?", f.Status)
	} else {
		q = q.Where("users.deleted_at IS NULL")
	}
	if f.Query != "" {
		q = q.Where("users.username::text ILIKE ?", "%"+escapeLike(f.Query)+"%")
	}
	var total int64
	if err := q.Session(&gorm.Session{}).Count(&total).Error; err != nil {
		return nil, 0, err
	}
	var out []models.User
	err := q.Order("users.created_at DESC").Offset(p.Offset()).Limit(p.PageSize).Find(&out).Error
	return out, total, err
}

// PurgeDeleted hard-deletes users soft-deleted before cutoff that are not
// referenced by an establishment (managers are anonymised and kept).
func (r *UserRepo) PurgeDeleted(ctx context.Context, cutoff time.Time) (int64, error) {
	res := r.db(ctx).Exec(`DELETE FROM users u WHERE u.deleted_at IS NOT NULL AND u.deleted_at < ?
		AND NOT EXISTS (SELECT 1 FROM establishments e WHERE e.manager_user_id = u.id)`, cutoff)
	return res.RowsAffected, res.Error
}

func (r *UserRepo) CountByRoleStatus(ctx context.Context) (map[string]map[string]int64, error) {
	type row struct {
		Role   string
		Status string
		N      int64
	}
	var rows []row
	err := r.db(ctx).Raw(`SELECT roles.code AS role, users.status, count(*) AS n FROM users
		JOIN user_roles ON user_roles.user_id = users.id JOIN roles ON roles.id = user_roles.role_id
		WHERE users.deleted_at IS NULL GROUP BY roles.code, users.status`).Scan(&rows).Error
	out := map[string]map[string]int64{}
	for _, rw := range rows {
		if out[rw.Role] == nil {
			out[rw.Role] = map[string]int64{}
		}
		out[rw.Role][rw.Status] = rw.N
	}
	return out, err
}

func escapeLike(s string) string {
	out := make([]rune, 0, len(s))
	for _, r := range s {
		if r == '%' || r == '_' || r == '\\' {
			out = append(out, '\\')
		}
		out = append(out, r)
	}
	return string(out)
}
