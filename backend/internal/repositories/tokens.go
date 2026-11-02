package repositories

import (
	"context"
	"time"

	"github.com/google/uuid"

	"opensex/backend/internal/models"
)

type TokenRepo struct{ base }

func (r *TokenRepo) CreateRefresh(ctx context.Context, t *models.RefreshToken) error {
	return r.db(ctx).Create(t).Error
}

// RefreshByHashForUpdate locks the token row to serialise concurrent rotations.
func (r *TokenRepo) RefreshByHashForUpdate(ctx context.Context, hash []byte) (*models.RefreshToken, error) {
	var t models.RefreshToken
	err := r.db(ctx).Raw("SELECT * FROM refresh_tokens WHERE token_hash = ? FOR UPDATE", hash).Take(&t).Error
	return &t, err
}

func (r *TokenRepo) RefreshByHash(ctx context.Context, hash []byte) (*models.RefreshToken, error) {
	var t models.RefreshToken
	err := r.db(ctx).Where("token_hash = ?", hash).Take(&t).Error
	return &t, err
}

func (r *TokenRepo) RevokeRefresh(ctx context.Context, id uuid.UUID, reason string, at time.Time) error {
	return r.db(ctx).Model(&models.RefreshToken{}).Where("id = ? AND revoked_at IS NULL", id).
		Updates(map[string]any{"revoked_at": at, "revoked_reason": reason}).Error
}

func (r *TokenRepo) TouchRefresh(ctx context.Context, id uuid.UUID, at time.Time) error {
	return r.db(ctx).Model(&models.RefreshToken{}).Where("id = ?", id).Update("last_used_at", at).Error
}

// RevokeFamily revokes every active token of a session family.
func (r *TokenRepo) RevokeFamily(ctx context.Context, familyID uuid.UUID, reason string, at time.Time) error {
	return r.db(ctx).Model(&models.RefreshToken{}).Where("family_id = ? AND revoked_at IS NULL", familyID).
		Updates(map[string]any{"revoked_at": at, "revoked_reason": reason}).Error
}

// RevokeUser revokes all sessions of a user except keepFamily (if not nil)
// and returns the revoked family ids.
func (r *TokenRepo) RevokeUser(ctx context.Context, userID uuid.UUID, keepFamily *uuid.UUID, reason string, at time.Time) ([]uuid.UUID, error) {
	q := r.db(ctx).Model(&models.RefreshToken{}).Where("user_id = ? AND revoked_at IS NULL", userID)
	if keepFamily != nil {
		q = q.Where("family_id <> ?", *keepFamily)
	}
	var fams []uuid.UUID
	if err := q.Distinct("family_id").Pluck("family_id", &fams).Error; err != nil {
		return nil, err
	}
	q2 := r.db(ctx).Model(&models.RefreshToken{}).Where("user_id = ? AND revoked_at IS NULL", userID)
	if keepFamily != nil {
		q2 = q2.Where("family_id <> ?", *keepFamily)
	}
	return fams, q2.Updates(map[string]any{"revoked_at": at, "revoked_reason": reason}).Error
}

// ActiveSessions returns the latest active token of each family of a user.
func (r *TokenRepo) ActiveSessions(ctx context.Context, userID uuid.UUID, now time.Time) ([]models.RefreshToken, error) {
	var out []models.RefreshToken
	err := r.db(ctx).Raw(`SELECT DISTINCT ON (family_id) * FROM refresh_tokens
		WHERE user_id = ? AND revoked_at IS NULL AND expires_at > ? AND absolute_expires_at > ?
		ORDER BY family_id, created_at DESC`, userID, now, now).Scan(&out).Error
	return out, err
}

func (r *TokenRepo) FamilyOwnedBy(ctx context.Context, familyID, userID uuid.UUID) (bool, error) {
	var n int64
	err := r.db(ctx).Model(&models.RefreshToken{}).Where("family_id = ? AND user_id = ?", familyID, userID).Count(&n).Error
	return n > 0, err
}

func (r *TokenRepo) PurgeExpired(ctx context.Context, cutoff time.Time) (int64, error) {
	res := r.db(ctx).Exec(`DELETE FROM refresh_tokens WHERE absolute_expires_at < ? OR (revoked_at IS NOT NULL AND revoked_at < ?)`, cutoff, cutoff)
	return res.RowsAffected, res.Error
}

// ---- recovery codes

func (r *TokenRepo) ReplaceRecoveryCodes(ctx context.Context, userID uuid.UUID, codes []models.RecoveryCode) error {
	if err := r.db(ctx).Where("user_id = ?", userID).Delete(&models.RecoveryCode{}).Error; err != nil {
		return err
	}
	return r.db(ctx).Create(&codes).Error
}

// ConsumeRecoveryCode marks an unused code as used; it reports whether a code matched.
func (r *TokenRepo) ConsumeRecoveryCode(ctx context.Context, userID uuid.UUID, hash []byte, at time.Time) (bool, error) {
	res := r.db(ctx).Model(&models.RecoveryCode{}).
		Where("user_id = ? AND code_hash = ? AND used_at IS NULL", userID, hash).
		Update("used_at", at)
	return res.RowsAffected == 1, res.Error
}

func (r *TokenRepo) RemainingRecoveryCodes(ctx context.Context, userID uuid.UUID) (int64, error) {
	var n int64
	err := r.db(ctx).Model(&models.RecoveryCode{}).Where("user_id = ? AND used_at IS NULL", userID).Count(&n).Error
	return n, err
}
