package handlers

import (
	"net/http"
	"time"

	"github.com/gin-gonic/gin"

	"opensex/backend/internal/dto"
	"opensex/backend/internal/services"
)

func (h *Handler) cookieName() string {
	if h.Cfg.CookieSecure {
		return "__Host-rt"
	}
	return "rt"
}

func (h *Handler) setRefreshCookie(c *gin.Context, s *services.Session) {
	maxAge := int(time.Until(s.RefreshExpiresAt).Seconds())
	http.SetCookie(c.Writer, &http.Cookie{Name: h.cookieName(), Value: s.RefreshToken, Path: "/", MaxAge: maxAge,
		HttpOnly: true, Secure: h.Cfg.CookieSecure, SameSite: http.SameSiteStrictMode})
}

func (h *Handler) clearRefreshCookie(c *gin.Context) {
	http.SetCookie(c.Writer, &http.Cookie{Name: h.cookieName(), Value: "", Path: "/", MaxAge: -1,
		HttpOnly: true, Secure: h.Cfg.CookieSecure, SameSite: http.SameSiteStrictMode})
}

func (h *Handler) refreshCookie(c *gin.Context) string {
	v, _ := c.Cookie(h.cookieName())
	return v
}

func (h *Handler) authResult(c *gin.Context, status int, res *services.AuthResult) {
	h.setRefreshCookie(c, res.Session)
	c.JSON(status, dto.NewAuthResponse(res))
}

func (h *Handler) RegisterManager(c *gin.Context) {
	var req dto.RegisterManagerRequest
	if !h.bind(c, &req) {
		return
	}
	res, err := h.Svc.Auth.RegisterManager(c.Request.Context(), services.RegisterInput{Username: req.Username, Password: req.Password})
	if err != nil {
		fail(c, err)
		return
	}
	h.authResult(c, http.StatusCreated, res)
}

func (h *Handler) login(c *gin.Context, admin bool) {
	var req dto.LoginRequest
	if !h.bind(c, &req) {
		return
	}
	res, err := h.Svc.Auth.Login(c.Request.Context(), services.LoginInput{Username: req.Username, Password: req.Password}, admin)
	if err != nil {
		fail(c, err)
		return
	}
	h.authResult(c, http.StatusOK, res)
}

func (h *Handler) Login(c *gin.Context)      { h.login(c, false) }
func (h *Handler) AdminLogin(c *gin.Context) { h.login(c, true) }

func (h *Handler) Refresh(c *gin.Context) {
	res, err := h.Svc.Auth.Refresh(c.Request.Context(), h.refreshCookie(c))
	if err != nil {
		h.clearRefreshCookie(c)
		fail(c, err)
		return
	}
	h.authResult(c, http.StatusOK, res)
}

// Logout accepts the refresh cookie and, optionally, a (possibly expired) bearer.
func (h *Handler) Logout(c *gin.Context) {
	ctx := c.Request.Context()
	a := actor(c)
	if err := h.Svc.Auth.Logout(ctx, h.refreshCookie(c), a); err != nil {
		fail(c, err)
		return
	}
	h.clearRefreshCookie(c)
	noContent(c)
}

func (h *Handler) LogoutAll(c *gin.Context) {
	if err := h.Svc.Auth.LogoutAll(c.Request.Context(), actor(c)); err != nil {
		fail(c, err)
		return
	}
	h.clearRefreshCookie(c)
	noContent(c)
}

func (h *Handler) Me(c *gin.Context) {
	res, err := h.Svc.Auth.Me(c.Request.Context(), actor(c))
	if err != nil {
		fail(c, err)
		return
	}
	ok(c, dto.NewMe(res))
}

func (h *Handler) ChangePassword(c *gin.Context) {
	var req dto.ChangePasswordRequest
	if !h.bind(c, &req) {
		return
	}
	if err := h.Svc.Auth.ChangePassword(c.Request.Context(), actor(c), req.CurrentPassword, req.NewPassword); err != nil {
		fail(c, err)
		return
	}
	noContent(c)
}

func (h *Handler) Recover(c *gin.Context) {
	var req dto.RecoverRequest
	if !h.bind(c, &req) {
		return
	}
	if err := h.Svc.Auth.Recover(c.Request.Context(), req.Username, req.RecoveryCode, req.NewPassword); err != nil {
		fail(c, err)
		return
	}
	noContent(c)
}

func (h *Handler) ConfirmRecoveryCodes(c *gin.Context) {
	if err := h.Svc.Auth.ConfirmRecoveryCodes(c.Request.Context(), actor(c)); err != nil {
		fail(c, err)
		return
	}
	noContent(c)
}

func (h *Handler) RecoveryCodesStatus(c *gin.Context) {
	st, err := h.Svc.Auth.RecoveryCodesStatus(c.Request.Context(), actor(c))
	if err != nil {
		fail(c, err)
		return
	}
	ok(c, st)
}

func (h *Handler) RegenerateRecoveryCodes(c *gin.Context) {
	var req dto.PasswordRequest
	if !h.bind(c, &req) {
		return
	}
	codes, err := h.Svc.Auth.RegenerateRecoveryCodes(c.Request.Context(), actor(c), req.Password)
	if err != nil {
		fail(c, err)
		return
	}
	ok(c, gin.H{"recovery_codes": codes})
}

func (h *Handler) Sessions(c *gin.Context) {
	s, err := h.Svc.Auth.Sessions(c.Request.Context(), actor(c))
	if err != nil {
		fail(c, err)
		return
	}
	ok(c, gin.H{"items": s})
}

func (h *Handler) RevokeSession(c *gin.Context) {
	id, valid := uuidParam(c, "id")
	if !valid {
		return
	}
	if err := h.Svc.Auth.RevokeSession(c.Request.Context(), actor(c), id); err != nil {
		fail(c, err)
		return
	}
	noContent(c)
}
