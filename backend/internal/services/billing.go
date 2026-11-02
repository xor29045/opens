package services

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/google/uuid"

	"opensex/backend/internal/apperr"
	"opensex/backend/internal/database"
	"opensex/backend/internal/models"
	"opensex/backend/internal/rbac"
	"opensex/backend/internal/repositories"
	"opensex/backend/pkg/payment"
	"opensex/backend/pkg/pdf"
	"opensex/backend/pkg/ratelimit"
)

// BillingService runs the subscription life cycle of establishments (§14):
// invoices per period, IPN processing with counter-verification, grace,
// cancellation, internal invoices. Only verified IPNs (or the provider API)
// change a payment (RM-31).
type BillingService struct{ *Services }

const (
	renewalLead        = 5 * 24 * time.Hour
	cancelAfterPastDue = 14 * 24 * time.Hour
	incompleteTTL      = 24 * time.Hour
	staleAfter         = 30 * time.Minute
)

var LimitPayment = []ratelimit.Limit{ratelimit.PerHour(10)}

func (s *BillingService) planForLimits(ctx context.Context, establishmentID uuid.UUID) (*models.SubscriptionPlan, error) {
	if sub, err := s.Repos.Billing.CurrentSubscription(ctx, establishmentID); err == nil {
		if p, err := s.Repos.Billing.PlanByID(ctx, sub.PlanID); err == nil {
			return p, nil
		}
	}
	p, err := s.Repos.Billing.DefaultPlan(ctx)
	if err != nil {
		return nil, apperr.Internal(fmt.Errorf("no active plan: %w", err))
	}
	return p, nil
}

func (s *BillingService) Plans(ctx context.Context) ([]models.SubscriptionPlan, error) {
	p, err := s.Repos.Billing.ActivePlans(ctx)
	if err != nil {
		return nil, apperr.Internal(err)
	}
	return p, nil
}

// ---------------------------------------------------------------- manager views

type Reminder struct {
	Level    string     `json:"level"` // none | renewal_due | past_due | grace_over
	DueAt    *time.Time `json:"due_at,omitempty"`
	DaysLeft *int       `json:"days_left,omitempty"`
}

type SubscriptionView struct {
	Subscription *models.Subscription
	Plan         *models.SubscriptionPlan
	Entitled     bool
	OpenPayment  *models.Payment
	Reminder     Reminder
}

func (s *BillingService) Current(ctx context.Context, actor *rbac.Actor) (*SubscriptionView, error) {
	out := &SubscriptionView{Reminder: Reminder{Level: "none"}}
	now := s.now()
	entitled, _, err := s.Repos.Profiles.HasEntitlement(ctx, actor.EstablishmentID, now)
	if err != nil {
		return nil, apperr.Internal(err)
	}
	out.Entitled = entitled
	sub, err := s.Repos.Billing.LatestSubscription(ctx, actor.EstablishmentID)
	if err != nil {
		if database.IsNotFound(err) {
			return out, nil
		}
		return nil, apperr.Internal(err)
	}
	out.Subscription = sub
	if out.Plan, err = s.Repos.Billing.PlanByID(ctx, sub.PlanID); err != nil {
		return nil, apperr.Internal(err)
	}
	if p, err := s.Repos.Billing.OpenPayment(ctx, sub.ID, now); err == nil {
		out.OpenPayment = p
	}
	out.Reminder = reminderFor(sub, out.Plan, now)
	return out, nil
}

// reminderFor computes the in-app reminder state (RM-32, RM-33) — there is
// no e-mail, the dashboard shows a banner and a badge.
func reminderFor(sub *models.Subscription, plan *models.SubscriptionPlan, now time.Time) Reminder {
	if sub.CurrentPeriodEnd == nil || sub.CancelAtPeriodEnd {
		return Reminder{Level: "none"}
	}
	end := *sub.CurrentPeriodEnd
	days := int(end.Sub(now).Hours() / 24)
	switch sub.Status {
	case models.SubActive, models.SubTrialing:
		if end.Sub(now) <= renewalLead {
			return Reminder{Level: "renewal_due", DueAt: &end, DaysLeft: &days}
		}
	case models.SubPastDue:
		grace := end.AddDate(0, 0, plan.GraceDays)
		if sub.GraceUntil != nil {
			grace = *sub.GraceUntil
		}
		left := int(grace.Sub(now).Hours() / 24)
		if now.Before(grace) {
			return Reminder{Level: "past_due", DueAt: &grace, DaysLeft: &left}
		}
		return Reminder{Level: "grace_over", DueAt: &grace}
	}
	return Reminder{Level: "none"}
}

