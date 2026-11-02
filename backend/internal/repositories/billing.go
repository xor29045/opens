package repositories

import (
	"context"
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"

	"opensex/backend/internal/models"
)

type BillingRepo struct{ base }

func (r *BillingRepo) ActivePlans(ctx context.Context) ([]models.SubscriptionPlan, error) {
	var out []models.SubscriptionPlan
	err := r.db(ctx).Where("is_active").Order("amount_cents").Find(&out).Error
	return out, err
}

func (r *BillingRepo) PlanByCode(ctx context.Context, code string) (*models.SubscriptionPlan, error) {
	var p models.SubscriptionPlan
	err := r.db(ctx).Where("code = ? AND is_active", code).Take(&p).Error
	return &p, err
}

func (r *BillingRepo) PlanByID(ctx context.Context, id uuid.UUID) (*models.SubscriptionPlan, error) {
	var p models.SubscriptionPlan
	err := r.db(ctx).Where("id = ?", id).Take(&p).Error
	return &p, err
}

// DefaultPlan returns the cheapest active plan (used for limits before any subscription).
func (r *BillingRepo) DefaultPlan(ctx context.Context) (*models.SubscriptionPlan, error) {
	var p models.SubscriptionPlan
	err := r.db(ctx).Where("is_active").Order("amount_cents").Take(&p).Error
	return &p, err
}

// CurrentSubscription returns the non-canceled subscription of an establishment.
func (r *BillingRepo) CurrentSubscription(ctx context.Context, establishmentID uuid.UUID) (*models.Subscription, error) {
	var s models.Subscription
	err := r.db(ctx).Where("establishment_id = ? AND status <> ?", establishmentID, models.SubCanceled).Take(&s).Error
	return &s, err
}

// LatestSubscription returns the most recent subscription, canceled or not.
func (r *BillingRepo) LatestSubscription(ctx context.Context, establishmentID uuid.UUID) (*models.Subscription, error) {
	var s models.Subscription
	err := r.db(ctx).Where("establishment_id = ?", establishmentID).Order("created_at DESC").Take(&s).Error
	return &s, err
}

func (r *BillingRepo) SubscriptionForUpdate(ctx context.Context, id uuid.UUID) (*models.Subscription, error) {
	var s models.Subscription
	err := r.db(ctx).Clauses(clause.Locking{Strength: "UPDATE"}).Where("id = ?", id).Take(&s).Error
	return &s, err
}

func (r *BillingRepo) CreateSubscription(ctx context.Context, s *models.Subscription) error {
	return r.db(ctx).Create(s).Error
}

func (r *BillingRepo) UpdateSubscription(ctx context.Context, id uuid.UUID, cols map[string]any) error {
	cols["updated_at"] = time.Now().UTC()
	return r.db(ctx).Model(&models.Subscription{}).Where("id = ?", id).Updates(cols).Error
}

func (r *BillingRepo) CreatePayment(ctx context.Context, p *models.Payment) error {
	return r.db(ctx).Create(p).Error
}

func (r *BillingRepo) UpdatePayment(ctx context.Context, id uuid.UUID, cols map[string]any) error {
	cols["updated_at"] = time.Now().UTC()
	return r.db(ctx).Model(&models.Payment{}).Where("id = ?", id).Updates(cols).Error
}

func (r *BillingRepo) PaymentByID(ctx context.Context, id uuid.UUID) (*models.Payment, error) {
	var p models.Payment
	err := r.db(ctx).Where("id = ?", id).Take(&p).Error
	return &p, err
}

func (r *BillingRepo) PaymentForUpdate(ctx context.Context, id uuid.UUID) (*models.Payment, error) {
	var p models.Payment
	err := r.db(ctx).Clauses(clause.Locking{Strength: "UPDATE"}).Where("id = ?", id).Take(&p).Error
	return &p, err
}

