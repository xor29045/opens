// Package routes declares every endpoint together with its access rule
// (the RBAC table of §10). Registering a route without an access rule, or
// leaving a Gin route undeclared, makes the startup fail.
package routes

import (
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"sort"
	"strings"

	"github.com/gin-gonic/gin"

	"opensex/backend/internal/config"
	"opensex/backend/internal/handlers"
	"opensex/backend/internal/middlewares"
	"opensex/backend/internal/models"
	"opensex/backend/internal/services"
	"opensex/backend/pkg/ratelimit"
)

// Access is the access rule of a route.
type Access struct {
	Public   bool     // no authentication (optional auth may apply)
	Roles    []string // authenticated roles allowed
	Internal bool     // infrastructure endpoints (health, readiness)
	Cookie   bool     // refresh-cookie authenticated (CSRF protected)
	Webhook  bool     // provider callback, authenticated by signature
}

func (a Access) valid() bool {
	n := 0
	for _, b := range []bool{a.Public, len(a.Roles) > 0, a.Internal, a.Cookie, a.Webhook} {
		if b {
			n++
		}
	}
	return n == 1
}

type RouteSpec struct {
	Method string
	Path   string
	Access Access
	// PasswordChangeExempt routes stay reachable while must_change_password is set.
	PasswordChangeExempt bool
}

var (
	pub      = Access{Public: true}
	internal = Access{Internal: true}
	cookie   = Access{Cookie: true}
	webhook  = Access{Webhook: true}
	anyUser  = Access{Roles: []string{models.RoleAdmin, models.RoleManager, models.RoleProfessional, models.RoleClient}}
	nonAdmin = Access{Roles: []string{models.RoleManager, models.RoleProfessional, models.RoleClient}}
	manager  = Access{Roles: []string{models.RoleManager}}
	pro      = Access{Roles: []string{models.RoleProfessional}}
	client   = Access{Roles: []string{models.RoleClient}}
	admin    = Access{Roles: []string{models.RoleAdmin}}
)

type registry struct {
	specs map[string]RouteSpec
	list  []RouteSpec
}

func (r *registry) handle(g *gin.RouterGroup, spec RouteSpec, h ...gin.HandlerFunc) {
	if !spec.Access.valid() {
		panic(fmt.Sprintf("route %s %s has an invalid access rule", spec.Method, spec.Path))
	}
	full := strings.TrimRight(g.BasePath(), "/") + spec.Path
	if full == "" {
		full = "/"
	}
	spec.Path = full
	key := spec.Method + " " + full
	if _, dup := r.specs[key]; dup {
		panic("duplicate route " + key)
	}
	r.specs[key] = spec
	r.list = append(r.list, spec)
	g.Handle(spec.Method, strings.TrimPrefix(full, g.BasePath()), h...)
}

type Deps struct {
	Cfg     *config.Config
	Svc     *services.Services
	Logger  *slog.Logger
	Limiter *ratelimit.Limiter
	// RateLimitDisabled is only used by tests that are not about rate limiting.
	RateLimitDisabled bool
}

