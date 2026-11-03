package dto

import (
	"time"

	"github.com/google/uuid"

	"opensex/backend/internal/models"
	"opensex/backend/internal/repositories"
	"opensex/backend/internal/services"
)

type Paginated[T any] struct {
	Items      []T   `json:"items"`
	Page       int   `json:"page"`
	PageSize   int   `json:"page_size"`
	Total      int64 `json:"total"`
	TotalPages int64 `json:"total_pages"`
}

func Paginate[T any](items []T, p repositories.Page, total int64) Paginated[T] {
	if items == nil {
		items = []T{}
	}
	pages := (total + int64(p.PageSize) - 1) / int64(p.PageSize)
	return Paginated[T]{Items: items, Page: p.Page, PageSize: p.PageSize, Total: total, TotalPages: pages}
}

func MapSlice[A, B any](in []A, f func(A) B) []B {
	out := make([]B, len(in))
	for i, v := range in {
		out[i] = f(v)
	}
	return out
}

// ---------------------------------------------------------------- auth

type User struct {
	ID                     uuid.UUID  `json:"id"`
	Username               string     `json:"username"`
	Role                   string     `json:"role"`
	Status                 string     `json:"status"`
	MustChangePassword     bool       `json:"must_change_password"`
	RecoveryCodesConfirmed bool       `json:"recovery_codes_confirmed"`
	CreatedAt              time.Time  `json:"created_at"`
	LastLoginAt            *time.Time `json:"last_login_at"`
}

func NewUser(u *models.User) *User {
	if u == nil {
		return nil
	}
	return &User{ID: u.ID, Username: u.Username, Role: u.Role, Status: u.Status, MustChangePassword: u.MustChangePassword,
		RecoveryCodesConfirmed: u.RecoveryCodesConfirmedAt != nil, CreatedAt: u.CreatedAt, LastLoginAt: u.LastLoginAt}
}

type AdminUser struct {
	User
	StatusReason     *string    `json:"status_reason"`
	FailedLoginCount int        `json:"failed_login_count"`
	LockedUntil      *time.Time `json:"locked_until"`
	DeletedAt        *time.Time `json:"deleted_at"`
}

func NewAdminUser(u models.User) AdminUser {
	out := AdminUser{User: *NewUser(&u), StatusReason: u.StatusReason, FailedLoginCount: u.FailedLoginCount, LockedUntil: u.LockedUntil}
	if u.DeletedAt.Valid {
		out.DeletedAt = &u.DeletedAt.Time
	}
	return out
}

type AuthResponse struct {
	AccessToken   string    `json:"access_token"`
	TokenType     string    `json:"token_type"`
	ExpiresAt     time.Time `json:"expires_at"`
	User          *User     `json:"user"`
	RecoveryCodes []string  `json:"recovery_codes,omitempty"`
}

func NewAuthResponse(r *services.AuthResult) AuthResponse {
	return AuthResponse{AccessToken: r.Session.AccessToken, TokenType: "Bearer", ExpiresAt: r.Session.ExpiresAt,
		User: NewUser(r.User), RecoveryCodes: r.RecoveryCodes}
}

type MeResponse struct {
	User                   *User             `json:"user"`
	RecoveryCodesRemaining int64             `json:"recovery_codes_remaining"`
	Establishment          *EstablishmentRef `json:"establishment,omitempty"`
	Profile                *ProfileRef       `json:"profile,omitempty"`
}

type EstablishmentRef struct {
	ID          uuid.UUID `json:"id"`
	Slug        string    `json:"slug"`
	DisplayName string    `json:"display_name"`
}

type ProfileRef struct {
	ID          uuid.UUID `json:"id"`
	Slug        string    `json:"slug"`
	DisplayName string    `json:"display_name"`
	Status      string    `json:"status"`
	IsPublished bool      `json:"is_published"`
}

