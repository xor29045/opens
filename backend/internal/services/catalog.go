package services

import (
	"context"
	"time"

	"github.com/google/uuid"

	"opensex/backend/internal/apperr"
	"opensex/backend/internal/database"
	"opensex/backend/internal/models"
	"opensex/backend/internal/rbac"
	"opensex/backend/internal/repositories"
	"opensex/backend/internal/validators"
	"opensex/backend/pkg/ratelimit"
)

// CatalogService serves the public catalogue. Contact details are filtered
// server-side according to their visibility and the viewer (RM-40 to RM-43).
type CatalogService struct{ *Services }

var LimitClientContacts = []ratelimit.Limit{ratelimit.PerHour(60)}

type viewMode int

const (
	viewPublic viewMode = iota
	viewPreview
)

type PublicContact struct {
	Type       string `json:"type"`
	Value      string `json:"value"`
	Visibility string `json:"visibility,omitempty"`
}

type PublicPhoto struct {
	ID        uuid.UUID `json:"id"`
	URL       string    `json:"url"`
	Width     int       `json:"width"`
	Height    int       `json:"height"`
	IsPrimary bool      `json:"is_primary"`
}

type PublicRate struct {
	Label           string `json:"label"`
	DurationMinutes *int   `json:"duration_minutes"`
	AmountCents     int64  `json:"amount_cents"`
	Currency        string `json:"currency"`
}

type PublicCategory struct {
	Slug string `json:"slug"`
	Name string `json:"name"`
}

type PublicPlace struct {
	Slug string `json:"slug"`
	Name string `json:"name"`
}

type PublicEstablishmentRef struct {
	Slug           string       `json:"slug"`
	DisplayName    string       `json:"display_name"`
	LegalName      string       `json:"legal_name,omitempty"`
	ApprovalNumber string       `json:"approval_number,omitempty"`
	City           *PublicPlace `json:"city"`
	Region         *PublicPlace `json:"region"`
}

type ProfileCard struct {
	Slug           string                 `json:"slug"`
	DisplayName    string                 `json:"display_name"`
	Headline       string                 `json:"headline"`
	Age            *int                   `json:"age"`
	PriceFromCents *int64                 `json:"price_from_cents"`
	PhotoURL       *string                `json:"photo_url"`
	Establishment  PublicEstablishmentRef `json:"establishment"`
	PublishedAt    *time.Time             `json:"published_at,omitempty"`
}

type PublicProfile struct {
	ID                  uuid.UUID              `json:"id"`
	Slug                string                 `json:"slug"`
	DisplayName         string                 `json:"display_name"`
	Headline            string                 `json:"headline"`
	Description         string                 `json:"description"`
	Age                 *int                   `json:"age"`
	HeightCm            *int16                 `json:"height_cm"`
	WeightKg            *int16                 `json:"weight_kg"`
	HairColor           *string                `json:"hair_color"`
	EyeColor            *string                `json:"eye_color"`
	BodyType            *string                `json:"body_type"`
	Languages           []string               `json:"languages"`
	ExtraAttributes     models.JSON            `json:"extra_attributes"`
	PriceFromCents      *int64                 `json:"price_from_cents"`
	Photos              []PublicPhoto          `json:"photos"`
	Rates               []PublicRate           `json:"rates"`
	Categories          []PublicCategory       `json:"categories"`
	Contacts            []PublicContact        `json:"contacts"`
	ContactsRestricted  bool                   `json:"contacts_restricted"`
	ContactsRateLimited bool                   `json:"contacts_rate_limited,omitempty"`
	Establishment       PublicEstablishmentRef `json:"establishment"`
	Published           bool                   `json:"published"`
}

func ageOf(b *time.Time, now time.Time) *int {
	if b == nil {
		return nil
	}
	a := AgeAt(*b, now)
	return &a
}

func place(slug, name *string) *PublicPlace {
	if slug == nil || name == nil {
		return nil
	}
	return &PublicPlace{Slug: *slug, Name: *name}
}

type SearchParams struct {
	repositories.CatalogFilter
	Page repositories.Page
}

