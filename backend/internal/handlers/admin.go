package handlers

import (
	"github.com/gin-gonic/gin"

	"opensex/backend/internal/apperr"
	"opensex/backend/internal/dto"
	"opensex/backend/internal/models"
	"opensex/backend/internal/repositories"
	"opensex/backend/internal/services"
)

func (h *Handler) AdminStats(c *gin.Context) {
	st, err := h.Svc.Admin.KPIs(c.Request.Context())
	if err != nil {
		fail(c, err)
		return
	}
	ok(c, st)
}

type adminEstablishmentRow struct {
	dto.Establishment
	ManagerUsername    string  `json:"manager_username"`
	ManagerStatus      string  `json:"manager_status"`
	SubscriptionStatus *string `json:"subscription_status"`
	ProfilesCount      int64   `json:"profiles_count"`
	PublishedCount     int64   `json:"published_count"`
}

func (h *Handler) AdminEstablishments(c *gin.Context) {
	p := page(c)
	rows, total, err := h.Svc.Admin.ListEstablishments(c.Request.Context(), repositories.EstablishmentFilter{SubscriptionStatus: c.Query("subscription_status"), Query: c.Query("q")}, p)
	if err != nil {
		fail(c, err)
		return
	}
	items := dto.MapSlice(rows, func(r repositories.EstablishmentRow) adminEstablishmentRow {
		e := r.Establishment
		return adminEstablishmentRow{Establishment: dto.NewEstablishment(&services.EstablishmentDetail{Establishment: &e}),
			ManagerUsername: r.ManagerUsername, ManagerStatus: r.ManagerStatus, SubscriptionStatus: r.SubscriptionStatus,
			ProfilesCount: r.ProfilesCount, PublishedCount: r.PublishedCount}
	})
	ok(c, dto.Paginate(items, p, total))
}

func (h *Handler) AdminEstablishment(c *gin.Context) {
	id, valid := uuidParam(c, "id")
	if !valid {
		return
	}
	d, err := h.Svc.Admin.Establishment(c.Request.Context(), id)
	if err != nil {
		fail(c, err)
		return
	}
	ok(c, gin.H{
		"establishment":    dto.NewEstablishment(d.EstablishmentDetail),
		"manager":          dto.NewUser(d.Manager),
		"profiles":         dto.MapSlice(d.Profiles, dto.NewAdminProfile),
		"subscription":     dto.NewSubscription(d.Subscription),
		"blocking_reasons": d.Reasons,
	})
}

func (h *Handler) withReason(c *gin.Context, fn func(id [16]byte, reason string) error) {
	id, valid := uuidParam(c, "id")
	if !valid {
		return
	}
	var req dto.ReasonRequest
	if !h.bind(c, &req) {
		return
	}
	if err := fn(id, req.Reason); err != nil {
		fail(c, err)
		return
	}
	noContent(c)
}

func (h *Handler) AdminHideEstablishment(c *gin.Context) {
	h.withReason(c, func(id [16]byte, reason string) error {
		return h.Svc.Admin.HideEstablishment(c.Request.Context(), id, true, reason)
	})
}

func (h *Handler) AdminUnhideEstablishment(c *gin.Context) {
	h.withReason(c, func(id [16]byte, reason string) error {
		return h.Svc.Admin.HideEstablishment(c.Request.Context(), id, false, reason)
	})
}

func (h *Handler) AdminProfiles(c *gin.Context) {
	p := page(c)
	f := repositories.ProfileFilter{Status: c.Query("status"), Published: optBool(c, "published"), Hidden: optBool(c, "hidden"), Query: c.Query("q")}
	if v := c.Query("establishment_id"); v != "" {
		id, err := parseID(v)
		if err != nil {
			fail(c, apperr.Invalid("establishment_id", "invalid_uuid"))
			return
		}
		f.EstablishmentID = &id
	}
	rows, total, err := h.Svc.Admin.ListProfiles(c.Request.Context(), f, p)
	if err != nil {
		fail(c, err)
		return
	}
	ok(c, dto.Paginate(dto.MapSlice(rows, dto.NewAdminProfile), p, total))
}

func (h *Handler) AdminHideProfile(c *gin.Context) {
	h.withReason(c, func(id [16]byte, reason string) error {
		return h.Svc.Admin.HideProfile(c.Request.Context(), id, true, reason)
	})
}

func (h *Handler) AdminUnhideProfile(c *gin.Context) {
	h.withReason(c, func(id [16]byte, reason string) error {
		return h.Svc.Admin.HideProfile(c.Request.Context(), id, false, reason)
	})
}