func NewMe(m *services.MeResult) MeResponse {
	out := MeResponse{User: NewUser(m.User), RecoveryCodesRemaining: m.RecoveryRemaining}
	if m.Establishment != nil {
		out.Establishment = &EstablishmentRef{ID: m.Establishment.ID, Slug: m.Establishment.Slug, DisplayName: m.Establishment.DisplayName}
	}
	if m.Profile != nil {
		out.Profile = &ProfileRef{ID: m.Profile.ID, Slug: m.Profile.Slug, DisplayName: m.Profile.DisplayName, Status: m.Profile.Status, IsPublished: m.Profile.IsPublished}
	}
	return out
}

// ---------------------------------------------------------------- reference data

type Region struct {
	ID       uuid.UUID  `json:"id"`
	Slug     string     `json:"slug"`
	Name     string     `json:"name"`
	Kind     string     `json:"kind"`
	ParentID *uuid.UUID `json:"parent_id"`
	IsActive bool       `json:"is_active"`
}

func NewRegion(r models.Region) Region {
	return Region{ID: r.ID, Slug: r.Slug, Name: r.Name, Kind: r.Kind, ParentID: r.ParentID, IsActive: r.IsActive}
}

type Category struct {
	ID          uuid.UUID  `json:"id"`
	Slug        string     `json:"slug"`
	Name        string     `json:"name"`
	Description string     `json:"description"`
	ParentID    *uuid.UUID `json:"parent_id"`
	Position    int        `json:"position"`
	IsActive    bool       `json:"is_active"`
}

func NewCategory(c models.Category) Category {
	return Category{ID: c.ID, Slug: c.Slug, Name: c.Name, Description: c.Description, ParentID: c.ParentID, Position: c.Position, IsActive: c.IsActive}
}

type SelectableEstablishment struct {
	Slug        string  `json:"slug"`
	DisplayName string  `json:"display_name"`
	City        *string `json:"city"`
	Region      *string `json:"region"`
}

// ---------------------------------------------------------------- establishments

type Contact struct {
	ID                  uuid.UUID `json:"id"`
	Type                string    `json:"type"`
	Value               string    `json:"value"`
	Visibility          string    `json:"visibility"`
	IsVerificationPhone bool      `json:"is_verification_phone,omitempty"`
	Position            int       `json:"position"`
}

func NewEstablishmentContact(c models.EstablishmentContact) Contact {
	return Contact{ID: c.ID, Type: c.Type, Value: c.Value, Visibility: c.Visibility, IsVerificationPhone: c.IsVerificationPhone, Position: c.Position}
}

func NewProfileContact(c models.ProfileContact) Contact {
	return Contact{ID: c.ID, Type: c.Type, Value: c.Value, Visibility: c.Visibility, Position: c.Position}
}

type Establishment struct {
	ID            uuid.UUID   `json:"id"`
	Slug          string      `json:"slug"`
	DisplayName   string      `json:"display_name"`
	LegalName     string      `json:"legal_name"`
	Description   string      `json:"description"`
	OpeningHours  models.JSON `json:"opening_hours"`
	Region        *Region     `json:"region"`
	City          *Region     `json:"city"`
	Address       *string     `json:"address"`
	AddressPublic bool        `json:"address_public"`
	IsHidden      bool        `json:"is_hidden"`
	HiddenReason  *string     `json:"hidden_reason"`
	Contacts      []Contact   `json:"contacts"`
	CreatedAt     time.Time   `json:"created_at"`
	UpdatedAt     time.Time   `json:"updated_at"`
}

func NewEstablishment(d *services.EstablishmentDetail) Establishment {
	e := d.Establishment
	out := Establishment{ID: e.ID, Slug: e.Slug, DisplayName: e.DisplayName, LegalName: e.LegalName, Description: e.Description,
		OpeningHours: e.OpeningHours, Address: e.Address, AddressPublic: e.AddressPublic, IsHidden: e.IsHidden,
		HiddenReason: e.HiddenReason, Contacts: MapSlice(d.Contacts, NewEstablishmentContact), CreatedAt: e.CreatedAt, UpdatedAt: e.UpdatedAt}
	if d.Region != nil {
		r := NewRegion(*d.Region)
		out.Region = &r
	}
	if d.City != nil {
		r := NewRegion(*d.City)
		out.City = &r
	}
	return out
}