func (s *CatalogService) Search(ctx context.Context, p SearchParams) ([]ProfileCard, int64, error) {
	var c apperr.Collector
	if p.AgeMin != 0 && (p.AgeMin < 18 || p.AgeMin > 99) {
		c.Add("age_min", "out_of_range")
	}
	if p.AgeMax != 0 && (p.AgeMax < 18 || p.AgeMax > 99) {
		c.Add("age_max", "out_of_range")
	}
	for _, v := range p.HairColors {
		if !validators.OneOf(v, validators.HairColors) {
			c.Add("hair_color", "invalid_value")
		}
	}
	for _, v := range p.EyeColors {
		if !validators.OneOf(v, validators.EyeColors) {
			c.Add("eye_color", "invalid_value")
		}
	}
	for _, v := range p.BodyTypes {
		if !validators.OneOf(v, validators.BodyTypes) {
			c.Add("body_type", "invalid_value")
		}
	}
	for _, v := range p.Languages {
		if !validators.Language(v) {
			c.Add("languages", "invalid_value")
		}
	}
	if p.Sort != "" && !validators.OneOf(p.Sort, []string{"recent", "price_asc", "price_desc"}) {
		c.Add("sort", "invalid_value")
	}
	if len(p.Categories) > 20 {
		c.Add("categories", "too_many")
	}
	if err := c.Err(); err != nil {
		return nil, 0, err
	}
	p.Today = s.now()
	rows, total, err := s.Repos.Catalog.Search(ctx, p.CatalogFilter, p.Page)
	if err != nil {
		return nil, 0, apperr.Internal(err)
	}
	cards, err := s.cards(ctx, rows)
	return cards, total, err
}

func (s *CatalogService) cards(ctx context.Context, rows []repositories.CatalogRow) ([]ProfileCard, error) {
	ids := make([]uuid.UUID, len(rows))
	for i, r := range rows {
		ids[i] = r.ID
	}
	primaries, err := s.Repos.Profiles.PrimaryPhotos(ctx, ids)
	if err != nil {
		return nil, apperr.Internal(err)
	}
	now := s.now()
	out := make([]ProfileCard, len(rows))
	for i, r := range rows {
		card := ProfileCard{Slug: r.Slug, DisplayName: r.DisplayName, Headline: r.Headline, Age: ageOf(r.BirthDateAttested, now),
			PriceFromCents: r.PriceFromCents, PublishedAt: r.PublishedAt,
			Establishment: PublicEstablishmentRef{Slug: r.EstablishmentSlug, DisplayName: r.EstablishmentName,
				City: place(r.CitySlug, r.CityName), Region: place(r.RegionSlug, r.RegionName)}}
		if ph, ok := primaries[r.ID]; ok {
			u := s.Profiles.photoURL(ctx, ph.StorageKey, true)
			card.PhotoURL = &u
		}
		out[i] = card
	}
	return out, nil
}

// canSeeClientContacts implements RM-40/RM-43 for a given establishment.
func canSeeClientContacts(actor *rbac.Actor, establishmentID uuid.UUID) bool {
	if actor == nil {
		return false
	}
	switch actor.Role {
	case models.RoleClient, models.RoleAdmin:
		return true
	case models.RoleManager, models.RoleProfessional:
		return actor.EstablishmentID == establishmentID
	}
	return false
}

// ProfileBySlug returns a published profile (404 otherwise).
func (s *CatalogService) ProfileBySlug(ctx context.Context, slug string) (*PublicProfile, error) {
	p, err := s.Repos.Profiles.BySlug(ctx, slug)
	if err != nil || !p.IsPublished {
		if err == nil || database.IsNotFound(err) {
			return nil, apperr.NotFound("profile")
		}
		return nil, apperr.Internal(err)
	}
	out, err := s.build(ctx, p, viewPublic)
	if err != nil {
		return nil, err
	}
	s.Stats.RecordView(ctx, p.ID)
	return out, nil
}

