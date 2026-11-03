package services

import (
	"context"
	"encoding/json"
	"strconv"
	"strings"

	"github.com/google/uuid"

	"opensex/backend/internal/apperr"
	"opensex/backend/internal/database"
	"opensex/backend/internal/models"
	"opensex/backend/internal/rbac"
	"opensex/backend/internal/validators"
	"opensex/backend/pkg/crypto"
)

// EstablishmentService manages the establishment of the acting MANAGER
// (RM-10 to RM-13). The establishment is always derived from the actor.
type EstablishmentService struct{ *Services }

func (s *EstablishmentService) own(ctx context.Context, actor *rbac.Actor) (*models.Establishment, error) {
	e, err := s.Repos.Establishments.ByID(ctx, actor.EstablishmentID)
	if err != nil {
		if database.IsNotFound(err) {
			return nil, apperr.NotFound("establishment")
		}
		return nil, apperr.Internal(err)
	}
	return e, nil
}

type EstablishmentDetail struct {
	Establishment *models.Establishment
	Contacts      []models.EstablishmentContact
	Region        *models.Region
	City          *models.Region
}

func (s *EstablishmentService) detail(ctx context.Context, e *models.Establishment) (*EstablishmentDetail, error) {
	contacts, err := s.Repos.Establishments.Contacts(ctx, e.ID)
	if err != nil {
		return nil, apperr.Internal(err)
	}
	d := &EstablishmentDetail{Establishment: e, Contacts: contacts}
	regs, err := s.Repos.Taxonomy.RegionsByIDs(ctx, compactIDs(e.RegionID, e.CityID))
	if err != nil {
		return nil, apperr.Internal(err)
	}
	if e.RegionID != nil {
		if r, ok := regs[*e.RegionID]; ok {
			d.Region = &r
		}
	}
	if e.CityID != nil {
		if r, ok := regs[*e.CityID]; ok {
			d.City = &r
		}
	}
	return d, nil
}

func compactIDs(ids ...*uuid.UUID) []uuid.UUID {
	var out []uuid.UUID
	for _, id := range ids {
		if id != nil {
			out = append(out, *id)
		}
	}
	return out
}

func (s *EstablishmentService) Get(ctx context.Context, actor *rbac.Actor) (*EstablishmentDetail, error) {
	e, err := s.own(ctx, actor)
	if err != nil {
		return nil, err
	}
	return s.detail(ctx, e)
}

type EstablishmentInput struct {
	DisplayName   string
	LegalName     string
	Description   string
	OpeningHours  json.RawMessage
	RegionID      *uuid.UUID
	CityID        *uuid.UUID
	Address       *string
	AddressPublic bool
}

func (s *EstablishmentService) Update(ctx context.Context, actor *rbac.Actor, in EstablishmentInput) (*EstablishmentDetail, error) {
	var c apperr.Collector
	displayName, code := validators.PlainText(in.DisplayName, 80)
	if code != "" {
		c.Add("display_name", code)
	}
	legalName, code := validators.PlainText(in.LegalName, 160)
	if code != "" {
		c.Add("legal_name", code)
	}
	description, code := validators.PlainText(in.Description, 5000)
	if code != "" {
		c.Add("description", code)
	}
	hours, code := validators.OpeningHours(in.OpeningHours)
	if code != "" {
		c.Add("opening_hours", code)
	}
	var address *string
	if in.Address != nil {
		a, code := validators.PlainText(*in.Address, 300)
		if code != "" {
			c.Add("address", code)
		} else if a != "" {
			address = &a
		}
	}
	if err := s.checkLocation(ctx, in.RegionID, in.CityID, &c); err != nil {
		return nil, err
	}
	if err := c.Err(); err != nil {
		return nil, err
	}
	var out *EstablishmentDetail
	err := s.tx(ctx, func(ctx context.Context) error {
		e, err := s.Repos.Establishments.ByIDForUpdate(ctx, actor.EstablishmentID)
		if err != nil {
			return apperr.NotFound("establishment")
		}
		if e.DisplayName != displayName && displayName != "" {
			slug, err := s.uniqueSlug(ctx, displayName, e.ID)
			if err != nil {
				return err
			}
			e.Slug = slug
		}
		e.DisplayName, e.LegalName, e.Description = displayName, legalName, description
		e.OpeningHours = models.JSON(hours)
		e.RegionID, e.CityID = in.RegionID, in.CityID
		e.Address, e.AddressPublic = address, in.AddressPublic && address != nil
		if err := s.Repos.Establishments.Save(ctx, e); err != nil {
			return apperr.Internal(err)
		}
		s.Audit.Record(ctx, AuditEvent{Action: "establishment.updated", Category: models.AuditEstablishment,
			TargetType: "establishment", TargetID: e.ID.String()})

		out, err = s.detail(ctx, e)
		return err
	})
	return out, err
}

