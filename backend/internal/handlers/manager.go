package handlers

import (
	"strconv"

	"github.com/gin-gonic/gin"

	"opensex/backend/internal/dto"
	"opensex/backend/internal/services"
)

func (h *Handler) GetEstablishment(c *gin.Context) {
	d, err := h.Svc.Establishments.Get(c.Request.Context(), actor(c))
	if err != nil {
		fail(c, err)
		return
	}
	ok(c, dto.NewEstablishment(d))
}

func (h *Handler) UpdateEstablishment(c *gin.Context) {
	var req dto.EstablishmentRequest
	if !h.bind(c, &req) {
		return
	}
	d, err := h.Svc.Establishments.Update(c.Request.Context(), actor(c), services.EstablishmentInput{
		DisplayName: req.DisplayName, LegalName: req.LegalName,
		Description: req.Description, OpeningHours: req.OpeningHours, RegionID: req.RegionID, CityID: req.CityID,
		Address: req.Address, AddressPublic: req.AddressPublic})
	if err != nil {
		fail(c, err)
		return
	}
	ok(c, dto.NewEstablishment(d))
}

func (h *Handler) ReplaceEstablishmentContacts(c *gin.Context) {
	var req dto.ContactsRequest
	if !h.bind(c, &req) {
		return
	}
	cs, err := h.Svc.Establishments.ReplaceContacts(c.Request.Context(), actor(c), contactInputs(req))
	if err != nil {
		fail(c, err)
		return
	}
	ok(c, gin.H{"items": dto.MapSlice(cs, dto.NewEstablishmentContact)})
}

func (h *Handler) ManagerOverview(c *gin.Context) {
	o, err := h.Svc.Staff.Overview(c.Request.Context(), actor(c))
	if err != nil {
		fail(c, err)
		return
	}
	profiles := dto.MapSlice(o.Profiles, dto.NewProfileSummary)
	ok(c, gin.H{
		"establishment":    dto.EstablishmentRef{ID: o.Establishment.ID, Slug: o.Establishment.Slug, DisplayName: o.Establishment.DisplayName},
		"checklist":        o.Checklist,
		"blocking_reasons": o.EstablishmentState,
		"subscription":     dto.NewSubscription(o.Subscription),
		"entitled":         o.Entitled,
		"profiles":         profiles,
	})
}

func (h *Handler) ManagerStats(c *gin.Context) {
	days, _ := strconv.Atoi(c.Query("days"))
	st, err := h.Svc.Stats.ForEstablishment(c.Request.Context(), actor(c), days)
	if err != nil {
		fail(c, err)
		return
	}
	ok(c, st)
}
