// Package models contains the GORM structs mapped to the tables of §8.
// The schema itself is owned by the SQL migrations, never by AutoMigrate.
package models

import (
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

// NewID returns a UUID v7 (time-ordered, non enumerable).
func NewID() uuid.UUID { return uuid.Must(uuid.NewV7()) }

// Roles.
const (
	RoleAdmin        = "ADMIN"
	RoleManager      = "MANAGER"
	RoleProfessional = "PROFESSIONAL"
	RoleClient       = "CLIENT"
)

var RoleIDs = map[string]int16{RoleAdmin: 1, RoleManager: 2, RoleProfessional: 3, RoleClient: 4}

// Account statuses.
const (
	UserActive    = "ACTIVE"
	UserSuspended = "SUSPENDED"
	UserDeleted   = "DELETED"
)

type User struct {
	ID                       uuid.UUID `gorm:"type:uuid;primaryKey"`
	Username                 string
	PasswordHash             string
	Status                   string
	StatusReason             *string
	MustChangePassword       bool
	RecoveryCodesConfirmedAt *time.Time
	FailedLoginCount         int
	LockedUntil              *time.Time
	LastLoginAt              *time.Time
	TermsVersionAccepted     string
	TermsAcceptedAt          time.Time
	CreatedAt                time.Time
	UpdatedAt                time.Time
	DeletedAt                gorm.DeletedAt
	// Role is the single functional role (RM-01), loaded from user_roles.
	Role string `gorm:"->;-:migration"`
}

type UserRole struct {
	UserID    uuid.UUID `gorm:"type:uuid;primaryKey"`
	RoleID    int16     `gorm:"primaryKey"`
	GrantedAt time.Time
}

// Refresh token revocation reasons.
const (
	RevokeLogout             = "LOGOUT"
	RevokeRotated            = "ROTATED"
	RevokeReuseDetected      = "REUSE_DETECTED"
	RevokePasswordRecovered  = "PASSWORD_RECOVERED"
	RevokeAdminPasswordReset = "ADMIN_PASSWORD_RESET"
	RevokePasswordChanged    = "PASSWORD_CHANGED"
	RevokeSuspended          = "SUSPENDED"
	RevokeAdmin              = "ADMIN"
	RevokeDeleted            = "DELETED"
)

type RefreshToken struct {
	ID                uuid.UUID `gorm:"type:uuid;primaryKey"`
	UserID            uuid.UUID `gorm:"type:uuid"`
	FamilyID          uuid.UUID `gorm:"type:uuid"`
	TokenHash         []byte
	ParentID          *uuid.UUID `gorm:"type:uuid"`
	ExpiresAt         time.Time
	AbsoluteExpiresAt time.Time
	RevokedAt         *time.Time
	RevokedReason     *string
	UserAgent         string
	IPHash            string
	CreatedAt         time.Time
	LastUsedAt        *time.Time
}

type RecoveryCode struct {
	ID        uuid.UUID `gorm:"type:uuid;primaryKey"`
	UserID    uuid.UUID `gorm:"type:uuid"`
	CodeHash  []byte
	UsedAt    *time.Time
	CreatedAt time.Time
}

const (
	RegionKindRegion = "REGION"
	RegionKindCity   = "CITY"
)

type Region struct {
	ID          uuid.UUID `gorm:"type:uuid;primaryKey;default:gen_random_uuid()"`
	CountryCode string    `gorm:"default:BE"`
	Name        string
	Slug        string
	ParentID    *uuid.UUID `gorm:"type:uuid"`
	Kind        string
	IsActive    bool
}

type Category struct {
	ID          uuid.UUID  `gorm:"type:uuid;primaryKey;default:gen_random_uuid()"`
	ParentID    *uuid.UUID `gorm:"type:uuid"`
	Slug        string
	Name        string
	Description string
	IsActive    bool
	Position    int
}

type Establishment struct {
	ID               uuid.UUID `gorm:"type:uuid;primaryKey"`
	ManagerUserID    uuid.UUID `gorm:"type:uuid"`
	Slug             string
	DisplayName      string
	LegalName        string
	CompanyNumber    *string
	ApprovalNumber   *string
	Description      string
	OpeningHours     JSON
	RegionID         *uuid.UUID `gorm:"type:uuid"`
	CityID           *uuid.UUID `gorm:"type:uuid"`
	Address          *string
	AddressPublic    bool
	VerificationNote *string
	SubmittedAt      *time.Time
	VerifiedAt       *time.Time
	VerifiedBy       *uuid.UUID `gorm:"type:uuid"`
	IsHidden         bool
	HiddenReason     *string
	CreatedAt        time.Time
	UpdatedAt        time.Time
	DeletedAt        gorm.DeletedAt
}

// Contact types and visibilities (RM-40).
const (
	VisibilityPublic  = "PUBLIC"
	VisibilityClients = "CLIENTS"
	VisibilityHidden  = "HIDDEN"
)

var ContactTypes = []string{"PHONE", "WHATSAPP", "TELEGRAM", "INSTAGRAM", "FACEBOOK", "WEBSITE", "OTHER"}

type EstablishmentContact struct {
	ID                  uuid.UUID `gorm:"type:uuid;primaryKey"`
	EstablishmentID     uuid.UUID `gorm:"type:uuid"`
	Type                string
	Value               string
	Visibility          string
	IsVerificationPhone bool
	Position            int
}

// Profile statuses.
const (
	ProfilePendingApproval = "PENDING_APPROVAL"
	ProfileDraft           = "DRAFT"
	ProfileActive          = "ACTIVE"
	ProfileArchived        = "ARCHIVED"
)

type Profile struct {
	ID                  uuid.UUID  `gorm:"type:uuid;primaryKey"`
	EstablishmentID     uuid.UUID  `gorm:"type:uuid"`
	UserID              *uuid.UUID `gorm:"type:uuid"`
	Slug                string
	DisplayName         string
	Headline            string
	Description         string
	HeightCm            *int16
	WeightKg            *int16
	HairColor           *string
	EyeColor            *string
	BodyType            *string
	Languages           StringArray `gorm:"type:text[]"`
	ExtraAttributes     JSON
	BirthDateAttested   *time.Time `gorm:"type:date"`
	EmploymentReference *string
	AttestedAt          *time.Time
	AttestedBy          *uuid.UUID `gorm:"type:uuid"`
	Status              string
	IsHidden            bool
	HiddenReason        *string
	PriceFromCents      *int64
	IsPublished         bool
	PublishedAt         *time.Time
	LastEditedBy        *uuid.UUID `gorm:"type:uuid"`
	CreatedAt           time.Time
	UpdatedAt           time.Time
	DeletedAt           gorm.DeletedAt
}

func (Profile) TableName() string { return "professional_profiles" }

type ProfilePhoto struct {
	ID         uuid.UUID `gorm:"type:uuid;primaryKey"`
	ProfileID  uuid.UUID `gorm:"type:uuid"`
	StorageKey string
	IsPrimary  bool
	Position   int
	Width      int
	Height     int
	CreatedAt  time.Time
}

type ProfileContact struct {
	ID         uuid.UUID `gorm:"type:uuid;primaryKey"`
	ProfileID  uuid.UUID `gorm:"type:uuid"`
	Type       string
	Value      string
	Visibility string
	Position   int
}

type ProfileRate struct {
	ID              uuid.UUID `gorm:"type:uuid;primaryKey"`
	ProfileID       uuid.UUID `gorm:"type:uuid"`
	Label           string
	DurationMinutes *int
	AmountCents     int64
	Currency        string
	Position        int
}

type ProfessionalCategory struct {
	ProfileID  uuid.UUID `gorm:"type:uuid;primaryKey"`
	CategoryID uuid.UUID `gorm:"type:uuid;primaryKey"`
}

type Favorite struct {
	ClientUserID uuid.UUID `gorm:"type:uuid;primaryKey"`
	ProfileID    uuid.UUID `gorm:"type:uuid;primaryKey"`
	CreatedAt    time.Time
}

type SubscriptionPlan struct {
	ID                 uuid.UUID `gorm:"type:uuid;primaryKey;default:gen_random_uuid()"`
	Code               string
	Name               string
	AmountCents        int64
	Currency           string
	IntervalDays       int
	TrialDays          int
	GraceDays          int
	MaxPendingProfiles int
	MaxActiveProfiles  *int
	IsActive           bool
	CreatedAt          time.Time
	UpdatedAt          time.Time
}

// Subscription statuses.
const (
	SubTrialing   = "TRIALING"
	SubActive     = "ACTIVE"
	SubPastDue    = "PAST_DUE"
	SubCanceled   = "CANCELED"
	SubIncomplete = "INCOMPLETE"
)

type Subscription struct {
	ID                 uuid.UUID `gorm:"type:uuid;primaryKey"`
	EstablishmentID    uuid.UUID `gorm:"type:uuid"`
	PlanID             uuid.UUID `gorm:"type:uuid"`
	Status             string
	CurrentPeriodStart *time.Time
	CurrentPeriodEnd   *time.Time
	GraceUntil         *time.Time
	CancelAtPeriodEnd  bool
	CanceledAt         *time.Time
	Provider           string
	CreatedAt          time.Time
	UpdatedAt          time.Time
}

type Payment struct {
	ID                uuid.UUID `gorm:"type:uuid;primaryKey"`
	SubscriptionID    uuid.UUID `gorm:"type:uuid"`
	EstablishmentID   uuid.UUID `gorm:"type:uuid"`
	PeriodStart       *time.Time
	PeriodEnd         *time.Time
	AmountCents       int64
	Currency          string
	Provider          string
	ProviderInvoiceID *string
	ProviderPaymentID *string
	PayCurrency       *string
	PayAmountExpected *string `gorm:"type:numeric"`
	PayAmountReceived *string `gorm:"type:numeric"`
	Status            string
	ProviderStatus    *string
	PaidAt            *time.Time
	InvoiceURL        *string
	ExpiresAt         *time.Time
	CreatedAt         time.Time
	UpdatedAt         time.Time
}

type Invoice struct {
	ID              uuid.UUID `gorm:"type:uuid;primaryKey"`
	Number          string
	PaymentID       uuid.UUID `gorm:"type:uuid"`
	EstablishmentID uuid.UUID `gorm:"type:uuid"`
	AmountCents     int64
	Currency        string
	IssuedAt        time.Time
	BillingSnapshot JSON
	PDFStorageKey   *string `gorm:"column:pdf_storage_key"`
}

type PaymentProviderEvent struct {
	ID              uuid.UUID `gorm:"type:uuid;primaryKey"`
	Provider        string
	EventKey        string
	Payload         JSON
	SignatureValid  bool
	ProcessedAt     *time.Time
	ProcessingError *string
	ReceivedAt      time.Time
}

// Audit categories and severities.
const (
	AuditSecurity      = "SECURITY"
	AuditAdmin         = "ADMIN"
	AuditBilling       = "BILLING"
	AuditEstablishment = "ESTABLISHMENT"
	AuditProfile       = "PROFILE"
	AuditAuth          = "AUTH"

	SeverityInfo     = "INFO"
	SeverityWarn     = "WARN"
	SeverityCritical = "CRITICAL"
)

type AuditLog struct {
	ID          int64 `gorm:"primaryKey;->"`
	OccurredAt  time.Time
	ActorUserID *uuid.UUID `gorm:"type:uuid"`
	ActorRole   *string
	Action      string
	Category    string
	Severity    string
	TargetType  *string
	TargetID    *string
	IPHash      *string
	UserAgent   *string
	RequestID   *string
	Metadata    JSON
}

// Payment statuses (same values as pkg/payment).
const (
	PaymentPending    = "PENDING"
	PaymentConfirming = "CONFIRMING"
	PaymentPartial    = "PARTIAL"
	PaymentPaid       = "PAID"
	PaymentFailed     = "FAILED"
	PaymentExpired    = "EXPIRED"
	PaymentRefunded   = "REFUNDED"
)
