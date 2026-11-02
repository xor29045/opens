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
)

// AdminService implements the platform administration (§11.7). Every action
// is audited; a reason is mandatory for suspensions, deletions, hiding,
// rejections and password resets (RM-60). No action targets an ADMIN (RM-62).
type AdminService struct{ *Services }

func (s *AdminService) adminAudit(ctx context.Context, action, targetType, targetID string, meta map[string]any) {
	s.Audit.Record(ctx, AuditEvent{Action: action, Category: models.AuditAdmin, Severity: models.SeverityWarn,
		TargetType: targetType, TargetID: targetID, Metadata: meta})
}

// ---------------------------------------------------------------- stats

type DueSubscription struct {
	SubscriptionID    uuid.UUID  `json:"subscription_id"`
	EstablishmentName string     `json:"establishment_name"`
	EstablishmentSlug string     `json:"establishment_slug"`
	Status            string     `json:"status"`
	PeriodEnd         *time.Time `json:"current_period_end"`
}

type AdminStats struct {
	PublishedProfiles     int64                       `json:"published_profiles"`
	SubscriptionsByStatus map[string]int64            `json:"subscriptions_by_status"`
	MRRCents              int64                       `json:"mrr_cents"`
	UsersByRole           map[string]map[string]int64 `json:"users_by_role_and_status"`
	SecurityEvents24h     int64                       `json:"security_events_24h"`
	UpcomingDue           []DueSubscription           `json:"upcoming_due"`
}

func (s *AdminService) KPIs(ctx context.Context) (*AdminStats, error) {
	var (
		out AdminStats
		err error
	)

	if out.PublishedProfiles, err = s.Repos.Profiles.CountPublished(ctx); err != nil {
		return nil, apperr.Internal(err)
	}
	if out.SubscriptionsByStatus, err = s.Repos.Billing.CountSubscriptionsByStatus(ctx); err != nil {
		return nil, apperr.Internal(err)
	}
	if out.MRRCents, err = s.Repos.Billing.MRRCents(ctx); err != nil {
		return nil, apperr.Internal(err)
	}
	if out.UsersByRole, err = s.Repos.Users.CountByRoleStatus(ctx); err != nil {
		return nil, apperr.Internal(err)
	}
	now := s.now()
	if out.SecurityEvents24h, err = s.Repos.Audit.CountSince(ctx, models.AuditSecurity, now.Add(-24*time.Hour)); err != nil {
		return nil, apperr.Internal(err)
	}
	due, err := s.Repos.Billing.SubscriptionsEndingBefore(ctx, []string{models.SubActive, models.SubTrialing, models.SubPastDue}, now.Add(7*24*time.Hour))
	if err != nil {
		return nil, apperr.Internal(err)
	}
	out.UpcomingDue = []DueSubscription{}
	for _, sub := range due {
		e, err := s.Repos.Establishments.ByID(ctx, sub.EstablishmentID)
		if err != nil {
			continue
		}
		out.UpcomingDue = append(out.UpcomingDue, DueSubscription{SubscriptionID: sub.ID, EstablishmentName: e.DisplayName,
			EstablishmentSlug: e.Slug, Status: sub.Status, PeriodEnd: sub.CurrentPeriodEnd})
	}
	return &out, nil
}

// ---------------------------------------------------------------- establishments

func (s *AdminService) ListEstablishments(ctx context.Context, f repositories.EstablishmentFilter, p repositories.Page) ([]repositories.EstablishmentRow, int64, error) {
	rows, total, err := s.Repos.Establishments.AdminList(ctx, f, p)
	if err != nil {
		return nil, 0, apperr.Internal(err)
	}
	return rows, total, nil
}

type AdminEstablishmentDetail struct {
	*EstablishmentDetail
	Manager      *models.User
	Profiles     []models.Profile
	Subscription *models.Subscription
	History      []models.AuditLog
	Reasons      []string
}