func (s *CatalogService) build(ctx context.Context, p *models.Profile, mode viewMode) (*PublicProfile, error) {
	e, err := s.Repos.Establishments.ByID(ctx, p.EstablishmentID)
	if err != nil {
		return nil, apperr.Internal(err)
	}
	ref, err := s.establishmentRef(ctx, e, true)
	if err != nil {
		return nil, err
	}
	out := &PublicProfile{ID: p.ID, Slug: p.Slug, DisplayName: p.DisplayName, Headline: p.Headline, Description: p.Description,
		Age: ageOf(p.BirthDateAttested, s.now()), HeightCm: p.HeightCm, WeightKg: p.WeightKg, HairColor: p.HairColor,
		EyeColor: p.EyeColor, BodyType: p.BodyType, Languages: []string(p.Languages), ExtraAttributes: p.ExtraAttributes,
		PriceFromCents: p.PriceFromCents, Establishment: *ref, Published: p.IsPublished,
		Photos: []PublicPhoto{}, Rates: []PublicRate{}, Categories: []PublicCategory{}, Contacts: []PublicContact{}}
	if out.Languages == nil {
		out.Languages = []string{}
	}
	photos, err := s.Repos.Profiles.Photos(ctx, p.ID)
	if err != nil {
		return nil, apperr.Internal(err)
	}
	for _, ph := range photos {
		out.Photos = append(out.Photos, PublicPhoto{ID: ph.ID, URL: s.Profiles.photoURL(ctx, ph.StorageKey, mode == viewPublic),
			Width: ph.Width, Height: ph.Height, IsPrimary: ph.IsPrimary})
	}
	rates, err := s.Repos.Profiles.Rates(ctx, p.ID)
	if err != nil {
		return nil, apperr.Internal(err)
	}
	for _, r := range rates {
		out.Rates = append(out.Rates, PublicRate{Label: r.Label, DurationMinutes: r.DurationMinutes, AmountCents: r.AmountCents, Currency: r.Currency})
	}
	cats, err := s.Repos.Profiles.Categories(ctx, p.ID)
	if err != nil {
		return nil, apperr.Internal(err)
	}
	for _, c := range cats {
		out.Categories = append(out.Categories, PublicCategory{Slug: c.Slug, Name: c.Name})
	}
	contacts, err := s.Repos.Profiles.Contacts(ctx, p.ID)
	if err != nil {
		return nil, apperr.Internal(err)
	}
	generic := make([]genericContact, len(contacts))
	for i, c := range contacts {
		generic[i] = genericContact{c.Type, c.Value, c.Visibility, false}
	}
	out.Contacts, out.ContactsRestricted, out.ContactsRateLimited = s.filterContacts(ctx, generic, e.ID, p.ID, mode)
	return out, nil
}

type genericContact struct {
	Type, Value, Visibility string
	VerificationPhone       bool
}

// filterContacts removes every contact the viewer may not see. It reports
// whether CLIENTS contacts exist but were withheld.
func (s *CatalogService) filterContacts(ctx context.Context, cs []genericContact, establishmentID, statsProfile uuid.UUID, mode viewMode) ([]PublicContact, bool, bool) {
	actor := rbac.ActorFrom(ctx)
	allowClients := mode == viewPreview || canSeeClientContacts(actor, establishmentID)
	hasClients := false
	for _, c := range cs {
		if c.Visibility == models.VisibilityClients && !c.VerificationPhone {
			hasClients = true
		}
	}
	rateLimited := false
	if allowClients && hasClients && mode == viewPublic && actor.Is(models.RoleClient) {
		res, err := s.Limiter.Allow(ctx, "contacts:clients:"+actor.UserID.String(), LimitClientContacts...)
		if err == nil && !res.Allowed {
			allowClients, rateLimited = false, true
			s.Audit.Security(ctx, "catalog.contacts.rate_limited", models.SeverityWarn, nil)
		} else if statsProfile != uuid.Nil {
			s.Stats.RecordContactView(ctx, statsProfile)
		}
	}
	out := []PublicContact{}
	restricted := false
	for _, c := range cs {
		if c.VerificationPhone || c.Visibility == models.VisibilityHidden {
			continue
		}
		if c.Visibility == models.VisibilityClients && !allowClients {
			restricted = true
			continue
		}
		pc := PublicContact{Type: c.Type, Value: c.Value}
		if mode == viewPreview {
			pc.Visibility = c.Visibility
		}
		out = append(out, pc)
	}
	return out, restricted, rateLimited
}

func (s *CatalogService) establishmentRef(ctx context.Context, e *models.Establishment, legal bool) (*PublicEstablishmentRef, error) {
	ref := &PublicEstablishmentRef{Slug: e.Slug, DisplayName: e.DisplayName}
	if legal {
		ref.LegalName = e.LegalName
		ref.ApprovalNumber = deref(e.ApprovalNumber)
	}
	regs, err := s.Repos.Taxonomy.RegionsByIDs(ctx, compactIDs(e.RegionID, e.CityID))
	if err != nil {
		return nil, apperr.Internal(err)
	}
	if e.CityID != nil {
		if r, ok := regs[*e.CityID]; ok {
			ref.City = &PublicPlace{Slug: r.Slug, Name: r.Name}
		}
	}
	if e.RegionID != nil {
		if r, ok := regs[*e.RegionID]; ok {
			ref.Region = &PublicPlace{Slug: r.Slug, Name: r.Name}
		}
	}
	return ref, nil
}

