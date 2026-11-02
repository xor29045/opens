package handlers

import (
	"io"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"

	"opensex/backend/internal/apperr"
	"opensex/backend/internal/dto"
	"opensex/backend/internal/models"
	"opensex/backend/internal/rbac"
	"opensex/backend/internal/repositories"
	"opensex/backend/internal/services"
)

// Profile routes are shared by the manager space (/manager/profiles/:profileId/...)
// and the professional space (/professional/me/...). For professionals the
// identifier is never read from the URL: the service uses the actor's profile.
func profileID(c *gin.Context) (uuid.UUID, bool) {
	a := actor(c)
	if a.Is(models.RoleProfessional) {
		return a.ProfileID, true
	}
	return uuidParam(c, "profileId")
}

func isManager(a *rbac.Actor) bool { return a.Is(models.RoleManager) }

func (h *Handler) renderProfile(c *gin.Context, status int, d *services.ProfileDetail) {
	c.JSON(status, dto.NewProfile(d, isManager(actor(c)), h.now()))
}

func profileInput(r dto.ProfileRequest) services.ProfileInput {
	return services.ProfileInput{DisplayName: r.DisplayName, Headline: r.Headline, Description: r.Description, HeightCm: r.HeightCm,
		WeightKg: r.WeightKg, HairColor: r.HairColor, EyeColor: r.EyeColor, BodyType: r.BodyType, Languages: r.Languages,
		ExtraAttributes: r.ExtraAttributes}
}

func (h *Handler) GetProfile(c *gin.Context) {
	id, valid := profileID(c)
	if !valid {
		return
	}
	d, err := h.Svc.Profiles.Get(c.Request.Context(), actor(c), id)
	if err != nil {
		fail(c, err)
		return
	}
	h.renderProfile(c, http.StatusOK, d)
}

func (h *Handler) UpdateProfile(c *gin.Context) {
	id, valid := profileID(c)
	if !valid {
		return
	}
	var req dto.ProfileRequest
	if !h.bind(c, &req) {
		return
	}
	d, err := h.Svc.Profiles.Update(c.Request.Context(), actor(c), id, profileInput(req))
	if err != nil {
		fail(c, err)
		return
	}
	h.renderProfile(c, http.StatusOK, d)
}

func (h *Handler) CreateProfile(c *gin.Context) {
	var req dto.ProfileRequest
	if !h.bind(c, &req) {
		return
	}
	d, err := h.Svc.Profiles.CreateByManager(c.Request.Context(), actor(c), profileInput(req))
	if err != nil {
		fail(c, err)
		return
	}
	h.renderProfile(c, http.StatusCreated, d)
}

func (h *Handler) ListProfiles(c *gin.Context) {
	p := page(c)
	f := repositories.ProfileFilter{Status: c.Query("status"), Published: optBool(c, "published"), Linked: optBool(c, "linked"), Query: c.Query("q")}
	items, total, err := h.Svc.Profiles.List(c.Request.Context(), actor(c), f, p)
	if err != nil {
		fail(c, err)
		return
	}
	ok(c, dto.Paginate(dto.MapSlice(items, dto.NewProfileSummary), p, total))
}

func (h *Handler) PendingProfiles(c *gin.Context) {
	p := page(c)
	items, total, err := h.Svc.Profiles.List(c.Request.Context(), actor(c), repositories.ProfileFilter{Status: models.ProfilePendingApproval}, p)
	if err != nil {
		fail(c, err)
		return
	}
	ok(c, dto.Paginate(dto.MapSlice(items, dto.NewProfileSummary), p, total))
}

func (h *Handler) DeleteProfile(c *gin.Context) {
	id, valid := uuidParam(c, "profileId")
	if !valid {
		return
	}
	if err := h.Svc.Profiles.Delete(c.Request.Context(), actor(c), id); err != nil {
		fail(c, err)
		return
	}
	noContent(c)
}

func (h *Handler) ReplaceProfileCategories(c *gin.Context) {
	id, valid := profileID(c)
	if !valid {
		return
	}
	var req dto.CategoriesRequest
	if !h.bind(c, &req) {
		return
	}
	cats, err := h.Svc.Profiles.ReplaceCategories(c.Request.Context(), actor(c), id, req.CategoryIDs)
	if err != nil {
		fail(c, err)
		return
	}
	ok(c, gin.H{"items": dto.MapSlice(cats, dto.NewCategory)})
}

