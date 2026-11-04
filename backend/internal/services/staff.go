package services

import (
	"context"

	"github.com/google/uuid"

	"opensex/backend/internal/apperr"
	"opensex/backend/internal/models"
	"opensex/backend/internal/rbac"
	"opensex/backend/internal/validators"
)

// StaffService covers the manager's actions on the professional accounts of
// their establishment: rejecting a pending registration, suspending,
// restoring and resetting the password of a linked account (RM-01b, RM-07, RM-20).
type StaffService struct{ *Services }

func reasonRequired(reason string) (string, error) {
	r, code := validators.PlainText(reason, 500)
	if code != "" || r == "" {
		return "", apperr.Invalid("reason", firstNonEmpty(code, "required"))
	}
	return r, nil
}

func (s *StaffService) linkedAccount(ctx context.Context, actor *rbac.Actor, profileID uuid.UUID) (*models.Profile, *models.User, error) {
	p, err := s.Profiles.resolve(ctx, actor, profileID, false)
	if err != nil {
		return nil, nil, err
	}
	if p.UserID == nil {
		return nil, nil, apperr.NotFound("linked account")
	}
	u, err := s.Repos.Users.ByID(ctx, *p.UserID)
	if err != nil {
		return nil, nil, apperr.NotFound("linked account")
	}
	return p, u, nil
}

func (s *StaffService) SuspendAccount(ctx context.Context, actor *rbac.Actor, profileID uuid.UUID, reason string) error {
	reason, err := reasonRequired(reason)
	if err != nil {
		return err
	}
	p, u, err := s.linkedAccount(ctx, actor, profileID)
	if err != nil {
		return err
	}
	err = s.tx(ctx, func(ctx context.Context) error {
		if err := s.Repos.Users.Update(ctx, u.ID, map[string]any{"status": models.UserSuspended, "status_reason": reason}); err != nil {
			return apperr.Internal(err)
		}
		if err := s.Auth.RevokeAllSessions(ctx, u.ID, nil, models.RevokeSuspended); err != nil {
			return err
		}
		return s.Publication.Recompute(ctx, p.EstablishmentID)
	})
	if err == nil {
		_ = s.Denylist.DenyUser(ctx, u.ID)
	}
	return err
}

func (s *StaffService) RestoreAccount(ctx context.Context, actor *rbac.Actor, profileID uuid.UUID) error {
	p, u, err := s.linkedAccount(ctx, actor, profileID)
	if err != nil {
		return err
	}
	if u.Status != models.UserSuspended {
		return apperr.Conflict("invalid_state", "account is not suspended")
	}
	err = s.tx(ctx, func(ctx context.Context) error {
		if err := s.Repos.Users.Update(ctx, u.ID, map[string]any{"status": models.UserActive, "status_reason": nil}); err != nil {
			return apperr.Internal(err)
		}
		return s.Publication.Recompute(ctx, p.EstablishmentID)
	})
	if err == nil {
		_ = s.Denylist.AllowUser(ctx, u.ID)
	}
	return err
}

// ResetPassword gives a one-time temporary password for a professional of
// the manager's establishment (handed over in person, RM-07).
func (s *StaffService) ResetPassword(ctx context.Context, actor *rbac.Actor, profileID uuid.UUID, reason string) (string, error) {
	reason, err := reasonRequired(reason)
	if err != nil {
		return "", err
	}
	_, u, err := s.linkedAccount(ctx, actor, profileID)
	if err != nil {
		return "", err
	}
	if u.Role != models.RoleProfessional {
		return "", apperr.NotFound("linked account")
	}
	return s.Auth.ResetPassword(ctx, u, reason)
}

type ChecklistItem struct {
	Key  string `json:"key"`
	Done bool   `json:"done"`
}

type ManagerOverview struct {
	Establishment      *models.Establishment
	Checklist          []ChecklistItem
	EstablishmentState []string
	Subscription       *models.Subscription
	Entitled           bool
	Profiles           []ProfileListItem
}

// Overview backs GET /manager/publication-status (§3.2).
func (s *StaffService) Overview(ctx context.Context, actor *rbac.Actor) (*ManagerOverview, error) {
	e, err := s.Repos.Establishments.ByID(ctx, actor.EstablishmentID)
	if err != nil {
		return nil, apperr.Internal(err)
	}
	st, err := s.Publication.EstablishmentState(ctx, e)
	if err != nil {
		return nil, err
	}
	out := &ManagerOverview{Establishment: e, EstablishmentState: st.Reasons(), Entitled: st.Entitled}
	if out.EstablishmentState == nil {
		out.EstablishmentState = []string{}
	}
	if sub, err := s.Repos.Billing.CurrentSubscription(ctx, e.ID); err == nil {
		out.Subscription = sub
	}
	out.Checklist = []ChecklistItem{
		{Key: "establishment_complete", Done: IsComplete(e)},
		{Key: "subscription_active", Done: st.Entitled},
	}
	all, err := s.Repos.Profiles.AllOfEstablishment(ctx, e.ID)
	if err != nil {
		return nil, apperr.Internal(err)
	}
	for i := range all {
		p := all[i]
		reasons, err := s.Publication.ProfileReasons(ctx, st, &p)
		if err != nil {
			return nil, err
		}
		item := ProfileListItem{Profile: p, Reasons: reasons}
		if p.UserID != nil {
			item.LinkedUser, _ = s.Repos.Users.ByID(ctx, *p.UserID)
		}
		if p.LastEditedBy != nil {
			item.LastEditor, _ = s.Repos.Users.ByID(ctx, *p.LastEditedBy)
		}
		out.Profiles = append(out.Profiles, item)
	}
	return out, nil
}
