package middlewares

import (
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"

	"opensex/backend/internal/apperr"
	"opensex/backend/internal/models"
	"opensex/backend/internal/rbac"
	"opensex/backend/internal/services"
)

const ctxUser = "auth.user"

type Auth struct {
	Svc *services.AuthService
}

func bearer(c *gin.Context) string {
	h := c.GetHeader("Authorization")
	if len(h) > 7 && strings.EqualFold(h[:7], "bearer ") {
		return strings.TrimSpace(h[7:])
	}
	return ""
}

func isMutation(m string) bool {
	return m != http.MethodGet && m != http.MethodHead && m != http.MethodOptions
}

func (a *Auth) authenticate(c *gin.Context) error {
	tok := bearer(c)
	if tok == "" {
		return apperr.Unauthorized("missing_token")
	}
	actor, user, err := a.Svc.Authenticate(c.Request.Context(), tok, isMutation(c.Request.Method))
	if err != nil {
		return err
	}
	c.Request = c.Request.WithContext(rbac.WithActor(c.Request.Context(), actor))
	c.Set(ctxUser, user)
	return nil
}

// RequireAuth rejects requests without a valid access token (401). It also
// enforces RequireAccountActive: suspended, deleted or orphaned accounts are
// rejected with 401 (§10.2).
func (a *Auth) RequireAuth() gin.HandlerFunc {
	return func(c *gin.Context) {
		if err := a.authenticate(c); err != nil {
			Abort(c, err)
			return
		}
		c.Next()
	}
}

// OptionalAuth attaches the actor when a valid token is present; an invalid
// token is treated as anonymous on public routes.
func (a *Auth) OptionalAuth() gin.HandlerFunc {
	return func(c *gin.Context) {
		if bearer(c) != "" {
			_ = a.authenticate(c)
		}
		c.Next()
	}
}

// RequireRole rejects authenticated actors of another role (403).
func RequireRole(roles ...string) gin.HandlerFunc {
	set := map[string]bool{}
	for _, r := range roles {
		set[r] = true
	}
	return func(c *gin.Context) {
		a := rbac.ActorFrom(c.Request.Context())
		if a == nil {
			Abort(c, apperr.Unauthorized("missing_token"))
			return
		}
		if !set[a.Role] {
			Abort(c, apperr.Forbidden("forbidden", "insufficient role"))
			return
		}
		c.Next()
	}
}

// RequirePasswordChanged blocks accounts that must change their password
// (RM-02c) except on the routes that allow the change.
func RequirePasswordChanged() gin.HandlerFunc {
	return func(c *gin.Context) {
		if u, ok := c.Get(ctxUser); ok {
			if user, _ := u.(*models.User); user != nil && user.MustChangePassword {
				Abort(c, apperr.Forbidden("password_change_required", "the password must be changed first"))
				return
			}
		}
		c.Next()
	}
}

// CurrentUser returns the authenticated user loaded by RequireAuth.
func CurrentUser(c *gin.Context) *models.User {
	u, _ := c.Get(ctxUser)
	user, _ := u.(*models.User)
	return user
}