func (h *Handler) AdminRemovePhoto(c *gin.Context) {
	photoID, valid := uuidParam(c, "photoId")
	if !valid {
		return
	}
	h.withReason(c, func(id [16]byte, reason string) error {
		return h.Svc.Admin.RemovePhoto(c.Request.Context(), id, photoID, reason)
	})
}

func (h *Handler) adminUsers(c *gin.Context, role string) {
	p := page(c)
	if role == "" {
		role = c.Query("role")
	}
	rows, total, err := h.Svc.Admin.ListUsers(c.Request.Context(), repositories.UserFilter{Role: role, Status: c.Query("status"), Query: c.Query("q")}, p)
	if err != nil {
		fail(c, err)
		return
	}
	ok(c, dto.Paginate(dto.MapSlice(rows, dto.NewAdminUser), p, total))
}

func (h *Handler) AdminUsers(c *gin.Context)   { h.adminUsers(c, "") }
func (h *Handler) AdminClients(c *gin.Context) { h.adminUsers(c, models.RoleClient) }

func (h *Handler) AdminUser(c *gin.Context) {
	id, valid := uuidParam(c, "id")
	if !valid {
		return
	}
	d, err := h.Svc.Admin.User(c.Request.Context(), id)
	if err != nil {
		fail(c, err)
		return
	}
	out := gin.H{"user": dto.NewAdminUser(*d.User), "active_sessions": d.Sessions}
	if d.Establishment != nil {
		out["establishment"] = dto.EstablishmentRef{ID: d.Establishment.ID, Slug: d.Establishment.Slug, DisplayName: d.Establishment.DisplayName}
	}
	if d.Profile != nil {
		out["profile"] = dto.NewAdminProfile(*d.Profile)
	}
	ok(c, out)
}

func (h *Handler) AdminSuspendUser(c *gin.Context) {
	h.withReason(c, func(id [16]byte, reason string) error { return h.Svc.Admin.Suspend(c.Request.Context(), id, reason) })
}

func (h *Handler) AdminRestoreUser(c *gin.Context) {
	id, valid := uuidParam(c, "id")
	if !valid {
		return
	}
	if err := h.Svc.Admin.Restore(c.Request.Context(), id); err != nil {
		fail(c, err)
		return
	}
	noContent(c)
}

func (h *Handler) AdminDeleteUser(c *gin.Context) {
	h.withReason(c, func(id [16]byte, reason string) error { return h.Svc.Admin.Delete(c.Request.Context(), id, reason) })
}

func (h *Handler) AdminResetPassword(c *gin.Context) {
	id, valid := uuidParam(c, "id")
	if !valid {
		return
	}
	var req dto.ReasonRequest
	if !h.bind(c, &req) {
		return
	}
	pwd, err := h.Svc.Admin.ResetPassword(c.Request.Context(), id, req.Reason)
	if err != nil {
		fail(c, err)
		return
	}
	ok(c, gin.H{"temporary_password": pwd})
}

type adminSubscription struct {
	dto.Subscription
	EstablishmentID   string `json:"establishment_id"`
	EstablishmentName string `json:"establishment_name"`
	EstablishmentSlug string `json:"establishment_slug"`
	PlanCode          string `json:"plan_code"`
}

func (h *Handler) AdminSubscriptions(c *gin.Context) {
	p := page(c)
	rows, total, err := h.Svc.Admin.ListSubscriptions(c.Request.Context(), repositories.SubscriptionFilter{Status: c.Query("status")}, p)
	if err != nil {
		fail(c, err)
		return
	}
	items := dto.MapSlice(rows, func(r repositories.SubscriptionRow) adminSubscription {
		s := r.Subscription
		return adminSubscription{Subscription: *dto.NewSubscription(&s), EstablishmentID: s.EstablishmentID.String(),
			EstablishmentName: r.EstablishmentName, EstablishmentSlug: r.EstablishmentSlug, PlanCode: r.PlanCode}
	})
	ok(c, dto.Paginate(items, p, total))
}

func (h *Handler) AdminPayments(c *gin.Context) {
	p := page(c)
	f := repositories.PaymentFilter{Status: c.Query("status")}
	if v := c.Query("establishment_id"); v != "" {
		id, err := parseID(v)
		if err != nil {
			fail(c, apperr.Invalid("establishment_id", "invalid_uuid"))
			return
		}
		f.EstablishmentID = &id
	}
	rows, total, err := h.Svc.Admin.ListPayments(c.Request.Context(), f, p)
	if err != nil {
		fail(c, err)
		return
	}
	ok(c, dto.Paginate(dto.MapSlice(rows, dto.NewAdminPayment), p, total))
}