func (s *AdminService) Establishment(ctx context.Context, id uuid.UUID) (*AdminEstablishmentDetail, error) {
	e, err := s.Repos.Establishments.ByID(ctx, id)
	if err != nil {
		if database.IsNotFound(err) {
			return nil, apperr.NotFound("establishment")
		}
		return nil, apperr.Internal(err)
	}
	d, err := s.Establishments.detail(ctx, e)
	if err != nil {
		return nil, err
	}
	out := &AdminEstablishmentDetail{EstablishmentDetail: d}
	out.Manager, _ = s.Repos.Users.ByID(ctx, e.ManagerUserID)
	if out.Profiles, err = s.Repos.Profiles.AllOfEstablishment(ctx, e.ID); err != nil {
		return nil, apperr.Internal(err)
	}
	if sub, err := s.Repos.Billing.LatestSubscription(ctx, e.ID); err == nil {
		out.Subscription = sub
	}
	if out.History, err = s.Repos.Audit.List(ctx, repositories.AuditFilter{TargetType: "establishment", TargetID: e.ID.String()}, 50); err != nil {
		return nil, apperr.Internal(err)
	}
	st, err := s.Publication.EstablishmentState(ctx, e)
	if err != nil {
		return nil, err
	}
	out.Reasons = st.Reasons()
	return out, nil
}

func (s *AdminService) mutateEstablishment(ctx context.Context, id uuid.UUID, fn func(e *models.Establishment) (map[string]any, string, map[string]any, error)) error {
	return s.tx(ctx, func(ctx context.Context) error {
		e, err := s.Repos.Establishments.ByIDForUpdate(ctx, id)
		if err != nil {
			if database.IsNotFound(err) {
				return apperr.NotFound("establishment")
			}
			return apperr.Internal(err)
		}
		cols, action, meta, err := fn(e)
		if err != nil {
			return err
		}
		if err := s.Repos.Establishments.Update(ctx, e.ID, cols); err != nil {
			return apperr.Internal(err)
		}
		s.adminAudit(ctx, action, "establishment", e.ID.String(), meta)
		return s.Publication.Recompute(ctx, e.ID)
	})
}

func (s *AdminService) HideEstablishment(ctx context.Context, id uuid.UUID, hide bool, reason string) error {
	if hide {
		var err error
		if reason, err = reasonRequired(reason); err != nil {
			return err
		}
	}
	return s.mutateEstablishment(ctx, id, func(e *models.Establishment) (map[string]any, string, map[string]any, error) {
		if hide {
			return map[string]any{"is_hidden": true, "hidden_reason": reason}, "admin.establishment.hidden", map[string]any{"reason": reason}, nil
		}
		return map[string]any{"is_hidden": false, "hidden_reason": nil}, "admin.establishment.unhidden", nil, nil
	})
}

func nilIfEmpty(s string) *string {
	if s == "" {
		return nil
	}
	return &s
}

// ---------------------------------------------------------------- profiles

func (s *AdminService) ListProfiles(ctx context.Context, f repositories.ProfileFilter, p repositories.Page) ([]models.Profile, int64, error) {
	rows, total, err := s.Repos.Profiles.List(ctx, f, p)
	if err != nil {
		return nil, 0, apperr.Internal(err)
	}
	return rows, total, nil
}

func (s *AdminService) HideProfile(ctx context.Context, id uuid.UUID, hide bool, reason string) error {
	if hide {
		var err error
		if reason, err = reasonRequired(reason); err != nil {
			return err
		}
	}
	return s.tx(ctx, func(ctx context.Context) error {
		p, err := s.Repos.Profiles.ByID(ctx, id)
		if err != nil {
			if database.IsNotFound(err) {
				return apperr.NotFound("profile")
			}
			return apperr.Internal(err)
		}
		cols := map[string]any{"is_hidden": false, "hidden_reason": nil}
		action := "admin.profile.unhidden"
		if hide {
			cols = map[string]any{"is_hidden": true, "hidden_reason": reason}
			action = "admin.profile.hidden"
		}
		if err := s.Repos.Profiles.Update(ctx, p.ID, cols); err != nil {
			return apperr.Internal(err)
		}
		s.adminAudit(ctx, action, "profile", p.ID.String(), map[string]any{"reason": reason, "establishment_id": p.EstablishmentID.String()})
		return s.Publication.Recompute(ctx, p.EstablishmentID)
	})
}