// Subscribe starts a subscription for a VERIFIED establishment (RM-37) and
// returns the payment carrying the provider invoice URL.
func (s *BillingService) Subscribe(ctx context.Context, actor *rbac.Actor, planCode string) (*models.Payment, error) {
	if res, err := s.Limiter.Allow(ctx, "payment:"+actor.UserID.String(), LimitPayment...); err == nil && !res.Allowed {
		return nil, apperr.RateLimited(res.RetryAfter)
	}
	var (
		pay    *models.Payment
		reused bool
		plan   *models.SubscriptionPlan
		est    *models.Establishment
	)
	err := s.tx(ctx, func(ctx context.Context) error {
		var err error
		est, err = s.Repos.Establishments.ByIDForUpdate(ctx, actor.EstablishmentID)
		if err != nil {
			return apperr.NotFound("establishment")
		}
		if planCode == "" {
			plan, err = s.Repos.Billing.DefaultPlan(ctx)
		} else {
			plan, err = s.Repos.Billing.PlanByCode(ctx, planCode)
		}
		if err != nil {
			return apperr.Invalid("plan_code", "unknown_plan")
		}
		now := s.now()
		sub, err := s.Repos.Billing.CurrentSubscription(ctx, est.ID)
		switch {
		case err == nil && sub.Status != models.SubIncomplete:
			return apperr.Conflict("already_subscribed", "the establishment already has a subscription")
		case err == nil:
			if open, err := s.Repos.Billing.OpenPayment(ctx, sub.ID, now); err == nil && open.InvoiceURL != nil {
				pay, reused = open, true
				return nil
			}
			if sub.PlanID != plan.ID {
				if err := s.Repos.Billing.UpdateSubscription(ctx, sub.ID, map[string]any{"plan_id": plan.ID}); err != nil {
					return apperr.Internal(err)
				}
			}
		case database.IsNotFound(err):
			sub = &models.Subscription{ID: models.NewID(), EstablishmentID: est.ID, PlanID: plan.ID, Status: models.SubIncomplete,
				Provider: s.Payment.Name(), CreatedAt: now, UpdatedAt: now}
			if err := s.Repos.Billing.CreateSubscription(ctx, sub); err != nil {
				if database.IsUniqueViolation(err) {
					return apperr.Conflict("already_subscribed", "the establishment already has a subscription")
				}
				return apperr.Internal(err)
			}
			s.Audit.Record(ctx, AuditEvent{Action: "billing.subscription.created", Category: models.AuditBilling,
				TargetType: "subscription", TargetID: sub.ID.String(), Metadata: map[string]any{"plan": plan.Code}})
		default:
			return apperr.Internal(err)
		}
		pay, err = s.newPayment(ctx, sub, plan, nil)
		return err
	})
	if err != nil || reused {
		return pay, err
	}
	return s.attachInvoice(ctx, pay, est)
}

// Renew creates (or returns) the invoice of the next period / the overdue one.
func (s *BillingService) Renew(ctx context.Context, actor *rbac.Actor) (*models.Payment, error) {
	if res, err := s.Limiter.Allow(ctx, "payment:"+actor.UserID.String(), LimitPayment...); err == nil && !res.Allowed {
		return nil, apperr.RateLimited(res.RetryAfter)
	}
	est, err := s.Repos.Establishments.ByID(ctx, actor.EstablishmentID)
	if err != nil {
		return nil, apperr.NotFound("establishment")
	}
	pay, reused, err := s.ensureRenewalPayment(ctx, est.ID)
	if err != nil || reused {
		return pay, err
	}
	return s.attachInvoice(ctx, pay, est)
}

// ensureRenewalPayment returns the open payment of the current subscription
// or creates a new PENDING one (without provider invoice yet).
func (s *BillingService) ensureRenewalPayment(ctx context.Context, establishmentID uuid.UUID) (*models.Payment, bool, error) {
	var (
		pay    *models.Payment
		reused bool
	)
	err := s.tx(ctx, func(ctx context.Context) error {
		sub, err := s.Repos.Billing.CurrentSubscription(ctx, establishmentID)
		if err != nil {
			if database.IsNotFound(err) {
				return apperr.Conflict("no_subscription", "no subscription to renew")
			}
			return apperr.Internal(err)
		}
		if sub, err = s.Repos.Billing.SubscriptionForUpdate(ctx, sub.ID); err != nil {
			return apperr.Internal(err)
		}
		switch sub.Status {
		case models.SubActive, models.SubTrialing, models.SubPastDue:
		default:
			return apperr.Conflict("invalid_state", "subscription is "+strings.ToLower(sub.Status))
		}
		if open, err := s.Repos.Billing.OpenPayment(ctx, sub.ID, s.now()); err == nil {
			if open.InvoiceURL != nil {
				pay, reused = open, true
				return nil
			}
			pay = open
			return nil
		}
		plan, err := s.Repos.Billing.PlanByID(ctx, sub.PlanID)
		if err != nil {
			return apperr.Internal(err)
		}
		pay, err = s.newPayment(ctx, sub, plan, sub.CurrentPeriodEnd)
		return err
	})
	return pay, reused, err
}

