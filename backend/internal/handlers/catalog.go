package handlers

import (
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"

	"opensex/backend/internal/apperr"
	"opensex/backend/internal/dto"
	"opensex/backend/internal/repositories"
	"opensex/backend/internal/services"
	"opensex/backend/internal/validators"
)

// multi reads repeated (?x=a&x=b), bracketed (?x[]=a) or comma separated values.
func multi(c *gin.Context, name string) []string {
	var out []string
	for _, v := range append(c.QueryArray(name), c.QueryArray(name+"[]")...) {
		for _, p := range strings.Split(v, ",") {
			if p = strings.TrimSpace(p); p != "" {
				out = append(out, p)
			}
		}
	}
	return out
}

func intQuery(c *gin.Context, name string, errs *apperr.Collector) int {
	v := c.Query(name)
	if v == "" {
		return 0
	}
	n, err := strconv.Atoi(v)
	if err != nil || n < 0 {
		errs.Add(name, "invalid_integer")
	}
	return n
}

func upper(in []string) []string {
	for i := range in {
		in[i] = strings.ToUpper(in[i])
	}
	return in
}

func (h *Handler) SearchProfiles(c *gin.Context) {
	var errs apperr.Collector
	f := repositories.CatalogFilter{
		Region: c.Query("region"), City: c.Query("city"), Establishment: c.Query("establishment"),
		Categories: multi(c, "categories"),
		AgeMin:     intQuery(c, "age_min", &errs), AgeMax: intQuery(c, "age_max", &errs),
		PriceMinCents: int64(intQuery(c, "price_min", &errs)) * 100, PriceMaxCents: int64(intQuery(c, "price_max", &errs)) * 100,
		HeightMin: intQuery(c, "height_min", &errs), HeightMax: intQuery(c, "height_max", &errs),
		HairColors: upper(multi(c, "hair_color")), EyeColors: upper(multi(c, "eye_color")), BodyTypes: upper(multi(c, "body_type")),
		Languages: multi(c, "languages"), Sort: c.Query("sort"),
	}
	for name, v := range map[string]string{"region": f.Region, "city": f.City, "establishment": f.Establishment} {
		if v != "" && !validators.IsSlug(v) {
			errs.Add(name, "invalid_slug")
		}
	}
	for _, v := range f.Categories {
		if !validators.IsSlug(v) {
			errs.Add("categories", "invalid_slug")
		}
	}
	if err := errs.Err(); err != nil {
		fail(c, err)
		return
	}
	p := page(c)
	items, total, err := h.Svc.Catalog.Search(c.Request.Context(), services.SearchParams{CatalogFilter: f, Page: p})
	if err != nil {
		fail(c, err)
		return
	}
	ok(c, dto.Paginate(items, p, total))
}

func (h *Handler) PublicProfile(c *gin.Context) {
	slug := c.Param("slug")
	if !validators.IsSlug(slug) {
		fail(c, apperr.NotFound("profile"))
		return
	}
	p, err := h.Svc.Catalog.ProfileBySlug(c.Request.Context(), slug)
	if err != nil {
		fail(c, err)
		return
	}
	c.Header("Referrer-Policy", "no-referrer")
	ok(c, p)
}

func (h *Handler) PublicEstablishment(c *gin.Context) {
	slug := c.Param("slug")
	if !validators.IsSlug(slug) {
		fail(c, apperr.NotFound("establishment"))
		return
	}
	e, err := h.Svc.Catalog.EstablishmentBySlug(c.Request.Context(), slug)
	if err != nil {
		fail(c, err)
		return
	}
	c.Header("Referrer-Policy", "no-referrer")
	ok(c, e)
}

func (h *Handler) Regions(c *gin.Context) {
	r, err := h.Svc.Catalog.Regions(c.Request.Context())
	if err != nil {
		fail(c, err)
		return
	}
	ok(c, gin.H{"items": dto.MapSlice(r, dto.NewRegion)})
}

func (h *Handler) Categories(c *gin.Context) {
	r, err := h.Svc.Catalog.Categories(c.Request.Context())
	if err != nil {
		fail(c, err)
		return
	}
	ok(c, gin.H{"items": dto.MapSlice(r, dto.NewCategory)})
}

func (h *Handler) Attributes(c *gin.Context) { ok(c, h.Svc.Catalog.Attributes()) }