func (s *AdminService) RemovePhoto(ctx context.Context, profileID, photoID uuid.UUID, reason string) error {
	reason, err := reasonRequired(reason)
	if err != nil {
		return err
	}
	p, err := s.Repos.Profiles.ByID(ctx, profileID)
	if err != nil {
		if database.IsNotFound(err) {
			return apperr.NotFound("profile")
		}
		return apperr.Internal(err)
	}
	return s.Profiles.removePhoto(ctx, p, photoID, nil, "admin.profile.photo_removed", reason)
}

// ---------------------------------------------------------------- users

func (s *AdminService) ListUsers(ctx context.Context, f repositories.UserFilter, p repositories.Page) ([]models.User, int64, error) {
	rows, total, err := s.Repos.Users.List(ctx, f, p)
	if err != nil {
		return nil, 0, apperr.Internal(err)
	}
	return rows, total, nil
}

type AdminUserDetail struct {
	User          *models.User
	Establishment *models.Establishment
	Profile       *models.Profile
	Sessions      int
	History       []models.AuditLog
}

func (s *AdminService) User(ctx context.Context, id uuid.UUID) (*AdminUserDetail, error) {
	u, err := s.target(ctx, id)
	if err != nil {
		return nil, err
	}
	out := &AdminUserDetail{User: u}
	if e, err := s.Repos.Establishments.ByManager(ctx, u.ID); err == nil {
		out.Establishment = e
	}
	if p, err := s.Repos.Profiles.ByUser(ctx, u.ID); err == nil {
		out.Profile = p
	}
	if sess, err := s.Repos.Tokens.ActiveSessions(ctx, u.ID, s.now()); err == nil {
		out.Sessions = len(sess)
	}
	out.History, _ = s.Repos.Audit.List(ctx, repositories.AuditFilter{TargetType: "user", TargetID: u.ID.String()}, 50)
	return out, nil
}

func (s *AdminService) target(ctx context.Context, id uuid.UUID) (*models.User, error) {
	u, err := s.Repos.Users.ByID(ctx, id)
	if err != nil {
		if database.IsNotFound(err) {
			return nil, apperr.NotFound("user")
		}
		return nil, apperr.Internal(err)
	}
	return u, nil
}

func (s *AdminService) nonAdminTarget(ctx context.Context, id uuid.UUID) (*models.User, error) {
	u, err := s.target(ctx, id)
	if err != nil {
		return nil, err
	}
	if u.Role == models.RoleAdmin {
		s.Audit.Security(ctx, "admin.action_on_admin_refused", models.SeverityCritical, map[string]any{"target_id": id.String()})
		return nil, apperr.Forbidden("target_is_admin", "actions on administrator accounts are not allowed")
	}
	return u, nil
}

// establishmentStaff returns the professional accounts of a manager's establishment.
func (s *AdminService) establishmentStaff(ctx context.Context, managerID uuid.UUID) (*models.Establishment, []uuid.UUID, error) {
	e, err := s.Repos.Establishments.ByManagerUnscoped(ctx, managerID)
	if err != nil {
		if database.IsNotFound(err) {
			return nil, nil, nil
		}
		return nil, nil, apperr.Internal(err)
	}
	ids, err := s.Repos.Profiles.LinkedUserIDs(ctx, e.ID)
	if err != nil {
		return nil, nil, apperr.Internal(err)
	}
	return e, ids, nil
}