func (s *BillingService) newPayment(ctx context.Context, sub *models.Subscription, plan *models.SubscriptionPlan, periodStart *time.Time) (*models.Payment, error) {
	now := s.now()
	p := &models.Payment{ID: models.NewID(), SubscriptionID: sub.ID, EstablishmentID: sub.EstablishmentID,
		AmountCents: plan.AmountCents, Currency: plan.Currency, Provider: s.Payment.Name(), Status: models.PaymentPending,
		ExpiresAt: ptr(now.Add(payment.InvoiceTTL)), CreatedAt: now, UpdatedAt: now}
	if periodStart != nil && periodStart.After(now) {
		end := periodStart.AddDate(0, 0, plan.IntervalDays)
		p.PeriodStart, p.PeriodEnd = periodStart, &end
	}
	if err := s.Repos.Billing.CreatePayment(ctx, p); err != nil {
		return nil, apperr.Internal(err)
	}
	s.Audit.Record(ctx, AuditEvent{Action: "billing.payment.created", Category: models.AuditBilling, TargetType: "payment", TargetID: p.ID.String()})
	return p, nil
}

// attachInvoice creates the provider invoice outside of any DB transaction.
func (s *BillingService) attachInvoice(ctx context.Context, p *models.Payment, est *models.Establishment) (*models.Payment, error) {
	back := s.Cfg.BaseURL + "/gerant/dashboard/abonnement?payment=" + p.ID.String()
	inv, err := s.Payment.CreateInvoice(ctx, payment.InvoiceRequest{
		OrderID: p.ID.String(), AmountCents: p.AmountCents, Currency: p.Currency,
		Description:    "Abonnement plateforme — " + est.LegalName,
		IPNCallbackURL: s.Cfg.NOWPaymentsIPNCallbackURL, SuccessURL: back, CancelURL: back + "&canceled=1",
	})
	if err != nil {
		_ = s.Repos.Billing.UpdatePayment(ctx, p.ID, map[string]any{"status": models.PaymentFailed, "provider_status": "invoice_creation_failed"})
		s.Logger.ErrorContext(ctx, "invoice creation failed", "payment_id", p.ID, "error", err)
		return nil, apperr.New(apperr.KindBadGateway, "payment_provider_unavailable", "the payment provider is unavailable, try again later")
	}
	if err := s.Repos.Billing.UpdatePayment(ctx, p.ID, map[string]any{"provider_invoice_id": inv.ID, "invoice_url": inv.URL}); err != nil {
		return nil, apperr.Internal(err)
	}
	p.ProviderInvoiceID, p.InvoiceURL = &inv.ID, &inv.URL
	return p, nil
}

func (s *BillingService) setCancel(ctx context.Context, actor *rbac.Actor, cancel bool) (*SubscriptionView, error) {
	err := s.tx(ctx, func(ctx context.Context) error {
		sub, err := s.Repos.Billing.CurrentSubscription(ctx, actor.EstablishmentID)
		if err != nil {
			return apperr.Conflict("no_subscription", "no active subscription")
		}
		if sub.Status != models.SubActive && sub.Status != models.SubTrialing {
			return apperr.Conflict("invalid_state", "subscription is "+strings.ToLower(sub.Status))
		}
		if sub.CancelAtPeriodEnd == cancel {
			return nil
		}
		if err := s.Repos.Billing.UpdateSubscription(ctx, sub.ID, map[string]any{"cancel_at_period_end": cancel}); err != nil {
			return apperr.Internal(err)
		}
		action := "billing.subscription.cancel_scheduled"
		if !cancel {
			action = "billing.subscription.resumed"
		}
		s.Audit.Record(ctx, AuditEvent{Action: action, Category: models.AuditBilling, TargetType: "subscription", TargetID: sub.ID.String()})
		return s.Publication.Recompute(ctx, actor.EstablishmentID)
	})
	if err != nil {
		return nil, err
	}
	return s.Current(ctx, actor)
}

func (s *BillingService) Cancel(ctx context.Context, actor *rbac.Actor) (*SubscriptionView, error) {
	return s.setCancel(ctx, actor, true)
}

func (s *BillingService) Resume(ctx context.Context, actor *rbac.Actor) (*SubscriptionView, error) {
	return s.setCancel(ctx, actor, false)
}

func (s *BillingService) ListPayments(ctx context.Context, actor *rbac.Actor, page repositories.Page) ([]models.Payment, int64, error) {
	rows, total, err := s.Repos.Billing.Payments(ctx, repositories.PaymentFilter{EstablishmentID: &actor.EstablishmentID}, page)
	if err != nil {
		return nil, 0, apperr.Internal(err)
	}
	return rows, total, nil
}