// ---------------------------------------------------------------- profiles (owner/manager views)

type Photo struct {
	ID        uuid.UUID `json:"id"`
	URL       string    `json:"url"`
	IsPrimary bool      `json:"is_primary"`
	Position  int       `json:"position"`
	Width     int       `json:"width"`
	Height    int       `json:"height"`
	CreatedAt time.Time `json:"created_at"`
}

func NewPhoto(p services.PhotoView) Photo {
	return Photo{ID: p.ID, URL: p.URL, IsPrimary: p.IsPrimary, Position: p.Position, Width: p.Width, Height: p.Height, CreatedAt: p.CreatedAt}
}

type Rate struct {
	ID              uuid.UUID `json:"id"`
	Label           string    `json:"label"`
	DurationMinutes *int      `json:"duration_minutes"`
	AmountCents     int64     `json:"amount_cents"`
	Currency        string    `json:"currency"`
	Position        int       `json:"position"`
}

func NewRate(r models.ProfileRate) Rate {
	return Rate{ID: r.ID, Label: r.Label, DurationMinutes: r.DurationMinutes, AmountCents: r.AmountCents, Currency: r.Currency, Position: r.Position}
}

type Attestation struct {
	Attested            bool       `json:"attested"`
	AttestedAt          *time.Time `json:"attested_at"`
	BirthDate           *string    `json:"birth_date,omitempty"`
	Age                 *int       `json:"age"`
	EmploymentReference *string    `json:"employment_reference,omitempty"`
}

type LinkedAccount struct {
	ID       uuid.UUID `json:"id"`
	Username string    `json:"username"`
	Status   string    `json:"status"`
}

type LastEdit struct {
	By   *string   `json:"by"`
	Role *string   `json:"role"`
	At   time.Time `json:"at"`
}

type Publication struct {
	Published   bool       `json:"published"`
	PublishedAt *time.Time `json:"published_at"`
	Reasons     []string   `json:"reasons"`
}

type Profile struct {
	ID              uuid.UUID        `json:"id"`
	Slug            string           `json:"slug"`
	Status          string           `json:"status"`
	DisplayName     string           `json:"display_name"`
	Headline        string           `json:"headline"`
	Description     string           `json:"description"`
	HeightCm        *int16           `json:"height_cm"`
	WeightKg        *int16           `json:"weight_kg"`
	HairColor       *string          `json:"hair_color"`
	EyeColor        *string          `json:"eye_color"`
	BodyType        *string          `json:"body_type"`
	Languages       []string         `json:"languages"`
	ExtraAttributes models.JSON      `json:"extra_attributes"`
	PriceFromCents  *int64           `json:"price_from_cents"`
	IsHidden        bool             `json:"is_hidden"`
	HiddenReason    *string          `json:"hidden_reason"`
	Attestation     Attestation      `json:"attestation"`
	Photos          []Photo          `json:"photos"`
	Contacts        []Contact        `json:"contacts"`
	Rates           []Rate           `json:"rates"`
	Categories      []Category       `json:"categories"`
	Establishment   EstablishmentRef `json:"establishment"`
	LinkedAccount   *LinkedAccount   `json:"linked_account,omitempty"`
	LastEdit        LastEdit         `json:"last_edit"`
	Publication     Publication      `json:"publication"`
	CreatedAt       time.Time        `json:"created_at"`
}

func langs(a models.StringArray) []string {
	if a == nil {
		return []string{}
	}
	return []string(a)
}