func (h *Handler) ReplaceProfileRates(c *gin.Context) {
	id, valid := profileID(c)
	if !valid {
		return
	}
	var req dto.RatesRequest
	if !h.bind(c, &req) {
		return
	}
	in := make([]services.RateInput, len(req.Rates))
	for i, r := range req.Rates {
		in[i] = services.RateInput{Label: r.Label, DurationMinutes: r.DurationMinutes, AmountCents: r.AmountCents}
	}
	rates, err := h.Svc.Profiles.ReplaceRates(c.Request.Context(), actor(c), id, in)
	if err != nil {
		fail(c, err)
		return
	}
	ok(c, gin.H{"items": dto.MapSlice(rates, dto.NewRate)})
}

func contactInputs(req dto.ContactsRequest) []services.ContactInput {
	in := make([]services.ContactInput, len(req.Contacts))
	for i, ct := range req.Contacts {
		in[i] = services.ContactInput{Type: ct.Type, Value: ct.Value, Visibility: ct.Visibility, IsVerificationPhone: ct.IsVerificationPhone}
	}
	return in
}

func (h *Handler) ReplaceProfileContacts(c *gin.Context) {
	id, valid := profileID(c)
	if !valid {
		return
	}
	var req dto.ContactsRequest
	if !h.bind(c, &req) {
		return
	}
	cs, err := h.Svc.Profiles.ReplaceContacts(c.Request.Context(), actor(c), id, contactInputs(req))
	if err != nil {
		fail(c, err)
		return
	}
	ok(c, gin.H{"items": dto.MapSlice(cs, dto.NewProfileContact)})
}

const maxUploadBytes = 10 << 20

func (h *Handler) UploadPhoto(c *gin.Context) {
	id, valid := profileID(c)
	if !valid {
		return
	}
	fh, err := c.FormFile("file")
	if err != nil {
		fail(c, apperr.Invalid("file", "required"))
		return
	}
	if fh.Size > maxUploadBytes {
		fail(c, apperr.Invalid("file", "file_too_large"))
		return
	}
	f, err := fh.Open()
	if err != nil {
		fail(c, apperr.Invalid("file", "unreadable"))
		return
	}
	defer f.Close()
	data, err := io.ReadAll(io.LimitReader(f, maxUploadBytes+1))
	if err != nil || len(data) > maxUploadBytes {
		fail(c, apperr.Invalid("file", "file_too_large"))
		return
	}
	ph, err := h.Svc.Profiles.UploadPhoto(c.Request.Context(), actor(c), id, data)
	if err != nil {
		fail(c, err)
		return
	}
	created(c, dto.NewPhoto(*ph))
}

func (h *Handler) PatchPhoto(c *gin.Context) {
	id, valid := profileID(c)
	if !valid {
		return
	}
	photoID, valid := uuidParam(c, "photoId")
	if !valid {
		return
	}
	var req dto.PhotoPatchRequest
	if !h.bind(c, &req) {
		return
	}
	photos, err := h.Svc.Profiles.UpdatePhoto(c.Request.Context(), actor(c), id, photoID, services.PhotoPatch{IsPrimary: req.IsPrimary, Position: req.Position})
	if err != nil {
		fail(c, err)
		return
	}
	ok(c, gin.H{"items": dto.MapSlice(photos, dto.NewPhoto)})
}

func (h *Handler) DeletePhoto(c *gin.Context) {
	id, valid := profileID(c)
	if !valid {
		return
	}
	photoID, valid := uuidParam(c, "photoId")
	if !valid {
		return
	}
	if err := h.Svc.Profiles.DeletePhoto(c.Request.Context(), actor(c), id, photoID); err != nil {
		fail(c, err)
		return
	}
	noContent(c)
}

func (h *Handler) ReorderPhotos(c *gin.Context) {
	id, valid := profileID(c)
	if !valid {
		return
	}
	var req dto.PhotoOrderRequest
	if !h.bind(c, &req) {
		return
	}
	photos, err := h.Svc.Profiles.ReorderPhotos(c.Request.Context(), actor(c), id, req.PhotoIDs)
	if err != nil {
		fail(c, err)
		return
	}
	ok(c, gin.H{"items": dto.MapSlice(photos, dto.NewPhoto)})
}