func (s *BillingService) GetPayment(ctx context.Context, actor *rbac.Actor, id uuid.UUID) (*models.Payment, error) {
	p, err := s.Repos.Billing.PaymentScoped(ctx, actor.EstablishmentID, id)
	if err != nil {
		if database.IsNotFound(err) {
			return nil, apperr.NotFound("payment")
		}
		return nil, apperr.Internal(err)
	}
	return p, nil
}

func (s *BillingService) Invoices(ctx context.Context, actor *rbac.Actor, page repositories.Page) ([]models.Invoice, int64, error) {
	rows, total, err := s.Repos.Billing.Invoices(ctx, actor.EstablishmentID, page)
	if err != nil {
		return nil, 0, apperr.Internal(err)
	}
	return rows, total, nil
}

// InvoicePDF renders an invoice of the actor's establishment.
func (s *BillingService) InvoicePDF(ctx context.Context, actor *rbac.Actor, id uuid.UUID) (*models.Invoice, []byte, error) {
	inv, err := s.Repos.Billing.InvoiceScoped(ctx, actor.EstablishmentID, id)
	if err != nil {
		if database.IsNotFound(err) {
			return nil, nil, apperr.NotFound("invoice")
		}
		return nil, nil, apperr.Internal(err)
	}
	var snap billingSnapshot
	_ = json.Unmarshal(inv.BillingSnapshot, &snap)
	return inv, renderInvoice(inv, snap), nil
}

// ---------------------------------------------------------------- IPN

// HandleIPN verifies, de-duplicates, counter-verifies and applies an IPN
// (§14.3). A returned error with KindUnauthorized maps to 401, any other to
// 500 so that NOWPayments retries.
func (s *BillingService) HandleIPN(ctx context.Context, body []byte, signature string) error {
	ipn, err := s.Payment.ParseIPN(body, signature)
	if err != nil {
		s.Audit.Security(ctx, "billing.ipn.invalid_signature", models.SeverityWarn, map[string]any{"provider": s.Payment.Name()})
		return apperr.Unauthorized("invalid_signature")
	}
	now := s.now()
	ev := &models.PaymentProviderEvent{ID: models.NewID(), Provider: s.Payment.Name(), EventKey: ipn.EventKey,
		Payload: models.JSON(body), SignatureValid: true, ReceivedAt: now}
	inserted, err := s.Repos.Billing.InsertEvent(ctx, ev)
	if err != nil {
		return apperr.Internal(err)
	}
	if !inserted {
		return nil // replay: no effect
	}
	ps, err := s.Payment.GetPayment(ctx, ipn.PaymentID)
	if err != nil {
		_ = s.Repos.Billing.DeleteEvent(ctx, ev.ID) // allow the provider's retry
		return apperr.Internal(fmt.Errorf("counter-verification failed: %w", err))
	}
	procErr := s.applyProviderStatus(ctx, ps, "ipn")
	var msg *string
	if procErr != nil {
		msg = ptr(procErr.Error())
		if apperr.IsKind(procErr, apperr.KindInternal) {
			_ = s.Repos.Billing.DeleteEvent(ctx, ev.ID)
			return procErr
		}
	}
	_ = s.Repos.Billing.MarkEvent(ctx, ev.ID, s.now(), msg)
	return nil
}

var statusRank = map[string]int{
	models.PaymentPending: 0, models.PaymentConfirming: 1, models.PaymentPartial: 2, models.PaymentFailed: 2,
	models.PaymentExpired: 2, models.PaymentPaid: 3, models.PaymentRefunded: 4,
}

func transitionAllowed(from, to string) bool {
	if from == to {
		return false
	}
	switch from {
	case models.PaymentPaid:
		return to == models.PaymentRefunded
	case models.PaymentRefunded:
		return false
	case models.PaymentFailed, models.PaymentExpired:
		return to == models.PaymentConfirming || to == models.PaymentPartial || to == models.PaymentPaid
	}
	return statusRank[to] >= statusRank[from]
}

var errMismatch = errors.New("provider data does not match the payment")

