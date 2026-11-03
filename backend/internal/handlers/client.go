package handlers

import (
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"

	"opensex/backend/internal/dto"
	"opensex/backend/internal/repositories"
	"opensex/backend/internal/services"
)

func (h *Handler) ClientMe(c *gin.Context) {
	u, err := h.Svc.Clients.Me(c.Request.Context(), actor(c))
	if err != nil {
		fail(c, err)
		return
	}
	ok(c, dto.NewUser(u))
}

func (h *Handler) ClientDelete(c *gin.Context) {
	var req dto.PasswordRequest
	if !h.bind(c, &req) {
		return
	}
	if err := h.Svc.Clients.DeleteAccount(c.Request.Context(), actor(c), req.Password); err != nil {
		fail(c, err)
		return
	}
	h.clearRefreshCookie(c)
	noContent(c)
}

type favoriteItem struct {
	ProfileID      uuid.UUID `json:"profile_id"`
	Slug           string    `json:"slug"`
	DisplayName    string    `json:"display_name"`
	Headline       string    `json:"headline"`
	Available      bool      `json:"available"`
	Age            *int      `json:"age"`
	PriceFromCents *int64    `json:"price_from_cents"`
	Establishment  string    `json:"establishment_name"`
	City           *string   `json:"city"`
	AddedAt        time.Time `json:"added_at"`
}

func (h *Handler) Favorites(c *gin.Context) {
	p := page(c)
	rows, total, err := h.Svc.Clients.Favorites(c.Request.Context(), actor(c), p)
	if err != nil {
		fail(c, err)
		return
	}
	now := h.now()
	items := dto.MapSlice(rows, func(r repositories.FavoriteRow) favoriteItem {
		it := favoriteItem{ProfileID: r.ProfileID, Slug: r.Slug, DisplayName: r.DisplayName, Available: r.IsPublished, AddedAt: r.CreatedAt}
		if r.IsPublished { // an unpublished favourite only shows as "unavailable" (RM-50)
			it.Headline, it.PriceFromCents, it.Establishment, it.City = r.Headline, r.PriceFromCents, r.EstablishmentName, r.CityName
			if r.BirthDateAttested != nil {
				a := services.AgeAt(*r.BirthDateAttested, now)
				it.Age = &a
			}
		}
		return it
	})
	ok(c, dto.Paginate(items, p, total))
}

func (h *Handler) AddFavorite(c *gin.Context) {
	id, valid := uuidParam(c, "profileId")
	if !valid {
		return
	}
	if err := h.Svc.Clients.AddFavorite(c.Request.Context(), actor(c), id); err != nil {
		fail(c, err)
		return
	}
	noContent(c)
}

func (h *Handler) RemoveFavorite(c *gin.Context) {
	id, valid := uuidParam(c, "profileId")
	if !valid {
		return
	}
	if err := h.Svc.Clients.RemoveFavorite(c.Request.Context(), actor(c), id); err != nil {
		fail(c, err)
		return
	}
	noContent(c)
}

func (h *Handler) ClientRename(c *gin.Context) {
	var req struct {
		Username string `json:"username"`
	}
	if !h.bind(c, &req) {
		return
	}
	u, err := h.Svc.Clients.Rename(c.Request.Context(), actor(c), req.Username)
	if err != nil {
		fail(c, err)
		return
	}
	ok(c, dto.NewUser(u))
}

// ---- "Découvrir" deck (like = favorite, dislike = hidden from the deck)

func (h *Handler) Discover(c *gin.Context) {
	limit, _ := strconv.Atoi(c.Query("limit"))
	res, err := h.Svc.Clients.Discover(c.Request.Context(), actor(c), limit)
	if err != nil {
		fail(c, err)
		return
	}
	ok(c, res)
}

func (h *Handler) Dislike(c *gin.Context) {
	id, valid := uuidParam(c, "profileId")
	if !valid {
		return
	}
	if err := h.Svc.Clients.Dislike(c.Request.Context(), actor(c), id); err != nil {
		fail(c, err)
		return
	}
	noContent(c)
}

func (h *Handler) Undislike(c *gin.Context) {
	id, valid := uuidParam(c, "profileId")
	if !valid {
		return
	}
	if err := h.Svc.Clients.Undislike(c.Request.Context(), actor(c), id); err != nil {
		fail(c, err)
		return
	}
	noContent(c)
}

func (h *Handler) ClearDislikes(c *gin.Context) {
	n, err := h.Svc.Clients.ClearDislikes(c.Request.Context(), actor(c))
	if err != nil {
		fail(c, err)
		return
	}
	ok(c, gin.H{"restored": n})
}