func (h *Handler) PreviewProfile(c *gin.Context) {
	id, valid := profileID(c)
	if !valid {
		return
	}
	p, err := h.Svc.Profiles.Preview(c.Request.Context(), actor(c), id)
	if err != nil {
		fail(c, err)
		return
	}
	ok(c, p)
}

func (h *Handler) ProfessionalPublicationStatus(c *gin.Context) {
	st, err := h.Svc.Profiles.PublicationStatus(c.Request.Context(), actor(c))
	if err != nil {
		fail(c, err)
		return
	}
	ok(c, st)
}

// ---------------------------------------------------------------- manager-only lifecycle

func (h *Handler) Attest(c *gin.Context) {
	id, valid := uuidParam(c, "profileId")
	if !valid {
		return
	}
	var req dto.AttestationRequest
	if !h.bind(c, &req) {
		return
	}
	birth, err := time.Parse("2006-01-02", req.BirthDate)
	if err != nil {
		fail(c, apperr.Invalid("birth_date", "invalid_date"))
		return
	}
	d, err := h.Svc.Profiles.Attest(c.Request.Context(), actor(c), id, birth, req.EmploymentReference, req.Attested)
	if err != nil {
		fail(c, err)
		return
	}
	h.renderProfile(c, http.StatusOK, d)
}

func (h *Handler) RemoveAttestation(c *gin.Context) {
	id, valid := uuidParam(c, "profileId")
	if !valid {
		return
	}
	if err := h.Svc.Profiles.RemoveAttestation(c.Request.Context(), actor(c), id); err != nil {
		fail(c, err)
		return
	}
	noContent(c)
}

func (h *Handler) ActivateProfile(c *gin.Context) {
	id, valid := uuidParam(c, "profileId")
	if !valid {
		return
	}
	d, err := h.Svc.Profiles.Activate(c.Request.Context(), actor(c), id)
	if err != nil {
		fail(c, err)
		return
	}
	h.renderProfile(c, http.StatusOK, d)
}

func (h *Handler) ArchiveProfile(c *gin.Context) {
	id, valid := uuidParam(c, "profileId")
	if !valid {
		return
	}
	if err := h.Svc.Profiles.Archive(c.Request.Context(), actor(c), id); err != nil {
		fail(c, err)
		return
	}
	noContent(c)
}

func (h *Handler) RejectRegistration(c *gin.Context) {
	id, valid := uuidParam(c, "profileId")
	if !valid {
		return
	}
	var req dto.ReasonRequest
	if !h.bind(c, &req) {
		return
	}
	if err := h.Svc.Staff.Reject(c.Request.Context(), actor(c), id, req.Reason); err != nil {
		fail(c, err)
		return
	}
	noContent(c)
}

func (h *Handler) SuspendStaff(c *gin.Context) {
	id, valid := uuidParam(c, "profileId")
	if !valid {
		return
	}
	var req dto.ReasonRequest
	if !h.bind(c, &req) {
		return
	}
	if err := h.Svc.Staff.SuspendAccount(c.Request.Context(), actor(c), id, req.Reason); err != nil {
		fail(c, err)
		return
	}
	noContent(c)
}

func (h *Handler) RestoreStaff(c *gin.Context) {
	id, valid := uuidParam(c, "profileId")
	if !valid {
		return
	}
	if err := h.Svc.Staff.RestoreAccount(c.Request.Context(), actor(c), id); err != nil {
		fail(c, err)
		return
	}
	noContent(c)
}

func (h *Handler) ResetStaffPassword(c *gin.Context) {
	id, valid := uuidParam(c, "profileId")
	if !valid {
		return
	}
	var req dto.ReasonRequest
	if !h.bind(c, &req) {
		return
	}
	pwd, err := h.Svc.Staff.ResetPassword(c.Request.Context(), actor(c), id, req.Reason)
	if err != nil {
		fail(c, err)
		return
	}
	ok(c, gin.H{"temporary_password": pwd})
}