// applyProviderStatus applies the provider's authoritative view of a payment.
func (s *BillingService) applyProviderStatus(ctx context.Context, ps *payment.PaymentStatus, source string) error {
	orderID, err := uuid.Parse(ps.OrderID)
	if err != nil {
		s.Audit.Security(ctx, "billing.ipn.unknown_order", models.SeverityWarn, map[string]any{"order_id": ps.OrderID, "source": source})
		return apperr.NotFound("payment")
	}
	newStatus, ok := payment.MapStatus(ps.RawStatus)
	if !ok {
		return apperr.Unprocessable("unknown_provider_status", ps.RawStatus)
	}
	return s.tx(ctx, func(ctx context.Context) error {
		p, err := s.Repos.Billing.PaymentForUpdate(ctx, orderID)
		if err != nil {
			if database.IsNotFound(err) {
				s.Audit.Security(ctx, "billing.ipn.unknown_order", models.SeverityWarn, map[string]any{"order_id": ps.OrderID, "source": source})
				return apperr.NotFound("payment")
			}
			return apperr.Internal(err)
		}
		cents, ok := payment.DecimalToCents(ps.PriceAmount)
		mismatch := !ok || cents != p.AmountCents || !strings.EqualFold(ps.PriceCurrency, p.Currency) ||
			(p.ProviderInvoiceID != nil && ps.InvoiceID != "" && *p.ProviderInvoiceID != ps.InvoiceID) ||
			(p.ProviderPaymentID != nil && *p.ProviderPaymentID != ps.PaymentID)
		if mismatch {
			s.Audit.Record(ctx, AuditEvent{Action: "billing.ipn.mismatch", Category: models.AuditSecurity, Severity: models.SeverityCritical,
				TargetType: "payment", TargetID: p.ID.String(), Metadata: map[string]any{"source": source}, Alert: true})
			return apperr.Unprocessable("payment_mismatch", errMismatch.Error())
		}
		if newStatus == models.PaymentPartial && payment.WithinTolerance(ps.PayAmount, ps.ActuallyPaid, s.Cfg.UnderpayToleranceBPS) {
			newStatus = models.PaymentPaid
		}
		if newStatus == models.PaymentPaid && ps.PayAmount != "" && ps.ActuallyPaid != "" &&
			!payment.WithinTolerance(ps.PayAmount, ps.ActuallyPaid, s.Cfg.UnderpayToleranceBPS) {
			newStatus = models.PaymentPartial
		}
		cols := map[string]any{"provider_status": ps.RawStatus}
		if p.ProviderPaymentID == nil && ps.PaymentID != "" {
			cols["provider_payment_id"] = ps.PaymentID
		}
		if ps.PayCurrency != "" {
			cols["pay_currency"] = ps.PayCurrency
		}
		if ps.PayAmount != "" {
			cols["pay_amount_expected"] = ps.PayAmount
		}
		if ps.ActuallyPaid != "" {
			cols["pay_amount_received"] = ps.ActuallyPaid
		}
		if !transitionAllowed(p.Status, newStatus) {
			// Out-of-order or duplicate notification: keep the status.
			return s.Repos.Billing.UpdatePayment(ctx, p.ID, cols)
		}
		cols["status"] = newStatus
		if err := s.Repos.Billing.UpdatePayment(ctx, p.ID, cols); err != nil {
			return apperr.Internal(err)
		}
		s.Audit.Record(ctx, AuditEvent{Action: "billing.payment.status_changed", Category: models.AuditBilling, TargetType: "payment",
			TargetID: p.ID.String(), Metadata: map[string]any{"from": p.Status, "to": newStatus, "source": source}})
		switch newStatus {
		case models.PaymentPaid:
			return s.markPaid(ctx, p)
		case models.PaymentRefunded:
			s.Audit.Record(ctx, AuditEvent{Action: "billing.payment.refunded", Category: models.AuditBilling, Severity: models.SeverityWarn,
				TargetType: "payment", TargetID: p.ID.String(), Alert: true})
		case models.PaymentPartial:
			s.Audit.Record(ctx, AuditEvent{Action: "billing.payment.partial", Category: models.AuditBilling, Severity: models.SeverityWarn,
				TargetType: "payment", TargetID: p.ID.String()})
		}
		return nil
	})
}

// markPaid extends the subscription period, issues the invoice and
// recomputes the publication of every profile of the establishment.
func (s *BillingService) markPaid(ctx context.Context, p *models.Payment) error {
	sub, err := s.Repos.Billing.SubscriptionForUpdate(ctx, p.SubscriptionID)
	if err != nil {
		return apperr.Internal(err)
	}
	plan, err := s.Repos.Billing.PlanByID(ctx, sub.PlanID)
	if err != nil {
		return apperr.Internal(err)
	}
	now := s.now()
	if sub.Status == models.SubCanceled {
		if other, err := s.Repos.Billing.CurrentSubscription(ctx, sub.EstablishmentID); err == nil && other.ID != sub.ID {
			s.Audit.Record(ctx, AuditEvent{Action: "billing.payment.orphan", Category: models.AuditBilling, Severity: models.SeverityCritical,
				TargetType: "payment", TargetID: p.ID.String(), Alert: true})
			return s.Repos.Billing.UpdatePayment(ctx, p.ID, map[string]any{"paid_at": now})
		}
	}
	start := now
	if (sub.Status == models.SubActive || sub.Status == models.SubTrialing) && sub.CurrentPeriodEnd != nil && sub.CurrentPeriodEnd.After(now) {
		start = *sub.CurrentPeriodEnd
	}
	end := start.AddDate(0, 0, plan.IntervalDays)
	periodStart := sub.CurrentPeriodStart
	if start.Equal(now) || periodStart == nil {
		periodStart = &start
	}
	if err := s.Repos.Billing.UpdateSubscription(ctx, sub.ID, map[string]any{
		"status": models.SubActive, "current_period_start": *periodStart, "current_period_end": end,
		"grace_until": nil, "canceled_at": nil,
	}); err != nil {
		return apperr.Internal(err)
	}
	if err := s.Repos.Billing.UpdatePayment(ctx, p.ID, map[string]any{"paid_at": now, "period_start": start, "period_end": end}); err != nil {
		return apperr.Internal(err)
	}
	if err := s.issueInvoice(ctx, p, now); err != nil {
		return err
	}
	s.Audit.Record(ctx, AuditEvent{Action: "billing.subscription.activated", Category: models.AuditBilling, TargetType: "subscription",
		TargetID: sub.ID.String(), Metadata: map[string]any{"from": sub.Status, "period_end": end.Format(time.RFC3339)}})
	return s.Publication.Recompute(ctx, sub.EstablishmentID)
}