// Suspend suspends an account; suspending a manager unpublishes the whole
// establishment and revokes the sessions of every professional (RM-63).
func (s *AdminService) Suspend(ctx context.Context, id uuid.UUID, reason string) error {
	reason, err := reasonRequired(reason)
	if err != nil {
		return err
	}
	u, err := s.nonAdminTarget(ctx, id)
	if err != nil {
		return err
	}
	if u.Status == models.UserSuspended {
		return apperr.Conflict("invalid_state", "account already suspended")
	}
	var staff []uuid.UUID
	err = s.tx(ctx, func(ctx context.Context) error {
		if err := s.Repos.Users.Update(ctx, u.ID, map[string]any{"status": models.UserSuspended, "status_reason": reason}); err != nil {
			return apperr.Internal(err)
		}
		if err := s.Auth.RevokeAllSessions(ctx, u.ID, nil, models.RevokeSuspended); err != nil {
			return err
		}
		var estID *uuid.UUID
		switch u.Role {
		case models.RoleManager:
			e, ids, err := s.establishmentStaff(ctx, u.ID)
			if err != nil {
				return err
			}
			staff = ids
			for _, pid := range ids {
				if err := s.Auth.RevokeAllSessions(ctx, pid, nil, models.RevokeSuspended); err != nil {
					return err
				}
			}
			if e != nil {
				estID = &e.ID
			}
		case models.RoleProfessional:
			if p, err := s.Repos.Profiles.ByUser(ctx, u.ID); err == nil {
				estID = &p.EstablishmentID
			}
		}
		s.adminAudit(ctx, "admin.user.suspended", "user", u.ID.String(), map[string]any{"reason": reason, "role": u.Role, "staff_revoked": len(staff)})
		if estID != nil {
			return s.Publication.Recompute(ctx, *estID)
		}
		return nil
	})
	if err != nil {
		return err
	}
	_ = s.Denylist.DenyUser(ctx, u.ID)
	for _, pid := range staff {
		_ = s.Denylist.DenyUser(ctx, pid)
	}
	return nil
}

func (s *AdminService) Restore(ctx context.Context, id uuid.UUID) error {
	u, err := s.nonAdminTarget(ctx, id)
	if err != nil {
		return err
	}
	if u.Status != models.UserSuspended {
		return apperr.Conflict("invalid_state", "account is not suspended")
	}
	var staff []uuid.UUID
	err = s.tx(ctx, func(ctx context.Context) error {
		if err := s.Repos.Users.Update(ctx, u.ID, map[string]any{"status": models.UserActive, "status_reason": nil}); err != nil {
			return apperr.Internal(err)
		}
		s.adminAudit(ctx, "admin.user.restored", "user", u.ID.String(), map[string]any{"role": u.Role})
		var estID *uuid.UUID
		if u.Role == models.RoleManager {
			e, ids, err := s.establishmentStaff(ctx, u.ID)
			if err != nil {
				return err
			}
			staff = ids
			if e != nil {
				estID = &e.ID
			}
		} else if p, err := s.Repos.Profiles.ByUser(ctx, u.ID); err == nil {
			estID = &p.EstablishmentID
		}
		if estID != nil {
			return s.Publication.Recompute(ctx, *estID)
		}
		return nil
	})
	if err != nil {
		return err
	}
	_ = s.Denylist.AllowUser(ctx, u.ID)
	for _, pid := range staff {
		if pu, err := s.Repos.Users.ByID(ctx, pid); err == nil && pu.Status == models.UserActive {
			_ = s.Denylist.AllowUser(ctx, pid)
		}
	}
	return nil
}