// NewProfile renders a profile for its owner (manager=false) or the manager.
func NewProfile(d *services.ProfileDetail, manager bool, now time.Time) Profile {
	p := d.Profile
	out := Profile{ID: p.ID, Slug: p.Slug, Status: p.Status, DisplayName: p.DisplayName, Headline: p.Headline, Description: p.Description,
		HeightCm: p.HeightCm, WeightKg: p.WeightKg, HairColor: p.HairColor, EyeColor: p.EyeColor, BodyType: p.BodyType,
		Languages: langs(p.Languages), ExtraAttributes: p.ExtraAttributes, PriceFromCents: p.PriceFromCents, IsHidden: p.IsHidden,
		HiddenReason: p.HiddenReason, Photos: MapSlice(d.Photos, NewPhoto), Contacts: MapSlice(d.Contacts, NewProfileContact),
		Rates: MapSlice(d.Rates, NewRate), Categories: MapSlice(d.Categories, NewCategory),
		Establishment: EstablishmentRef{ID: d.Establishment.ID, Slug: d.Establishment.Slug, DisplayName: d.Establishment.DisplayName},
		LastEdit:      LastEdit{At: p.UpdatedAt},
		Publication:   Publication{Published: p.IsPublished, PublishedAt: p.PublishedAt, Reasons: d.Reasons},
		CreatedAt:     p.CreatedAt}
	out.Attestation = Attestation{Attested: p.AttestedAt != nil, AttestedAt: p.AttestedAt}
	if p.BirthDateAttested != nil {
		age := services.AgeAt(*p.BirthDateAttested, now)
		out.Attestation.Age = &age
		if manager {
			bd := p.BirthDateAttested.Format("2006-01-02")
			out.Attestation.BirthDate = &bd
		}
	}
	if manager {
		if d.EmploymentReference != "" {
			out.Attestation.EmploymentReference = &d.EmploymentReference
		}
		if d.LinkedUser != nil {
			out.LinkedAccount = &LinkedAccount{ID: d.LinkedUser.ID, Username: d.LinkedUser.Username, Status: d.LinkedUser.Status}
		}
	}
	if d.LastEditor != nil {
		out.LastEdit.By, out.LastEdit.Role = &d.LastEditor.Username, &d.LastEditor.Role
	}
	return out
}

type ProfileSummary struct {
	ID            uuid.UUID      `json:"id"`
	Slug          string         `json:"slug"`
	DisplayName   string         `json:"display_name"`
	Status        string         `json:"status"`
	Attested      bool           `json:"attested"`
	IsHidden      bool           `json:"is_hidden"`
	PhotoURL      *string        `json:"photo_url,omitempty"`
	LinkedAccount *LinkedAccount `json:"linked_account"`
	LastEdit      LastEdit       `json:"last_edit"`
	Publication   Publication    `json:"publication"`
	CreatedAt     time.Time      `json:"created_at"`
}

func NewProfileSummary(it services.ProfileListItem) ProfileSummary {
	p := it.Profile
	out := ProfileSummary{ID: p.ID, Slug: p.Slug, DisplayName: p.DisplayName, Status: p.Status, Attested: p.AttestedAt != nil,
		IsHidden: p.IsHidden, LastEdit: LastEdit{At: p.UpdatedAt}, CreatedAt: p.CreatedAt,
		Publication: Publication{Published: p.IsPublished, PublishedAt: p.PublishedAt, Reasons: it.Reasons}}
	if out.Publication.Reasons == nil {
		out.Publication.Reasons = []string{}
	}
	if it.PrimaryURL != "" {
		out.PhotoURL = &it.PrimaryURL
	}
	if it.LinkedUser != nil {
		out.LinkedAccount = &LinkedAccount{ID: it.LinkedUser.ID, Username: it.LinkedUser.Username, Status: it.LinkedUser.Status}
	}
	if it.LastEditor != nil {
		out.LastEdit.By, out.LastEdit.Role = &it.LastEditor.Username, &it.LastEditor.Role
	}
	return out
}