type billingSnapshot struct {
	Number         string `json:"number"`
	LegalName      string `json:"legal_name"`
	DisplayName    string `json:"display_name"`
	CompanyNumber  string `json:"company_number"`
	ApprovalNumber string `json:"approval_number"`
	Address        string `json:"address"`
	PeriodStart    string `json:"period_start"`
	PeriodEnd      string `json:"period_end"`
	PlatformName   string `json:"platform_legal_name"`
	PlatformNumber string `json:"platform_company_number"`
	PlatformAddr   string `json:"platform_address"`
	PayCurrency    string `json:"pay_currency"`
	PayAmount      string `json:"pay_amount"`
}

// issueInvoice numbers invoices without gaps per year (RM-36) under a
// transaction-scoped advisory lock.
func (s *BillingService) issueInvoice(ctx context.Context, p *models.Payment, now time.Time) error {
	if _, err := s.Repos.Billing.InvoiceByPayment(ctx, p.ID); err == nil {
		return nil
	}
	if err := s.DB.XactLock(ctx, "invoice-number"); err != nil {
		return apperr.Internal(err)
	}
	prefix := fmt.Sprintf("F%d-", now.Year())
	last, err := s.Repos.Billing.LastInvoiceNumber(ctx, prefix)
	if err != nil {
		return apperr.Internal(err)
	}
	seq := 1
	if last != "" {
		n, _ := strconv.Atoi(strings.TrimPrefix(last, prefix))
		seq = n + 1
	}
	number := fmt.Sprintf("%s%06d", prefix, seq)
	est, err := s.Repos.Establishments.ByID(ctx, p.EstablishmentID)
	if err != nil {
		return apperr.Internal(err)
	}
	fresh, err := s.Repos.Billing.PaymentByID(ctx, p.ID)
	if err != nil {
		return apperr.Internal(err)
	}
	snap := billingSnapshot{Number: number, LegalName: est.LegalName, DisplayName: est.DisplayName,
		CompanyNumber: deref(est.CompanyNumber), ApprovalNumber: deref(est.ApprovalNumber), Address: deref(est.Address),
		PlatformName: s.Cfg.PlatformLegalName, PlatformNumber: s.Cfg.PlatformCompanyNumber, PlatformAddr: s.Cfg.PlatformAddress,
		PayCurrency: deref(fresh.PayCurrency), PayAmount: deref(fresh.PayAmountReceived)}
	if fresh.PeriodStart != nil {
		snap.PeriodStart = fresh.PeriodStart.Format("2006-01-02")
	}
	if fresh.PeriodEnd != nil {
		snap.PeriodEnd = fresh.PeriodEnd.Format("2006-01-02")
	}
	raw, _ := json.Marshal(snap)
	inv := &models.Invoice{ID: models.NewID(), Number: number, PaymentID: p.ID, EstablishmentID: p.EstablishmentID,
		AmountCents: p.AmountCents, Currency: p.Currency, IssuedAt: now, BillingSnapshot: raw}
	if err := s.Repos.Billing.CreateInvoice(ctx, inv); err != nil {
		return apperr.Internal(err)
	}
	s.Audit.Record(ctx, AuditEvent{Action: "billing.invoice.issued", Category: models.AuditBilling, TargetType: "invoice",
		TargetID: inv.ID.String(), Metadata: map[string]any{"number": number}})
	return nil
}