func (r *BillingRepo) PaymentScoped(ctx context.Context, establishmentID, id uuid.UUID) (*models.Payment, error) {
	var p models.Payment
	err := r.db(ctx).Where("id = ? AND establishment_id = ?", id, establishmentID).Take(&p).Error
	return &p, err
}

// OpenPayment returns a payment of the subscription still awaiting payment.
func (r *BillingRepo) OpenPayment(ctx context.Context, subscriptionID uuid.UUID, now time.Time) (*models.Payment, error) {
	var p models.Payment
	err := r.db(ctx).Where("subscription_id = ? AND status IN ? AND (expires_at IS NULL OR expires_at > ? OR status = ?)",
		subscriptionID, []string{models.PaymentPending, models.PaymentConfirming}, now, models.PaymentConfirming).
		Order("created_at DESC").Take(&p).Error
	return &p, err
}

func (r *BillingRepo) LastPayment(ctx context.Context, subscriptionID uuid.UUID) (*models.Payment, error) {
	var p models.Payment
	err := r.db(ctx).Where("subscription_id = ?", subscriptionID).Order("created_at DESC").Take(&p).Error
	return &p, err
}

type PaymentFilter struct {
	EstablishmentID *uuid.UUID
	Status          string
}

func (r *BillingRepo) Payments(ctx context.Context, f PaymentFilter, p Page) ([]models.Payment, int64, error) {
	q := r.db(ctx).Model(&models.Payment{})
	if f.EstablishmentID != nil {
		q = q.Where("establishment_id = ?", *f.EstablishmentID)
	}
	if f.Status != "" {
		q = q.Where("status = ?", f.Status)
	}
	var total int64
	if err := q.Session(&gorm.Session{}).Count(&total).Error; err != nil {
		return nil, 0, err
	}
	var out []models.Payment
	err := q.Order("created_at DESC").Offset(p.Offset()).Limit(p.PageSize).Find(&out).Error
	return out, total, err
}

// StalePayments returns PENDING/CONFIRMING payments older than cutoff.
func (r *BillingRepo) StalePayments(ctx context.Context, cutoff time.Time, limit int) ([]models.Payment, error) {
	var out []models.Payment
	err := r.db(ctx).Where("status IN ? AND created_at < ?", []string{models.PaymentPending, models.PaymentConfirming}, cutoff).
		Order("created_at").Limit(limit).Find(&out).Error
	return out, err
}

type SubscriptionFilter struct {
	Status string
}

type SubscriptionRow struct {
	models.Subscription
	EstablishmentName string
	EstablishmentSlug string
	PlanCode          string
}

func (r *BillingRepo) Subscriptions(ctx context.Context, f SubscriptionFilter, p Page) ([]SubscriptionRow, int64, error) {
	q := r.db(ctx).Table("subscriptions s").
		Joins("JOIN establishments e ON e.id = s.establishment_id").
		Joins("JOIN subscription_plans pl ON pl.id = s.plan_id")
	if f.Status != "" {
		q = q.Where("s.status = ?", f.Status)
	}
	var total int64
	if err := q.Session(&gorm.Session{}).Count(&total).Error; err != nil {
		return nil, 0, err
	}
	var out []SubscriptionRow
	err := q.Select("s.*, e.display_name AS establishment_name, e.slug AS establishment_slug, pl.code AS plan_code").
		Order("s.current_period_end NULLS LAST, s.created_at DESC").Offset(p.Offset()).Limit(p.PageSize).Scan(&out).Error
	return out, total, err
}

// SubscriptionsDue returns subscriptions in the given statuses whose period
// ends before cutoff.
func (r *BillingRepo) SubscriptionsEndingBefore(ctx context.Context, statuses []string, cutoff time.Time) ([]models.Subscription, error) {
	var out []models.Subscription
	err := r.db(ctx).Where("status IN ? AND current_period_end IS NOT NULL AND current_period_end <= ?", statuses, cutoff).Find(&out).Error
	return out, err
}