// Delete deletes/anonymises an account. Deleting a manager deletes the
// establishment's profiles and professional accounts; the establishment and
// its billing are kept (soft delete) for the legal retention period.
func (s *AdminService) Delete(ctx context.Context, id uuid.UUID, reason string) error {
	reason, err := reasonRequired(reason)
	if err != nil {
		return err
	}
	u, err := s.nonAdminTarget(ctx, id)
	if err != nil {
		return err
	}
	var denied []uuid.UUID
	err = s.tx(ctx, func(ctx context.Context) error {
		now := s.now()
		switch u.Role {
		case models.RoleManager:
			e, ids, err := s.establishmentStaff(ctx, u.ID)
			if err != nil {
				return err
			}
			if e != nil {
				profiles, err := s.Repos.Profiles.AllOfEstablishment(ctx, e.ID)
				if err != nil {
					return apperr.Internal(err)
				}
				for _, p := range profiles {
					if err := s.Repos.Profiles.SoftDelete(ctx, p.ID, now); err != nil {
						return apperr.Internal(err)
					}
				}
				for _, pid := range ids {
					if err := s.Clients.deleteAccount(ctx, pid); err != nil {
						return err
					}
					denied = append(denied, pid)
				}
				if sub, err := s.Repos.Billing.CurrentSubscription(ctx, e.ID); err == nil {
					if err := s.Repos.Billing.UpdateSubscription(ctx, sub.ID, map[string]any{"status": models.SubCanceled, "canceled_at": now}); err != nil {
						return apperr.Internal(err)
					}
				}
				if err := s.Repos.Establishments.Update(ctx, e.ID, map[string]any{"deleted_at": now, "is_hidden": true, "hidden_reason": "deleted"}); err != nil {
					return apperr.Internal(err)
				}
			}
		case models.RoleProfessional:
			if p, err := s.Repos.Profiles.ByUser(ctx, u.ID); err == nil {
				cols := clearAttestation(rbac.ActorFrom(ctx))
				cols["status"] = models.ProfileArchived
				if err := s.Repos.Profiles.Update(ctx, p.ID, cols); err != nil {
					return apperr.Internal(err)
				}
				defer func() { _ = s.Publication.Recompute(ctx, p.EstablishmentID) }()
			}
		}
		if err := s.Clients.deleteAccount(ctx, u.ID); err != nil {
			return err
		}
		denied = append(denied, u.ID)
		s.adminAudit(ctx, "admin.user.deleted", "user", u.ID.String(), map[string]any{"reason": reason, "role": u.Role})
		return nil
	})
	if err != nil {
		return err
	}
	for _, d := range denied {
		_ = s.Denylist.DenyUser(ctx, d)
	}
	return nil
}

// ResetPassword is limited to MANAGER accounts (RM-07): professionals go
// through their manager, clients use their codes or create a new account.
func (s *AdminService) ResetPassword(ctx context.Context, id uuid.UUID, reason string) (string, error) {
	reason, err := reasonRequired(reason)
	if err != nil {
		return "", err
	}
	u, err := s.nonAdminTarget(ctx, id)
	if err != nil {
		return "", err
	}
	if u.Role != models.RoleManager {
		return "", apperr.Forbidden("manager_accounts_only", "administrators can only reset manager passwords")
	}
	return s.Auth.ResetPassword(ctx, u, reason)
}

// ---------------------------------------------------------------- billing

func (s *AdminService) ListSubscriptions(ctx context.Context, f repositories.SubscriptionFilter, p repositories.Page) ([]repositories.SubscriptionRow, int64, error) {
	rows, total, err := s.Repos.Billing.Subscriptions(ctx, f, p)
	if err != nil {
		return nil, 0, apperr.Internal(err)
	}
	return rows, total, nil
}

func (s *AdminService) ListPayments(ctx context.Context, f repositories.PaymentFilter, p repositories.Page) ([]models.Payment, int64, error) {
	rows, total, err := s.Repos.Billing.Payments(ctx, f, p)
	if err != nil {
		return nil, 0, apperr.Internal(err)
	}
	return rows, total, nil
}

// ---------------------------------------------------------------- taxonomy

type CategoryInput struct {
	ParentID    *uuid.UUID
	Slug        string
	Name        string
	Description string
	Position    int
	IsActive    bool
}