func renderInvoice(inv *models.Invoice, snap billingSnapshot) []byte {
	amount := payment.CentsToDecimal(inv.AmountCents) + " " + inv.Currency
	lines := []pdf.Line{
		{X: 50, Y: 790, Size: 18, Bold: true, Text: "Facture " + inv.Number},
		{X: 50, Y: 765, Size: 10, Text: "Date d’émission : " + inv.IssuedAt.Format("02/01/2006")},
		{X: 50, Y: 730, Size: 11, Bold: true, Text: "Émetteur"},
		{X: 50, Y: 715, Size: 10, Text: snap.PlatformName},
		{X: 50, Y: 702, Size: 10, Text: "N° d’entreprise : " + snap.PlatformNumber},
		{X: 50, Y: 689, Size: 10, Text: snap.PlatformAddr},
		{X: 320, Y: 730, Size: 11, Bold: true, Text: "Client"},
		{X: 320, Y: 715, Size: 10, Text: snap.LegalName},
		{X: 320, Y: 702, Size: 10, Text: "BCE : " + snap.CompanyNumber},
		{X: 320, Y: 689, Size: 10, Text: "N° d’agrément : " + snap.ApprovalNumber},
		{X: 320, Y: 676, Size: 10, Text: snap.Address},
		{X: 50, Y: 630, Size: 11, Bold: true, Text: "Désignation"},
		{X: 430, Y: 630, Size: 11, Bold: true, Text: "Montant"},
		{X: 50, Y: 610, Size: 10, Text: "Abonnement plateforme — " + snap.DisplayName},
		{X: 50, Y: 597, Size: 10, Text: "Période du " + snap.PeriodStart + " au " + snap.PeriodEnd},
		{X: 430, Y: 610, Size: 10, Text: amount},
		{X: 50, Y: 560, Size: 11, Bold: true, Text: "Total : " + amount},
		{X: 50, Y: 540, Size: 9, Text: "Réglé en cryptomonnaie via NOWPayments : " + snap.PayAmount + " " + snap.PayCurrency},
		{X: 50, Y: 80, Size: 8, Text: "Pas de remboursement au prorata en cas de résiliation (voir CGV)."},
	}
	return pdf.Render(lines)
}

// ---------------------------------------------------------------- admin reconciliation

// Reconcile re-queries the provider for a payment; with forcePaid an admin
// accepts a PARTIAL payment (RM-35). Always audited with a reason.
func (s *BillingService) Reconcile(ctx context.Context, paymentID uuid.UUID, reason string, forcePaid bool) (*models.Payment, error) {
	reason, err := reasonRequired(reason)
	if err != nil {
		return nil, err
	}
	p, err := s.Repos.Billing.PaymentByID(ctx, paymentID)
	if err != nil {
		if database.IsNotFound(err) {
			return nil, apperr.NotFound("payment")
		}
		return nil, apperr.Internal(err)
	}
	s.Audit.Record(ctx, AuditEvent{Action: "admin.payment.reconcile", Category: models.AuditAdmin, Severity: models.SeverityWarn,
		TargetType: "payment", TargetID: p.ID.String(), Metadata: map[string]any{"reason": reason, "force_paid": forcePaid}})
	if p.ProviderPaymentID != nil {
		ps, err := s.Payment.GetPayment(ctx, *p.ProviderPaymentID)
		if err != nil {
			return nil, apperr.New(apperr.KindBadGateway, "payment_provider_unavailable", err.Error())
		}
		if err := s.applyProviderStatus(ctx, ps, "admin"); err != nil {
			return nil, err
		}
	}
	if forcePaid {
		err := s.tx(ctx, func(ctx context.Context) error {
			p, err := s.Repos.Billing.PaymentForUpdate(ctx, paymentID)
			if err != nil {
				return apperr.Internal(err)
			}
			if p.Status != models.PaymentPartial {
				return apperr.Conflict("invalid_state", "only PARTIAL payments can be forced to PAID")
			}
			if err := s.Repos.Billing.UpdatePayment(ctx, p.ID, map[string]any{"status": models.PaymentPaid}); err != nil {
				return apperr.Internal(err)
			}
			return s.markPaid(ctx, p)
		})
		if err != nil {
			return nil, err
		}
	}
	return s.Repos.Billing.PaymentByID(ctx, paymentID)
}

// ---------------------------------------------------------------- scheduled tasks

// CreateRenewalInvoices prepares the next period's invoice from J-5 (RM-32)
// and the regularisation invoice of PAST_DUE subscriptions.
func (s *BillingService) CreateRenewalInvoices(ctx context.Context) (int, error) {
	now := s.now()
	subs, err := s.Repos.Billing.SubscriptionsEndingBefore(ctx, []string{models.SubActive, models.SubTrialing, models.SubPastDue}, now.Add(renewalLead))
	if err != nil {
		return 0, err
	}
	n := 0
	for _, sub := range subs {
		if sub.CancelAtPeriodEnd {
			continue
		}
		if _, err := s.Repos.Billing.OpenPayment(ctx, sub.ID, now); err == nil {
			continue
		}
		est, err := s.Repos.Establishments.ByID(ctx, sub.EstablishmentID)
		if err != nil {
			continue
		}
		pay, reused, err := s.ensureRenewalPayment(ctx, sub.EstablishmentID)
		if err != nil || reused {
			continue
		}
		if _, err := s.attachInvoice(ctx, pay, est); err != nil {
			s.Logger.WarnContext(ctx, "renewal invoice failed", "subscription", sub.ID, "error", err)
			continue
		}
		n++
	}
	return n, nil
}