type PublicEstablishment struct {
	PublicEstablishmentRef
	Description        string          `json:"description"`
	OpeningHours       models.JSON     `json:"opening_hours"`
	Address            *string         `json:"address"`
	Contacts           []PublicContact `json:"contacts"`
	ContactsRestricted bool            `json:"contacts_restricted"`
	Profiles           []ProfileCard   `json:"profiles"`
}

// EstablishmentBySlug returns the public page of a verified, entitled establishment.
func (s *CatalogService) EstablishmentBySlug(ctx context.Context, slug string) (*PublicEstablishment, error) {
	e, err := s.Repos.Establishments.BySlug(ctx, slug)
	if err != nil {
		if database.IsNotFound(err) {
			return nil, apperr.NotFound("establishment")
		}
		return nil, apperr.Internal(err)
	}
	st, err := s.Publication.EstablishmentState(ctx, e)
	if err != nil {
		return nil, err
	}
	if len(st.Reasons()) > 0 {
		return nil, apperr.NotFound("establishment")
	}
	ref, err := s.establishmentRef(ctx, e, true)
	if err != nil {
		return nil, err
	}
	out := &PublicEstablishment{PublicEstablishmentRef: *ref, Description: e.Description, OpeningHours: e.OpeningHours}
	if e.AddressPublic {
		out.Address = e.Address
	}
	contacts, err := s.Repos.Establishments.Contacts(ctx, e.ID)
	if err != nil {
		return nil, apperr.Internal(err)
	}
	generic := make([]genericContact, len(contacts))
	for i, c := range contacts {
		generic[i] = genericContact{c.Type, c.Value, c.Visibility, c.IsVerificationPhone}
	}
	out.Contacts, out.ContactsRestricted, _ = s.filterContacts(ctx, generic, e.ID, uuid.Nil, viewPublic)
	rows, err := s.Repos.Catalog.PublishedOfEstablishment(ctx, e.ID, 200)
	if err != nil {
		return nil, apperr.Internal(err)
	}
	if out.Profiles, err = s.cards(ctx, rows); err != nil {
		return nil, err
	}
	return out, nil
}

func (s *CatalogService) Regions(ctx context.Context) ([]models.Region, error) {
	r, err := s.Repos.Taxonomy.Regions(ctx, true)
	if err != nil {
		return nil, apperr.Internal(err)
	}
	return r, nil
}

func (s *CatalogService) Categories(ctx context.Context) ([]models.Category, error) {
	c, err := s.Repos.Taxonomy.Categories(ctx, true)
	if err != nil {
		return nil, apperr.Internal(err)
	}
	return c, nil
}

func (s *CatalogService) SelectableEstablishments(ctx context.Context) ([]repositories.EstablishmentSummary, error) {
	out, err := s.Repos.Establishments.Selectable(ctx)
	if err != nil {
		return nil, apperr.Internal(err)
	}
	return out, nil
}

type Attributes struct {
	HairColors      []string            `json:"hair_colors"`
	EyeColors       []string            `json:"eye_colors"`
	BodyTypes       []string            `json:"body_types"`
	Languages       []string            `json:"languages"`
	ExtraAttributes map[string][]string `json:"extra_attributes"`
	ContactTypes    []string            `json:"contact_types"`
	Visibilities    []string            `json:"visibilities"`
	Sorts           []string            `json:"sorts"`
}

func (s *CatalogService) Attributes() Attributes {
	return Attributes{HairColors: validators.HairColors, EyeColors: validators.EyeColors, BodyTypes: validators.BodyTypes,
		Languages: validators.Languages, ExtraAttributes: validators.ExtraAttributes, ContactTypes: models.ContactTypes,
		Visibilities: []string{models.VisibilityPublic, models.VisibilityClients, models.VisibilityHidden},
		Sorts:        []string{"recent", "price_asc", "price_desc"}}
}
