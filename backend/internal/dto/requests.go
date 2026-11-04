// Package dto defines the request and response bodies of the REST API.
// Requests are decoded strictly (unknown fields rejected); responses are
// explicit whitelists built from the models (no model is serialised as is).
package dto

import (
	"encoding/json"

	"github.com/google/uuid"
)

type RegisterManagerRequest struct {
	Username    string `json:"username"`
	Password    string `json:"password"`
	AcceptTerms bool   `json:"accept_terms"`
	AcceptCGV   bool   `json:"accept_cgv"`
	AcceptDPA   bool   `json:"accept_dpa"`
}

type LoginRequest struct {
	Username string `json:"username"`
	Password string `json:"password"`
}

type ChangePasswordRequest struct {
	CurrentPassword string `json:"current_password"`
	NewPassword     string `json:"new_password"`
}

type RecoverRequest struct {
	Username     string `json:"username"`
	RecoveryCode string `json:"recovery_code"`
	NewPassword  string `json:"new_password"`
}

type PasswordRequest struct {
	Password string `json:"password"`
}

type ReasonRequest struct {
	Reason string `json:"reason"`
}

type ContactRequest struct {
	Type                string `json:"type"`
	Value               string `json:"value"`
	Visibility          string `json:"visibility"`
	IsVerificationPhone bool   `json:"is_verification_phone"`
}

type ContactsRequest struct {
	Contacts []ContactRequest `json:"contacts"`
}

type EstablishmentRequest struct {
	DisplayName   string          `json:"display_name"`
	LegalName     string          `json:"legal_name"`
	Description   string          `json:"description"`
	OpeningHours  json.RawMessage `json:"opening_hours"`
	RegionID      *uuid.UUID      `json:"region_id"`
	CityID        *uuid.UUID      `json:"city_id"`
	Address       *string         `json:"address"`
	AddressPublic bool            `json:"address_public"`
}

type ProfileRequest struct {
	DisplayName     string            `json:"display_name"`
	Headline        string            `json:"headline"`
	Description     string            `json:"description"`
	Age             *int              `json:"age"`
	HeightCm        *int              `json:"height_cm"`
	WeightKg        *int              `json:"weight_kg"`
	HairColor       *string           `json:"hair_color"`
	EyeColor        *string           `json:"eye_color"`
	BodyType        *string           `json:"body_type"`
	Languages       []string          `json:"languages"`
	ExtraAttributes map[string]string `json:"extra_attributes"`
}

type CategoriesRequest struct {
	CategoryIDs []uuid.UUID `json:"category_ids"`
}

type RateRequest struct {
	Label           string `json:"label"`
	DurationMinutes *int   `json:"duration_minutes"`
	AmountCents     int64  `json:"amount_cents"`
}

type RatesRequest struct {
	Rates []RateRequest `json:"rates"`
}

type PhotoPatchRequest struct {
	IsPrimary *bool `json:"is_primary"`
	Position  *int  `json:"position"`
}

type PhotoOrderRequest struct {
	PhotoIDs []uuid.UUID `json:"photo_ids"`
}

type SubscribeRequest struct {
	PlanCode string `json:"plan_code"`
}

type ReconcileRequest struct {
	Reason    string `json:"reason"`
	ForcePaid bool   `json:"force_paid"`
}

type CategoryRequest struct {
	ParentID    *uuid.UUID `json:"parent_id"`
	Slug        string     `json:"slug"`
	Name        string     `json:"name"`
	Description string     `json:"description"`
	Position    int        `json:"position"`
	IsActive    *bool      `json:"is_active"`
}

type RegionRequest struct {
	ParentID *uuid.UUID `json:"parent_id"`
	Slug     string     `json:"slug"`
	Name     string     `json:"name"`
	Kind     string     `json:"kind"`
	IsActive *bool      `json:"is_active"`
}

// ProfileCreateRequest creates a profile of the house; username and
// password, when given, create the professional's login (ADR-022).
type ProfileCreateRequest struct {
	ProfileRequest
	Username string `json:"username"`
	Password string `json:"password"`
}

type AccountRequest struct {
	Username string `json:"username"`
	Password string `json:"password"`
}

// DiscoverRequest asks for deck cards, leaving out the profiles the visitor
// already liked or passed (kept in the browser, ADR-023).
type DiscoverRequest struct {
	Exclude []uuid.UUID     `json:"exclude"`
	Limit   int             `json:"limit"`
	Filters DiscoverFilters `json:"filters"`
}

// DiscoverFilters narrow the deck. Lists match any of their values; empty
// fields do not filter. Prices are in euros.
type DiscoverFilters struct {
	City       string   `json:"city"`
	HairColors []string `json:"hair_color"`
	EyeColors  []string `json:"eye_color"`
	SkinTones  []string `json:"skin_tone"`
	Genders    []string `json:"gender"`
	HeightMin  int      `json:"height_min"`
	HeightMax  int      `json:"height_max"`
	AgeMin     int      `json:"age_min"`
	AgeMax     int      `json:"age_max"`
	PriceMin   int      `json:"price_min"`
	PriceMax   int      `json:"price_max"`
}