// ExpirePeriods applies the state machine of §3.3 for elapsed periods.
func (s *BillingService) ExpirePeriods(ctx context.Context) (int, error) {
	now := s.now()
	touched := map[uuid.UUID]bool{}
	changes := 0
	transition := func(sub models.Subscription, cols map[string]any, action string) {
		err := s.tx(ctx, func(ctx context.Context) error {
			if err := s.Repos.Billing.UpdateSubscription(ctx, sub.ID, cols); err != nil {
				return err
			}
			s.Audit.Record(ctx, AuditEvent{Action: action, Category: models.AuditBilling, TargetType: "subscription", TargetID: sub.ID.String(), ActorRole: "SYSTEM"})
			return nil
		})
		if err != nil {
			s.Logger.ErrorContext(ctx, "subscription transition failed", "subscription", sub.ID, "error", err)
			return
		}
		touched[sub.EstablishmentID] = true
		changes++
	}
	due, err := s.Repos.Billing.SubscriptionsEndingBefore(ctx, []string{models.SubActive, models.SubTrialing}, now)
	if err != nil {
		return 0, err
	}
	for _, sub := range due {
		if sub.CancelAtPeriodEnd || sub.Status == models.SubTrialing && sub.CurrentPeriodStart == nil {
			transition(sub, map[string]any{"status": models.SubCanceled, "canceled_at": now}, "billing.subscription.canceled")
			continue
		}
		plan, err := s.Repos.Billing.PlanByID(ctx, sub.PlanID)
		if err != nil {
			continue
		}
		grace := sub.CurrentPeriodEnd.AddDate(0, 0, plan.GraceDays)
		transition(sub, map[string]any{"status": models.SubPastDue, "grace_until": grace}, "billing.subscription.past_due")
	}
	pastDue, err := s.Repos.Billing.SubscriptionsByStatus(ctx, models.SubPastDue)
	if err != nil {
		return changes, err
	}
	for _, sub := range pastDue {
		touched[sub.EstablishmentID] = true // grace may have ended: unpublish
		if sub.CurrentPeriodEnd != nil && !now.Before(sub.CurrentPeriodEnd.Add(cancelAfterPastDue)) {
			transition(sub, map[string]any{"status": models.SubCanceled, "canceled_at": now}, "billing.subscription.canceled")
		}
	}
	incomplete, err := s.Repos.Billing.SubscriptionsByStatus(ctx, models.SubIncomplete)
	if err != nil {
		return changes, err
	}
	for _, sub := range incomplete {
		if _, err := s.Repos.Billing.OpenPayment(ctx, sub.ID, now); err == nil {
			continue
		}
		last, err := s.Repos.Billing.LastPayment(ctx, sub.ID)
		ref := sub.CreatedAt
		if err == nil {
			ref = last.UpdatedAt
		}
		if now.Sub(ref) >= incompleteTTL {
			transition(sub, map[string]any{"status": models.SubCanceled, "canceled_at": now}, "billing.subscription.canceled")
		}
	}
	ids := make([]uuid.UUID, 0, len(touched))
	for id := range touched {
		ids = append(ids, id)
	}
	if len(ids) > 0 {
		if err := s.Publication.Recompute(ctx, ids...); err != nil {
			return changes, err
		}
	}
	return changes, nil
}

// ReconcileStalePayments catches up missed IPNs (§14.3 point 8) and expires
// invoices that never received a payment.
func (s *BillingService) ReconcileStalePayments(ctx context.Context) (int, error) {
	now := s.now()
	stale, err := s.Repos.Billing.StalePayments(ctx, now.Add(-staleAfter), 200)
	if err != nil {
		return 0, err
	}
	n := 0
	for _, p := range stale {
		if p.ProviderPaymentID != nil {
			ps, err := s.Payment.GetPayment(ctx, *p.ProviderPaymentID)
			if err != nil {
				continue
			}
			if err := s.applyProviderStatus(ctx, ps, "reconcile"); err == nil {
				n++
			}
			continue
		}
		if p.ExpiresAt != nil && now.After(*p.ExpiresAt) {
			if err := s.Repos.Billing.UpdatePayment(ctx, p.ID, map[string]any{"status": models.PaymentExpired}); err == nil {
				n++
			}
		}
	}
	return n, nil
}