func (s *AdminService) validateCategory(ctx context.Context, in *CategoryInput, self *uuid.UUID) error {
	var c apperr.Collector
	if !validators.IsSlug(in.Slug) {
		c.Add("slug", "invalid_slug")
	}
	name, code := validators.PlainText(in.Name, 80)
	if code != "" || name == "" {
		c.Add("name", firstNonEmpty(code, "required"))
	}
	in.Name = name
	desc, code := validators.PlainText(in.Description, 500)
	if code != "" {
		c.Add("description", code)
	}
	in.Description = desc
	if in.ParentID != nil {
		parent, err := s.Repos.Taxonomy.CategoryByID(ctx, *in.ParentID)
		if err != nil || parent.ParentID != nil || (self != nil && *self == parent.ID) {
			c.Add("parent_id", "invalid_parent")
		}
	}
	return c.Err()
}

func (s *AdminService) CreateCategory(ctx context.Context, in CategoryInput) (*models.Category, error) {
	if err := s.validateCategory(ctx, &in, nil); err != nil {
		return nil, err
	}
	c := &models.Category{ID: models.NewID(), ParentID: in.ParentID, Slug: in.Slug, Name: in.Name, Description: in.Description, Position: in.Position, IsActive: in.IsActive}
	if err := s.Repos.Taxonomy.CreateCategory(ctx, c); err != nil {
		if database.IsUniqueViolation(err) {
			return nil, apperr.Conflict("slug_taken", "slug already used")
		}
		return nil, apperr.Internal(err)
	}
	s.adminAudit(ctx, "admin.category.created", "category", c.ID.String(), map[string]any{"slug": c.Slug})
	return c, nil
}

func (s *AdminService) UpdateCategory(ctx context.Context, id uuid.UUID, in CategoryInput) (*models.Category, error) {
	c, err := s.Repos.Taxonomy.CategoryByID(ctx, id)
	if err != nil {
		return nil, apperr.NotFound("category")
	}
	if err := s.validateCategory(ctx, &in, &id); err != nil {
		return nil, err
	}
	c.ParentID, c.Slug, c.Name, c.Description, c.Position, c.IsActive = in.ParentID, in.Slug, in.Name, in.Description, in.Position, in.IsActive
	if err := s.Repos.Taxonomy.SaveCategory(ctx, c); err != nil {
		if database.IsUniqueViolation(err) {
			return nil, apperr.Conflict("slug_taken", "slug already used")
		}
		return nil, apperr.Internal(err)
	}
	s.adminAudit(ctx, "admin.category.updated", "category", c.ID.String(), nil)
	return c, nil
}

// DeleteCategory deletes an unused category, or deactivates it when used.
func (s *AdminService) DeleteCategory(ctx context.Context, id uuid.UUID) (deactivated bool, err error) {
	c, err := s.Repos.Taxonomy.CategoryByID(ctx, id)
	if err != nil {
		return false, apperr.NotFound("category")
	}
	used, err := s.Repos.Taxonomy.CategoryInUse(ctx, id)
	if err != nil {
		return false, apperr.Internal(err)
	}
	if used {
		c.IsActive = false
		if err := s.Repos.Taxonomy.SaveCategory(ctx, c); err != nil {
			return false, apperr.Internal(err)
		}
		s.adminAudit(ctx, "admin.category.deactivated", "category", id.String(), nil)
		return true, nil
	}
	if err := s.Repos.Taxonomy.DeleteCategory(ctx, id); err != nil {
		return false, apperr.Internal(err)
	}
	s.adminAudit(ctx, "admin.category.deleted", "category", id.String(), nil)
	return false, nil
}

type RegionInput struct {
	ParentID *uuid.UUID
	Slug     string
	Name     string
	Kind     string
	IsActive bool
}

func (s *AdminService) validateRegion(ctx context.Context, in *RegionInput) error {
	var c apperr.Collector
	if !validators.IsSlug(in.Slug) {
		c.Add("slug", "invalid_slug")
	}
	name, code := validators.PlainText(in.Name, 80)
	if code != "" || name == "" {
		c.Add("name", firstNonEmpty(code, "required"))
	}
	in.Name = name
	switch in.Kind {
	case models.RegionKindRegion:
		if in.ParentID != nil {
			c.Add("parent_id", "must_be_null_for_region")
		}
	case models.RegionKindCity:
		if in.ParentID == nil {
			c.Add("parent_id", "required")
		} else if p, err := s.Repos.Taxonomy.RegionByID(ctx, *in.ParentID); err != nil || p.Kind != models.RegionKindRegion {
			c.Add("parent_id", "invalid_parent")
		}
	default:
		c.Add("kind", "invalid_value")
	}
	return c.Err()
}

