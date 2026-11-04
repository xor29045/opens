package services

import (
	"context"

	"github.com/google/uuid"

	"opensex/backend/internal/apperr"
	"opensex/backend/internal/models"
)

// PublicationService maintains the is_published projection (RM-22) and
// explains why a profile is not published.
type PublicationService struct{ *Services }

// Recompute refreshes the projection of the given establishments (all if none).
func (s *PublicationService) Recompute(ctx context.Context, establishmentIDs ...uuid.UUID) error {
	if _, err := s.Repos.Profiles.RecomputePublication(ctx, s.now(), establishmentIDs...); err != nil {
		return apperr.Internal(err)
	}
	return nil
}

// Reason codes returned by publication-status endpoints.
const (
	ReasonManagerInactive     = "MANAGER_ACCOUNT_INACTIVE"
	ReasonEstablishmentHidden = "ESTABLISHMENT_HIDDEN"
	ReasonNoEntitlement       = "ESTABLISHMENT_SUBSCRIPTION_INACTIVE"
	ReasonNotActive           = "PROFILE_NOT_ACTIVE"
	ReasonHidden              = "PROFILE_HIDDEN"
	ReasonAccountInactive     = "PROFESSIONAL_ACCOUNT_INACTIVE"
	ReasonQuotaExceeded       = "ACTIVE_PROFILES_QUOTA_EXCEEDED"
)

// EstablishmentState gathers the establishment-level conditions.
type EstablishmentState struct {
	Establishment *models.Establishment
	ManagerActive bool
	Entitled      bool
	MaxActive     *int
}

func (s *PublicationService) EstablishmentState(ctx context.Context, e *models.Establishment) (*EstablishmentState, error) {
	m, err := s.Repos.Users.ByID(ctx, e.ManagerUserID)
	if err != nil {
		return nil, apperr.Internal(err)
	}
	entitled, maxActive, err := s.Repos.Profiles.HasEntitlement(ctx, e.ID, s.now())
	if err != nil {
		return nil, apperr.Internal(err)
	}
	return &EstablishmentState{Establishment: e, ManagerActive: m.Status == models.UserActive, Entitled: entitled, MaxActive: maxActive}, nil
}

// EstablishmentReasons lists the establishment-level blockers.
func (st *EstablishmentState) Reasons() []string {
	var r []string
	if !st.ManagerActive {
		r = append(r, ReasonManagerInactive)
	}
	if st.Establishment.IsHidden {
		r = append(r, ReasonEstablishmentHidden)
	}
	if !st.Entitled {
		r = append(r, ReasonNoEntitlement)
	}
	return r
}

// ProfileReasons explains why p is (not) published; empty when published.
func (s *PublicationService) ProfileReasons(ctx context.Context, st *EstablishmentState, p *models.Profile) ([]string, error) {
	if p.IsPublished {
		return []string{}, nil
	}
	reasons := append([]string{}, st.Reasons()...)
	if p.Status != models.ProfileActive {
		reasons = append(reasons, ReasonNotActive)
	}
	if p.IsHidden {
		reasons = append(reasons, ReasonHidden)
	}
	if p.UserID != nil {
		u, err := s.Repos.Users.ByID(ctx, *p.UserID)
		if err != nil || u.Status != models.UserActive {
			reasons = append(reasons, ReasonAccountInactive)
		}
	}
	// A photo is no longer required to publish (ADR-027): an active profile of
	// a subscribed house shows in the catalogue and the deck at once.
	if len(reasons) == 0 && st.MaxActive != nil {
		reasons = append(reasons, ReasonQuotaExceeded)
	}
	return reasons, nil
}
