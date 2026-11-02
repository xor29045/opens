package repositories

import (
	"context"
	"time"

	"opensex/backend/internal/models"
)

type AuditRepo struct{ base }

func (r *AuditRepo) Insert(ctx context.Context, l *models.AuditLog) error {
	return r.db(ctx).Omit("ID").Create(l).Error
}

type AuditFilter struct {
	Category   string
	Action     string
	Severity   string
	ActorID    string
	TargetType string
	TargetID   string
	From, To   *time.Time
	BeforeID   int64 // cursor
}

func (r *AuditRepo) List(ctx context.Context, f AuditFilter, limit int) ([]models.AuditLog, error) {
	q := r.db(ctx).Model(&models.AuditLog{})
	if f.Category != "" {
		q = q.Where("category = ?", f.Category)
	}
	if f.Action != "" {
		q = q.Where("action = ?", f.Action)
	}
	if f.Severity != "" {
		q = q.Where("severity = ?", f.Severity)
	}
	if f.ActorID != "" {
		q = q.Where("actor_user_id::text = ?", f.ActorID)
	}
	if f.TargetType != "" {
		q = q.Where("target_type = ?", f.TargetType)
	}
	if f.TargetID != "" {
		q = q.Where("target_id = ?", f.TargetID)
	}
	if f.From != nil {
		q = q.Where("occurred_at >= ?", *f.From)
	}
	if f.To != nil {
		q = q.Where("occurred_at < ?", *f.To)
	}
	if f.BeforeID > 0 {
		q = q.Where("id < ?", f.BeforeID)
	}
	var out []models.AuditLog
	err := q.Order("id DESC").Limit(limit).Find(&out).Error
	return out, err
}

func (r *AuditRepo) CountSince(ctx context.Context, category string, since time.Time) (int64, error) {
	var n int64
	err := r.db(ctx).Model(&models.AuditLog{}).Where("category = ? AND occurred_at >= ?", category, since).Count(&n).Error
	return n, err
}