func (s *AdminService) CreateRegion(ctx context.Context, in RegionInput) (*models.Region, error) {
	if err := s.validateRegion(ctx, &in); err != nil {
		return nil, err
	}
	r := &models.Region{ID: models.NewID(), CountryCode: "BE", ParentID: in.ParentID, Slug: in.Slug, Name: in.Name, Kind: in.Kind, IsActive: in.IsActive}
	if err := s.Repos.Taxonomy.CreateRegion(ctx, r); err != nil {
		if database.IsUniqueViolation(err) {
			return nil, apperr.Conflict("slug_taken", "slug already used")
		}
		return nil, apperr.Internal(err)
	}
	s.adminAudit(ctx, "admin.region.created", "region", r.ID.String(), map[string]any{"slug": r.Slug})
	return r, nil
}

func (s *AdminService) UpdateRegion(ctx context.Context, id uuid.UUID, in RegionInput) (*models.Region, error) {
	r, err := s.Repos.Taxonomy.RegionByID(ctx, id)
	if err != nil {
		return nil, apperr.NotFound("region")
	}
	if err := s.validateRegion(ctx, &in); err != nil {
		return nil, err
	}
	if in.Kind != r.Kind {
		return nil, apperr.Invalid("kind", "immutable")
	}
	r.ParentID, r.Slug, r.Name, r.IsActive = in.ParentID, in.Slug, in.Name, in.IsActive
	if err := s.Repos.Taxonomy.SaveRegion(ctx, r); err != nil {
		if database.IsUniqueViolation(err) {
			return nil, apperr.Conflict("slug_taken", "slug already used")
		}
		return nil, apperr.Internal(err)
	}
	s.adminAudit(ctx, "admin.region.updated", "region", r.ID.String(), nil)
	return r, nil
}

func (s *AdminService) DeleteRegion(ctx context.Context, id uuid.UUID) (bool, error) {
	r, err := s.Repos.Taxonomy.RegionByID(ctx, id)
	if err != nil {
		return false, apperr.NotFound("region")
	}
	used, err := s.Repos.Taxonomy.RegionInUse(ctx, id)
	if err != nil {
		return false, apperr.Internal(err)
	}
	if used {
		r.IsActive = false
		if err := s.Repos.Taxonomy.SaveRegion(ctx, r); err != nil {
			return false, apperr.Internal(err)
		}
		s.adminAudit(ctx, "admin.region.deactivated", "region", id.String(), nil)
		return true, nil
	}
	if err := s.Repos.Taxonomy.DeleteRegion(ctx, id); err != nil {
		return false, apperr.Internal(err)
	}
	s.adminAudit(ctx, "admin.region.deleted", "region", id.String(), nil)
	return false, nil
}

// ---------------------------------------------------------------- audit

func (s *AdminService) AuditLogs(ctx context.Context, f repositories.AuditFilter, limit int) ([]models.AuditLog, error) {
	if limit < 1 || limit > 200 {
		limit = 100
	}
	rows, err := s.Repos.Audit.List(ctx, f, limit)
	if err != nil {
		return nil, apperr.Internal(err)
	}
	return rows, nil
}

const maxExportRows = 10000

// ExportAudit returns up to 10 000 entries; the export itself is audited.
func (s *AdminService) ExportAudit(ctx context.Context, f repositories.AuditFilter) ([]models.AuditLog, error) {
	rows, err := s.Repos.Audit.List(ctx, f, maxExportRows)
	if err != nil {
		return nil, apperr.Internal(err)
	}
	s.adminAudit(ctx, "admin.audit.exported", "audit_logs", "", map[string]any{"rows": len(rows), "category": f.Category})
	return rows, nil
}