type AdminProfile struct {
	ID              uuid.UUID  `json:"id"`
	EstablishmentID uuid.UUID  `json:"establishment_id"`
	UserID          *uuid.UUID `json:"user_id"`
	Slug            string     `json:"slug"`
	DisplayName     string     `json:"display_name"`
	Status          string     `json:"status"`
	Attested        bool       `json:"attested"`
	IsHidden        bool       `json:"is_hidden"`
	HiddenReason    *string    `json:"hidden_reason"`
	IsPublished     bool       `json:"is_published"`
	PublishedAt     *time.Time `json:"published_at"`
	CreatedAt       time.Time  `json:"created_at"`
	UpdatedAt       time.Time  `json:"updated_at"`
}

func NewAdminProfile(p models.Profile) AdminProfile {
	return AdminProfile{ID: p.ID, EstablishmentID: p.EstablishmentID, UserID: p.UserID, Slug: p.Slug, DisplayName: p.DisplayName,
		Status: p.Status, Attested: p.AttestedAt != nil, IsHidden: p.IsHidden, HiddenReason: p.HiddenReason, IsPublished: p.IsPublished,
		PublishedAt: p.PublishedAt, CreatedAt: p.CreatedAt, UpdatedAt: p.UpdatedAt}
}

// ---------------------------------------------------------------- billing

type Plan struct {
	Code               string `json:"code"`
	Name               string `json:"name"`
	AmountCents        int64  `json:"amount_cents"`
	Currency           string `json:"currency"`
	IntervalDays       int    `json:"interval_days"`
	TrialDays          int    `json:"trial_days"`
	GraceDays          int    `json:"grace_days"`
	MaxActiveProfiles  *int   `json:"max_active_profiles"`
	MaxPendingProfiles int    `json:"max_pending_profiles"`
}

func NewPlan(p models.SubscriptionPlan) Plan {
	return Plan{Code: p.Code, Name: p.Name, AmountCents: p.AmountCents, Currency: p.Currency, IntervalDays: p.IntervalDays,
		TrialDays: p.TrialDays, GraceDays: p.GraceDays, MaxActiveProfiles: p.MaxActiveProfiles, MaxPendingProfiles: p.MaxPendingProfiles}
}

type Subscription struct {
	ID                 uuid.UUID  `json:"id"`
	Status             string     `json:"status"`
	CurrentPeriodStart *time.Time `json:"current_period_start"`
	CurrentPeriodEnd   *time.Time `json:"current_period_end"`
	GraceUntil         *time.Time `json:"grace_until"`
	CancelAtPeriodEnd  bool       `json:"cancel_at_period_end"`
	CanceledAt         *time.Time `json:"canceled_at"`
	CreatedAt          time.Time  `json:"created_at"`
}

func NewSubscription(s *models.Subscription) *Subscription {
	if s == nil {
		return nil
	}
	return &Subscription{ID: s.ID, Status: s.Status, CurrentPeriodStart: s.CurrentPeriodStart, CurrentPeriodEnd: s.CurrentPeriodEnd,
		GraceUntil: s.GraceUntil, CancelAtPeriodEnd: s.CancelAtPeriodEnd, CanceledAt: s.CanceledAt, CreatedAt: s.CreatedAt}
}

type Payment struct {
	ID                uuid.UUID  `json:"id"`
	Status            string     `json:"status"`
	AmountCents       int64      `json:"amount_cents"`
	Currency          string     `json:"currency"`
	PeriodStart       *time.Time `json:"period_start"`
	PeriodEnd         *time.Time `json:"period_end"`
	InvoiceURL        *string    `json:"invoice_url"`
	PayCurrency       *string    `json:"pay_currency"`
	PayAmountExpected *string    `json:"pay_amount_expected"`
	PayAmountReceived *string    `json:"pay_amount_received"`
	PaidAt            *time.Time `json:"paid_at"`
	ExpiresAt         *time.Time `json:"expires_at"`
	CreatedAt         time.Time  `json:"created_at"`
}