// New builds the Gin engine and returns the declared route table.
func New(d Deps) (*gin.Engine, []RouteSpec) {
	if !d.Cfg.IsDev() {
		gin.SetMode(gin.ReleaseMode)
	}
	e := gin.New()
	e.HandleMethodNotAllowed = false
	e.RemoteIPHeaders = []string{"X-Forwarded-For", "X-Real-IP"}
	if err := e.SetTrustedProxies(d.Cfg.TrustedProxies); err != nil {
		panic(err)
	}
	if len(d.Cfg.TrustedProxies) == 0 {
		e.ForwardedByClientIP = false
	}
	h := handlers.New(d.Svc, d.Cfg)
	auth := &middlewares.Auth{Svc: d.Svc.Auth}
	rl := &middlewares.RateLimiter{L: d.Limiter, Log: d.Logger, Enabled: !d.RateLimitDisabled}

	e.Use(
		middlewares.RequestID(d.Cfg.IPHashKey),
		middlewares.Recovery(d.Logger),
		middlewares.Logger(d.Logger),
		middlewares.SecurityHeaders(d.Cfg.CookieSecure),
		middlewares.CORS(d.Cfg.CORSAllowedOrigins),
		middlewares.BodySizeLimit(1<<20, 11<<20),
	)
	notFound := func(c *gin.Context) { middlewares.Abort(c, apperrNotFound()) }
	e.NoRoute(notFound)

	r := &registry{specs: map[string]RouteSpec{}}
	root := e.Group("")
	r.handle(root, RouteSpec{Method: http.MethodGet, Path: "/healthz", Access: internal}, h.Healthz)
	r.handle(root, RouteSpec{Method: http.MethodGet, Path: "/readyz", Access: internal}, h.Readyz)

	api := e.Group("/api/v1")
	publicRL := rl.ByIP("public", false, ratelimit.PerMinute(120))
	write := rl.ByUser("write", ratelimit.PerMinute(120))

	if d.Cfg.SwaggerOn {
		docs := api.Group("/docs")
		r.handle(docs, RouteSpec{Method: http.MethodGet, Path: "", Access: pub}, swaggerUI)
		r.handle(docs, RouteSpec{Method: http.MethodGet, Path: "/openapi.yaml", Access: pub}, openAPISpec)
	}

	// ---- authentication
	a := api.Group("/auth")
	registerRL := rl.ByIP("register", false, ratelimit.PerHour(3), ratelimit.PerDay(10))
	loginRL := rl.ByIP("login", false, ratelimit.PerMinute(20))
	adminLoginRL := rl.ByIP("login_admin", true, ratelimit.PerMinute(3), ratelimit.PerHour(10))
	r.handle(a, RouteSpec{Method: http.MethodPost, Path: "/register/client", Access: pub}, registerRL, h.RegisterClient)
	r.handle(a, RouteSpec{Method: http.MethodPost, Path: "/register/manager", Access: pub}, registerRL, h.RegisterManager)
	r.handle(a, RouteSpec{Method: http.MethodPost, Path: "/register/professional", Access: pub}, registerRL, h.RegisterProfessional)
	r.handle(a, RouteSpec{Method: http.MethodGet, Path: "/register/establishments", Access: pub}, publicRL, h.RegisterEstablishments)
	r.handle(a, RouteSpec{Method: http.MethodPost, Path: "/login", Access: pub}, loginRL, h.Login)
	r.handle(a, RouteSpec{Method: http.MethodPost, Path: "/admin/login", Access: pub},
		middlewares.AdminIPAllowlist(d.Cfg.AdminAllowedCIDRs), adminLoginRL, h.AdminLogin)
	r.handle(a, RouteSpec{Method: http.MethodPost, Path: "/password/recover", Access: pub}, rl.ByIP("recover", false, ratelimit.PerHour(10)), h.Recover)
	csrf := middlewares.CSRF(d.Cfg.CORSAllowedOrigins)
	r.handle(a, RouteSpec{Method: http.MethodPost, Path: "/refresh", Access: cookie}, csrf, rl.ByIP("refresh", false, ratelimit.PerMinute(30)), h.Refresh)
	r.handle(a, RouteSpec{Method: http.MethodPost, Path: "/logout", Access: cookie, PasswordChangeExempt: true}, csrf, auth.OptionalAuth(), h.Logout)

	self := func(acc Access, exempt bool, path, method string, hs ...gin.HandlerFunc) {
		chain := []gin.HandlerFunc{auth.RequireAuth(), middlewares.RequireRole(acc.Roles...)}
		if !exempt {
			chain = append(chain, middlewares.RequirePasswordChanged())
		}
		r.handle(a, RouteSpec{Method: method, Path: path, Access: acc, PasswordChangeExempt: exempt}, append(chain, hs...)...)
	}
	self(anyUser, true, "/me", http.MethodGet, h.Me)
	self(anyUser, true, "/password", http.MethodPut, rl.ByUser("password_change", ratelimit.PerHour(10)), h.ChangePassword)
	self(anyUser, false, "/logout-all", http.MethodPost, h.LogoutAll)
	self(anyUser, false, "/sessions", http.MethodGet, h.Sessions)
	self(anyUser, false, "/sessions/:id", http.MethodDelete, h.RevokeSession)
	self(nonAdmin, false, "/recovery-codes/confirm", http.MethodPost, h.ConfirmRecoveryCodes)
	self(nonAdmin, false, "/recovery-codes/status", http.MethodGet, h.RecoveryCodesStatus)
	self(nonAdmin, false, "/recovery-codes/regenerate", http.MethodPost, rl.ByUser("sensitive", ratelimit.PerHour(5)), h.RegenerateRecoveryCodes)

	// ---- catalogue: profiles and establishments are visible to signed-in users only
	catG := api.Group("", auth.RequireAuth(), middlewares.RequireRole(anyUser.Roles...), middlewares.RequirePasswordChanged(),
		rl.ByUser("catalog", ratelimit.PerMinute(120)))
	r.handle(catG, RouteSpec{Method: http.MethodGet, Path: "/profiles", Access: anyUser}, h.SearchProfiles)
	r.handle(catG, RouteSpec{Method: http.MethodGet, Path: "/profiles/:slug", Access: anyUser}, h.PublicProfile)
	r.handle(catG, RouteSpec{Method: http.MethodGet, Path: "/establishments/:slug", Access: anyUser}, h.PublicEstablishment)

	// ---- public reference data
	pubG := api.Group("", publicRL, auth.OptionalAuth())
	r.handle(pubG, RouteSpec{Method: http.MethodGet, Path: "/regions", Access: pub}, h.Regions)
	r.handle(pubG, RouteSpec{Method: http.MethodGet, Path: "/categories", Access: pub}, h.Categories)
	r.handle(pubG, RouteSpec{Method: http.MethodGet, Path: "/attributes", Access: pub}, h.Attributes)
	r.handle(pubG, RouteSpec{Method: http.MethodGet, Path: "/billing/plans", Access: pub}, h.Plans)

	guard := func(roles ...string) []gin.HandlerFunc {
		return []gin.HandlerFunc{auth.RequireAuth(), middlewares.RequireRole(roles...), middlewares.RequirePasswordChanged(), write}
	}

	// ---- manager space
	m := api.Group("/manager", guard(models.RoleManager)...)
	mh := func(method, path string, hs ...gin.HandlerFunc) {
		r.handle(m, RouteSpec{Method: method, Path: path, Access: manager}, hs...)
	}
	mh(http.MethodGet, "/establishment", h.GetEstablishment)
	mh(http.MethodPut, "/establishment", h.UpdateEstablishment)
	mh(http.MethodPut, "/establishment/contacts", h.ReplaceEstablishmentContacts)
	mh(http.MethodGet, "/publication-status", h.ManagerOverview)
	mh(http.MethodGet, "/stats", h.ManagerStats)
	mh(http.MethodGet, "/pending", h.PendingProfiles)
	mh(http.MethodGet, "/profiles", h.ListProfiles)
	mh(http.MethodPost, "/profiles", h.CreateProfile)
	mh(http.MethodGet, "/profiles/:profileId", h.GetProfile)
	mh(http.MethodPut, "/profiles/:profileId", h.UpdateProfile)
	mh(http.MethodDelete, "/profiles/:profileId", h.DeleteProfile)
	mh(http.MethodPost, "/profiles/:profileId/reject", h.RejectRegistration)
	mh(http.MethodPut, "/profiles/:profileId/attestation", h.Attest)
	mh(http.MethodDelete, "/profiles/:profileId/attestation", h.RemoveAttestation)
	mh(http.MethodPost, "/profiles/:profileId/activate", h.ActivateProfile)
	mh(http.MethodPost, "/profiles/:profileId/archive", h.ArchiveProfile)
	mh(http.MethodPut, "/profiles/:profileId/categories", h.ReplaceProfileCategories)
	mh(http.MethodPut, "/profiles/:profileId/rates", h.ReplaceProfileRates)
	mh(http.MethodPut, "/profiles/:profileId/contacts", h.ReplaceProfileContacts)
	mh(http.MethodPost, "/profiles/:profileId/photos", h.UploadPhoto)
	mh(http.MethodPut, "/profiles/:profileId/photos/order", h.ReorderPhotos)
	mh(http.MethodPatch, "/profiles/:profileId/photos/:photoId", h.PatchPhoto)
	mh(http.MethodDelete, "/profiles/:profileId/photos/:photoId", h.DeletePhoto)
	mh(http.MethodGet, "/profiles/:profileId/preview", h.PreviewProfile)
	mh(http.MethodPost, "/profiles/:profileId/account/suspend", h.SuspendStaff)
	mh(http.MethodPost, "/profiles/:profileId/account/restore", h.RestoreStaff)
	mh(http.MethodPost, "/profiles/:profileId/account/password-reset", h.ResetStaffPassword)

	// ---- professional space (no identifier in the URL)
	p := api.Group("/professional/me", guard(models.RoleProfessional)...)
	ph := func(method, path string, hs ...gin.HandlerFunc) {
		r.handle(p, RouteSpec{Method: method, Path: path, Access: pro}, hs...)
	}
	ph(http.MethodGet, "", h.GetProfile)
	ph(http.MethodPut, "", h.UpdateProfile)
	ph(http.MethodPut, "/categories", h.ReplaceProfileCategories)
	ph(http.MethodPut, "/rates", h.ReplaceProfileRates)
	ph(http.MethodPut, "/contacts", h.ReplaceProfileContacts)
	ph(http.MethodPost, "/photos", h.UploadPhoto)
	ph(http.MethodPut, "/photos/order", h.ReorderPhotos)
	ph(http.MethodPatch, "/photos/:photoId", h.PatchPhoto)
	ph(http.MethodDelete, "/photos/:photoId", h.DeletePhoto)
	ph(http.MethodGet, "/preview", h.PreviewProfile)
	ph(http.MethodGet, "/publication-status", h.ProfessionalPublicationStatus)

	// ---- billing (manager)
	b := api.Group("/billing", guard(models.RoleManager)...)
	bh := func(method, path string, hs ...gin.HandlerFunc) {
		r.handle(b, RouteSpec{Method: method, Path: path, Access: manager}, hs...)
	}
	bh(http.MethodGet, "/subscription", h.CurrentSubscription)
	bh(http.MethodPost, "/subscription", h.Subscribe)
	bh(http.MethodPost, "/subscription/renew", h.Renew)
	bh(http.MethodPost, "/subscription/cancel", h.CancelSubscription)
	bh(http.MethodPost, "/subscription/resume", h.ResumeSubscription)
	bh(http.MethodGet, "/payments", h.Payments)
	bh(http.MethodGet, "/payments/:id", h.Payment)
	bh(http.MethodGet, "/invoices", h.Invoices)
	bh(http.MethodGet, "/invoices/:id/pdf", h.InvoicePDF)

	// ---- client space
	cl := api.Group("/client", guard(models.RoleClient)...)
	ch := func(method, path string, hs ...gin.HandlerFunc) {
		r.handle(cl, RouteSpec{Method: method, Path: path, Access: client}, hs...)
	}
	ch(http.MethodGet, "/me", h.ClientMe)
	ch(http.MethodPut, "/me", h.ClientRename)
	ch(http.MethodDelete, "/me", h.ClientDelete)
	ch(http.MethodGet, "/favorites", h.Favorites)
	ch(http.MethodPut, "/favorites/:profileId", h.AddFavorite)
	ch(http.MethodDelete, "/favorites/:profileId", h.RemoveFavorite)
	ch(http.MethodGet, "/discover", h.Discover)
	ch(http.MethodPut, "/dislikes/:profileId", h.Dislike)
	ch(http.MethodDelete, "/dislikes/:profileId", h.Undislike)
	ch(http.MethodDelete, "/dislikes", h.ClearDislikes)

	// ---- administration
	ad := api.Group("/admin", middlewares.AdminIPAllowlist(d.Cfg.AdminAllowedCIDRs), auth.RequireAuth(), middlewares.RequireRole(models.RoleAdmin),
		middlewares.RequirePasswordChanged(), rl.ByUser("admin", ratelimit.PerMinute(300)))
	adh := func(method, path string, hs ...gin.HandlerFunc) {
		r.handle(ad, RouteSpec{Method: method, Path: path, Access: admin}, hs...)
	}
	adh(http.MethodGet, "/stats", h.AdminStats)
	adh(http.MethodGet, "/establishments", h.AdminEstablishments)
	adh(http.MethodGet, "/establishments/:id", h.AdminEstablishment)
	adh(http.MethodPost, "/establishments/:id/hide", h.AdminHideEstablishment)
	adh(http.MethodPost, "/establishments/:id/unhide", h.AdminUnhideEstablishment)
	adh(http.MethodGet, "/profiles", h.AdminProfiles)
	adh(http.MethodPost, "/profiles/:id/hide", h.AdminHideProfile)
	adh(http.MethodPost, "/profiles/:id/unhide", h.AdminUnhideProfile)
	adh(http.MethodDelete, "/profiles/:id/photos/:photoId", h.AdminRemovePhoto)
	adh(http.MethodGet, "/users", h.AdminUsers)
	adh(http.MethodGet, "/users/:id", h.AdminUser)
	adh(http.MethodGet, "/clients", h.AdminClients)
	adh(http.MethodPost, "/users/:id/suspend", h.AdminSuspendUser)
	adh(http.MethodPost, "/users/:id/restore", h.AdminRestoreUser)
	adh(http.MethodDelete, "/users/:id", h.AdminDeleteUser)
	adh(http.MethodPost, "/users/:id/password-reset", h.AdminResetPassword)
	adh(http.MethodGet, "/subscriptions", h.AdminSubscriptions)
	adh(http.MethodGet, "/payments", h.AdminPayments)
	adh(http.MethodPost, "/payments/:id/reconcile", h.AdminReconcilePayment)
	adh(http.MethodGet, "/categories", h.AdminCategories)
	adh(http.MethodPost, "/categories", h.AdminCreateCategory)
	adh(http.MethodPut, "/categories/:id", h.AdminUpdateCategory)
	adh(http.MethodDelete, "/categories/:id", h.AdminDeleteCategory)
	adh(http.MethodGet, "/regions", h.AdminRegions)
	adh(http.MethodPost, "/regions", h.AdminCreateRegion)
	adh(http.MethodPut, "/regions/:id", h.AdminUpdateRegion)
	adh(http.MethodDelete, "/regions/:id", h.AdminDeleteRegion)

	// ---- webhooks
	wh := api.Group("/webhooks")
	r.handle(wh, RouteSpec{Method: http.MethodPost, Path: "/nowpayments", Access: webhook},
		rl.ByIP("webhook", false, ratelimit.PerMinute(300)), h.NOWPaymentsWebhook)

	// Every Gin route must be declared in the access table.
	for _, rt := range e.Routes() {
		if _, ok := r.specs[rt.Method+" "+rt.Path]; !ok {
			panic("route without access declaration: " + rt.Method + " " + rt.Path)
		}
	}
	sort.Slice(r.list, func(i, j int) bool { return r.list[i].Path+r.list[i].Method < r.list[j].Path+r.list[j].Method })
	return e, r.list
}

func swaggerUI(c *gin.Context) {
	c.Header("Content-Security-Policy", "default-src 'self'; script-src 'self' https://unpkg.com 'unsafe-inline'; style-src 'self' https://unpkg.com 'unsafe-inline'; img-src 'self' data:")
	c.Data(http.StatusOK, "text/html; charset=utf-8", []byte(`<!doctype html><html><head><meta charset="utf-8"><title>API</title>
<link rel="stylesheet" href="https://unpkg.com/swagger-ui-dist@5/swagger-ui.css"></head><body><div id="ui"></div>
<script src="https://unpkg.com/swagger-ui-dist@5/swagger-ui-bundle.js"></script>
<script>SwaggerUIBundle({url: "/api/v1/docs/openapi.yaml", dom_id: "#ui"});</script></body></html>`))
}

func openAPISpec(c *gin.Context) {
	path := os.Getenv("OPENAPI_SPEC_FILE")
	if path == "" {
		path = "../api/openapi.yaml"
	}
	data, err := os.ReadFile(path)
	if err != nil {
		middlewares.Abort(c, apperrNotFound())
		return
	}
	c.Data(http.StatusOK, "application/yaml", data)
}