func (s *EstablishmentService) checkLocation(ctx context.Context, regionID, cityID *uuid.UUID, c *apperr.Collector) error {
	if regionID == nil {
		if cityID != nil {
			c.Add("region_id", "required")
		}
		return nil
	}
	region, err := s.Repos.Taxonomy.RegionByID(ctx, *regionID)
	if err != nil {
		if !database.IsNotFound(err) {
			return apperr.Internal(err)
		}
		c.Add("region_id", "unknown_region")
		return nil
	}
	if region.Kind != models.RegionKindRegion || !region.IsActive {
		c.Add("region_id", "unknown_region")
	}
	if cityID != nil {
		city, err := s.Repos.Taxonomy.RegionByID(ctx, *cityID)
		if err != nil || city.Kind != models.RegionKindCity || city.ParentID == nil || *city.ParentID != *regionID || !city.IsActive {
			c.Add("city_id", "unknown_city")
		}
	}
	return nil
}

func (s *EstablishmentService) uniqueSlug(ctx context.Context, name string, id uuid.UUID) (string, error) {
	base := validators.Slugify(name)
	if base == "" {
		base = "etablissement"
	}
	slug := base
	for i := 0; i < 5; i++ {
		taken, err := s.Repos.Establishments.SlugTaken(ctx, slug, id)
		if err != nil {
			return "", apperr.Internal(err)
		}
		if !taken {
			return slug, nil
		}
		slug = base + "-" + strings.ToLower(crypto.RandomPassword(4))
	}
	return "", apperr.Conflict("slug_unavailable", "could not allocate a slug")
}

type ContactInput struct {
	Type                string
	Value               string
	Visibility          string
	IsVerificationPhone bool
}

const maxContacts = 20

func normalizeContacts(in []ContactInput, allowVerification bool) ([]ContactInput, error) {
	var c apperr.Collector
	if len(in) > maxContacts {
		c.Add("contacts", "too_many")
		return nil, c.Err()
	}
	seen := map[string]bool{}
	verification := 0
	out := make([]ContactInput, 0, len(in))
	for i, ct := range in {
		field := "contacts[" + itoa(i) + "]"
		if !validators.OneOf(ct.Type, models.ContactTypes) {
			c.Add(field+".type", "invalid_contact_type")
			continue
		}
		if !validators.OneOf(ct.Visibility, []string{models.VisibilityPublic, models.VisibilityClients, models.VisibilityHidden}) {
			c.Add(field+".visibility", "invalid_visibility")
		}
		v, code := validators.NormalizeContact(ct.Type, ct.Value)
		if code != "" {
			c.Add(field+".value", code)
			continue
		}
		if ct.IsVerificationPhone {
			if !allowVerification || ct.Type != "PHONE" {
				c.Add(field+".is_verification_phone", "not_allowed")
			}
			verification++
		}
		key := ct.Type + "|" + v
		if seen[key] {
			c.Add(field+".value", "duplicate")
		}
		seen[key] = true
		ct.Value = v
		out = append(out, ct)
	}
	if verification > 1 {
		c.Add("contacts", "single_verification_phone")
	}
	return out, c.Err()
}

func (s *EstablishmentService) ReplaceContacts(ctx context.Context, actor *rbac.Actor, in []ContactInput) ([]models.EstablishmentContact, error) {
	norm, err := normalizeContacts(in, true)
	if err != nil {
		return nil, err
	}
	rows := make([]models.EstablishmentContact, len(norm))
	for i, ct := range norm {
		rows[i] = models.EstablishmentContact{ID: models.NewID(), EstablishmentID: actor.EstablishmentID, Type: ct.Type, Value: ct.Value,
			Visibility: ct.Visibility, IsVerificationPhone: ct.IsVerificationPhone, Position: i}
	}
	err = s.tx(ctx, func(ctx context.Context) error {
		if err := s.Repos.Establishments.ReplaceContacts(ctx, actor.EstablishmentID, rows); err != nil {
			return apperr.Internal(err)
		}
		s.Audit.Record(ctx, AuditEvent{Action: "establishment.contacts.updated", Category: models.AuditEstablishment,
			TargetType: "establishment", TargetID: actor.EstablishmentID.String(), Metadata: map[string]any{"count": len(rows)}})
		return nil
	})
	return rows, err
}

// IsComplete reports whether the public page has the minimum information
// (name and location) shown in the manager's checklist.
func IsComplete(e *models.Establishment) bool {
	return e.DisplayName != "" && e.RegionID != nil && e.CityID != nil
}

func itoa(i int) string { return strconv.Itoa(i) }