func (r *BillingRepo) SubscriptionsByStatus(ctx context.Context, status string) ([]models.Subscription, error) {
	var out []models.Subscription
	err := r.db(ctx).Where("status = ?", status).Find(&out).Error
	return out, err
}

func (r *BillingRepo) CountSubscriptionsByStatus(ctx context.Context) (map[string]int64, error) {
	type row struct {
		Status string
		N      int64
	}
	var rows []row
	err := r.db(ctx).Raw(`SELECT status, count(*) AS n FROM subscriptions GROUP BY status`).Scan(&rows).Error
	out := map[string]int64{}
	for _, rw := range rows {
		out[rw.Status] = rw.N
	}
	return out, err
}

// MRRCents sums the monthly-normalised amount of entitled subscriptions.
func (r *BillingRepo) MRRCents(ctx context.Context) (int64, error) {
	var n *int64
	err := r.db(ctx).Raw(`SELECT COALESCE(sum(pl.amount_cents * 30 / pl.interval_days), 0) FROM subscriptions s
		JOIN subscription_plans pl ON pl.id = s.plan_id WHERE s.status IN ('ACTIVE','TRIALING','PAST_DUE')`).Scan(&n).Error
	if n == nil {
		return 0, err
	}
	return *n, err
}

// ---- invoices

func (r *BillingRepo) CreateInvoice(ctx context.Context, inv *models.Invoice) error {
	return r.db(ctx).Create(inv).Error
}

// NextInvoiceNumber must be called under the invoice-number advisory lock.
func (r *BillingRepo) LastInvoiceNumber(ctx context.Context, prefix string) (string, error) {
	var n *string
	err := r.db(ctx).Raw(`SELECT max(number) FROM invoices WHERE number LIKE ?`, prefix+"%").Scan(&n).Error
	if n == nil {
		return "", err
	}
	return *n, err
}

func (r *BillingRepo) InvoiceByPayment(ctx context.Context, paymentID uuid.UUID) (*models.Invoice, error) {
	var inv models.Invoice
	err := r.db(ctx).Where("payment_id = ?", paymentID).Take(&inv).Error
	return &inv, err
}

func (r *BillingRepo) InvoiceScoped(ctx context.Context, establishmentID, id uuid.UUID) (*models.Invoice, error) {
	var inv models.Invoice
	err := r.db(ctx).Where("id = ? AND establishment_id = ?", id, establishmentID).Take(&inv).Error
	return &inv, err
}

func (r *BillingRepo) Invoices(ctx context.Context, establishmentID uuid.UUID, p Page) ([]models.Invoice, int64, error) {
	q := r.db(ctx).Model(&models.Invoice{}).Where("establishment_id = ?", establishmentID)
	var total int64
	if err := q.Session(&gorm.Session{}).Count(&total).Error; err != nil {
		return nil, 0, err
	}
	var out []models.Invoice
	err := q.Order("issued_at DESC").Offset(p.Offset()).Limit(p.PageSize).Find(&out).Error
	return out, total, err
}

// ---- provider events

// InsertEvent records an IPN; it returns false if the event was already seen.
func (r *BillingRepo) InsertEvent(ctx context.Context, e *models.PaymentProviderEvent) (bool, error) {
	res := r.db(ctx).Clauses(clause.OnConflict{DoNothing: true}).Create(e)
	return res.RowsAffected == 1, res.Error
}

func (r *BillingRepo) MarkEvent(ctx context.Context, id uuid.UUID, at time.Time, procErr *string) error {
	return r.db(ctx).Model(&models.PaymentProviderEvent{}).Where("id = ?", id).
		Updates(map[string]any{"processed_at": at, "processing_error": procErr}).Error
}

func (r *BillingRepo) DeleteEvent(ctx context.Context, id uuid.UUID) error {
	return r.db(ctx).Delete(&models.PaymentProviderEvent{}, "id = ?", id).Error
}