func NewPayment(p models.Payment) Payment {
	return Payment{ID: p.ID, Status: p.Status, AmountCents: p.AmountCents, Currency: p.Currency, PeriodStart: p.PeriodStart,
		PeriodEnd: p.PeriodEnd, InvoiceURL: p.InvoiceURL, PayCurrency: p.PayCurrency, PayAmountExpected: p.PayAmountExpected,
		PayAmountReceived: p.PayAmountReceived, PaidAt: p.PaidAt, ExpiresAt: p.ExpiresAt, CreatedAt: p.CreatedAt}
}

type AdminPayment struct {
	Payment
	SubscriptionID    uuid.UUID `json:"subscription_id"`
	EstablishmentID   uuid.UUID `json:"establishment_id"`
	Provider          string    `json:"provider"`
	ProviderInvoiceID *string   `json:"provider_invoice_id"`
	ProviderPaymentID *string   `json:"provider_payment_id"`
	ProviderStatus    *string   `json:"provider_status"`
}

func NewAdminPayment(p models.Payment) AdminPayment {
	return AdminPayment{Payment: NewPayment(p), SubscriptionID: p.SubscriptionID, EstablishmentID: p.EstablishmentID, Provider: p.Provider,
		ProviderInvoiceID: p.ProviderInvoiceID, ProviderPaymentID: p.ProviderPaymentID, ProviderStatus: p.ProviderStatus}
}

type SubscriptionResponse struct {
	Subscription *Subscription     `json:"subscription"`
	Plan         *Plan             `json:"plan"`
	Entitled     bool              `json:"entitled"`
	OpenPayment  *Payment          `json:"open_payment"`
	Reminder     services.Reminder `json:"reminder"`
}

func NewSubscriptionResponse(v *services.SubscriptionView) SubscriptionResponse {
	out := SubscriptionResponse{Subscription: NewSubscription(v.Subscription), Entitled: v.Entitled, Reminder: v.Reminder}
	if v.Plan != nil {
		p := NewPlan(*v.Plan)
		out.Plan = &p
	}
	if v.OpenPayment != nil {
		p := NewPayment(*v.OpenPayment)
		out.OpenPayment = &p
	}
	return out
}

type Invoice struct {
	ID          uuid.UUID `json:"id"`
	Number      string    `json:"number"`
	PaymentID   uuid.UUID `json:"payment_id"`
	AmountCents int64     `json:"amount_cents"`
	Currency    string    `json:"currency"`
	IssuedAt    time.Time `json:"issued_at"`
}

func NewInvoice(i models.Invoice) Invoice {
	return Invoice{ID: i.ID, Number: i.Number, PaymentID: i.PaymentID, AmountCents: i.AmountCents, Currency: i.Currency, IssuedAt: i.IssuedAt}
}

// ---------------------------------------------------------------- audit

type AuditLog struct {
	ID          int64       `json:"id"`
	OccurredAt  time.Time   `json:"occurred_at"`
	ActorUserID *uuid.UUID  `json:"actor_user_id"`
	ActorRole   *string     `json:"actor_role"`
	Action      string      `json:"action"`
	Category    string      `json:"category"`
	Severity    string      `json:"severity"`
	TargetType  *string     `json:"target_type"`
	TargetID    *string     `json:"target_id"`
	IPHash      *string     `json:"ip_hash"`
	UserAgent   *string     `json:"user_agent"`
	RequestID   *string     `json:"request_id"`
	Metadata    models.JSON `json:"metadata"`
}

func NewAuditLog(l models.AuditLog) AuditLog {
	return AuditLog{ID: l.ID, OccurredAt: l.OccurredAt, ActorUserID: l.ActorUserID, ActorRole: l.ActorRole, Action: l.Action,
		Category: l.Category, Severity: l.Severity, TargetType: l.TargetType, TargetID: l.TargetID, IPHash: l.IPHash,
		UserAgent: l.UserAgent, RequestID: l.RequestID, Metadata: l.Metadata}
}