func (h *Handler) AdminReconcilePayment(c *gin.Context) {
	id, valid := uuidParam(c, "id")
	if !valid {
		return
	}
	var req dto.ReconcileRequest
	if !h.bind(c, &req) {
		return
	}
	p, err := h.Svc.Billing.Reconcile(c.Request.Context(), id, req.Reason, req.ForcePaid)
	if err != nil {
		fail(c, err)
		return
	}
	ok(c, dto.NewAdminPayment(*p))
}

// ---------------------------------------------------------------- taxonomy

func boolOr(b *bool, def bool) bool {
	if b == nil {
		return def
	}
	return *b
}

func (h *Handler) AdminCategories(c *gin.Context) {
	rows, err := h.Svc.Repos.Taxonomy.Categories(c.Request.Context(), false)
	if err != nil {
		fail(c, apperr.Internal(err))
		return
	}
	ok(c, gin.H{"items": dto.MapSlice(rows, dto.NewCategory)})
}

func categoryInput(r dto.CategoryRequest) services.CategoryInput {
	return services.CategoryInput{ParentID: r.ParentID, Slug: r.Slug, Name: r.Name, Description: r.Description, Position: r.Position, IsActive: boolOr(r.IsActive, true)}
}

func (h *Handler) AdminCreateCategory(c *gin.Context) {
	var req dto.CategoryRequest
	if !h.bind(c, &req) {
		return
	}
	cat, err := h.Svc.Admin.CreateCategory(c.Request.Context(), categoryInput(req))
	if err != nil {
		fail(c, err)
		return
	}
	created(c, dto.NewCategory(*cat))
}

func (h *Handler) AdminUpdateCategory(c *gin.Context) {
	id, valid := uuidParam(c, "id")
	if !valid {
		return
	}
	var req dto.CategoryRequest
	if !h.bind(c, &req) {
		return
	}
	cat, err := h.Svc.Admin.UpdateCategory(c.Request.Context(), id, categoryInput(req))
	if err != nil {
		fail(c, err)
		return
	}
	ok(c, dto.NewCategory(*cat))
}

func (h *Handler) AdminDeleteCategory(c *gin.Context) {
	id, valid := uuidParam(c, "id")
	if !valid {
		return
	}
	deactivated, err := h.Svc.Admin.DeleteCategory(c.Request.Context(), id)
	if err != nil {
		fail(c, err)
		return
	}
	ok(c, gin.H{"deactivated": deactivated, "deleted": !deactivated})
}

func (h *Handler) AdminRegions(c *gin.Context) {
	rows, err := h.Svc.Repos.Taxonomy.Regions(c.Request.Context(), false)
	if err != nil {
		fail(c, apperr.Internal(err))
		return
	}
	ok(c, gin.H{"items": dto.MapSlice(rows, dto.NewRegion)})
}

func regionInput(r dto.RegionRequest) services.RegionInput {
	return services.RegionInput{ParentID: r.ParentID, Slug: r.Slug, Name: r.Name, Kind: r.Kind, IsActive: boolOr(r.IsActive, true)}
}

func (h *Handler) AdminCreateRegion(c *gin.Context) {
	var req dto.RegionRequest
	if !h.bind(c, &req) {
		return
	}
	r, err := h.Svc.Admin.CreateRegion(c.Request.Context(), regionInput(req))
	if err != nil {
		fail(c, err)
		return
	}
	created(c, dto.NewRegion(*r))
}

func (h *Handler) AdminUpdateRegion(c *gin.Context) {
	id, valid := uuidParam(c, "id")
	if !valid {
		return
	}
	var req dto.RegionRequest
	if !h.bind(c, &req) {
		return
	}
	r, err := h.Svc.Admin.UpdateRegion(c.Request.Context(), id, regionInput(req))
	if err != nil {
		fail(c, err)
		return
	}
	ok(c, dto.NewRegion(*r))
}

func (h *Handler) AdminDeleteRegion(c *gin.Context) {
	id, valid := uuidParam(c, "id")
	if !valid {
		return
	}
	deactivated, err := h.Svc.Admin.DeleteRegion(c.Request.Context(), id)
	if err != nil {
		fail(c, err)
		return
	}
	ok(c, gin.H{"deactivated": deactivated, "deleted": !deactivated})
}
