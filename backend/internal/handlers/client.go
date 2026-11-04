package handlers

import (
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"

	"opensex/backend/internal/dto"
	"opensex/backend/internal/repositories"
)

// ---- "Découvrir" deck, anonymous: likes and passes live in the browser
// (ADR-023); the client sends the ids to leave out or to describe.

func (h *Handler) Discover(c *gin.Context) {
	var req dto.DiscoverRequest
	if !h.bind(c, &req) {
		return
	}
	fl := req.Filters
	f := repositories.CatalogFilter{City: fl.City, HairColors: upper(fl.HairColors), EyeColors: upper(fl.EyeColors),
		SkinTones: upper(fl.SkinTones), Genders: upper(fl.Genders), HeightMin: fl.HeightMin, HeightMax: fl.HeightMax,
		AgeMin: fl.AgeMin, AgeMax: fl.AgeMax, PriceMinCents: int64(fl.PriceMin) * 100, PriceMaxCents: int64(fl.PriceMax) * 100}
	res, err := h.Svc.Clients.Discover(c.Request.Context(), f, req.Exclude, req.Limit)
	if err != nil {
		fail(c, err)
		return
	}
	ok(c, res)
}

func (h *Handler) DiscoverCards(c *gin.Context) {
	var req struct {
		IDs []uuid.UUID `json:"ids"`
	}
	if !h.bind(c, &req) {
		return
	}
	items, err := h.Svc.Clients.Cards(c.Request.Context(), req.IDs)
	if err != nil {
		fail(c, err)
		return
	}
	ok(c, gin.H{"items": items})
}
