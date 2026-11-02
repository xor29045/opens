// Package middlewares contains the HTTP pipeline (§5.3). Middlewares never
// access repositories directly; they use services.
package middlewares

import (
	"context"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"runtime/debug"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"

	"opensex/backend/internal/apperr"
	"opensex/backend/internal/rbac"
	"opensex/backend/pkg/crypto"
	"opensex/backend/pkg/problem"
	"opensex/backend/pkg/ratelimit"
)

const HeaderRequestID = "X-Request-ID"

// RequestID assigns a request id (a client-provided id is accepted only if
// it is a short safe token) and stores request metadata in the context.
func RequestID(ipHashKey []byte) gin.HandlerFunc {
	return func(c *gin.Context) {
		id := c.GetHeader(HeaderRequestID)
		if len(id) == 0 || len(id) > 64 || strings.ContainsFunc(id, func(r rune) bool {
			return !(r >= 'a' && r <= 'z' || r >= 'A' && r <= 'Z' || r >= '0' && r <= '9' || r == '-' || r == '_')
		}) {
			id = uuid.NewString()
		}
		c.Header(HeaderRequestID, id)
		ip := c.ClientIP()
		meta := rbac.RequestMeta{RequestID: id, IP: ip, IPHash: crypto.HMACSHA256Hex(ipHashKey, ip), UserAgent: c.Request.UserAgent()}
		c.Request = c.Request.WithContext(rbac.WithMeta(c.Request.Context(), meta))
		c.Next()
	}
}

// Recovery turns panics into a generic 500 Problem (no stack in the response).
func Recovery(log *slog.Logger) gin.HandlerFunc {
	return func(c *gin.Context) {
		defer func() {
			if r := recover(); r != nil {
				log.ErrorContext(c.Request.Context(), "panic", "error", fmt.Sprint(r), "stack", string(debug.Stack()),
					"request_id", rbac.MetaFrom(c.Request.Context()).RequestID)
				Abort(c, apperr.Internal(fmt.Errorf("panic: %v", r)))
			}
		}()
		c.Next()
	}
}

// Logger writes one structured line per request; bodies, query strings and
// sensitive headers are never logged.
func Logger(log *slog.Logger) gin.HandlerFunc {
	return func(c *gin.Context) {
		start := time.Now()
		c.Next()
		meta := rbac.MetaFrom(c.Request.Context())
		attrs := []any{"request_id", meta.RequestID, "method", c.Request.Method, "path", c.FullPath(),
			"status", c.Writer.Status(), "duration_ms", time.Since(start).Milliseconds(), "ip_hash", meta.IPHash}
		if a := rbac.ActorFrom(c.Request.Context()); a != nil {
			attrs = append(attrs, "user_id", a.UserID.String(), "role", a.Role)
		}
		lvl := slog.LevelInfo
		if c.Writer.Status() >= 500 {
			lvl = slog.LevelError
		} else if c.FullPath() == "/healthz" || c.FullPath() == "/readyz" {
			lvl = slog.LevelDebug
		}
		log.Log(c.Request.Context(), lvl, "http_request", attrs...)
	}
}

// SecurityHeaders sets the API response headers (§13.2).
func SecurityHeaders(hsts bool) gin.HandlerFunc {
	return func(c *gin.Context) {
		h := c.Writer.Header()
		h.Set("X-Content-Type-Options", "nosniff")
		h.Set("X-Frame-Options", "DENY")
		h.Set("Referrer-Policy", "strict-origin-when-cross-origin")
		h.Set("Cross-Origin-Opener-Policy", "same-origin")
		h.Set("Cross-Origin-Resource-Policy", "same-origin")
		h.Set("Permissions-Policy", "camera=(), microphone=(), geolocation=(), payment=()")
		if !strings.HasPrefix(c.Request.URL.Path, "/api/v1/docs") {
			h.Set("Content-Security-Policy", "default-src 'none'; frame-ancestors 'none'; base-uri 'none'; form-action 'none'")
		}
		if hsts {
			h.Set("Strict-Transport-Security", "max-age=63072000; includeSubDomains; preload")
		}
		if strings.HasPrefix(c.Request.URL.Path, "/api/v1/auth") {
			h.Set("Cache-Control", "no-store")
		}
		c.Next()
	}
}

// CORS allows only the exact configured origins.
func CORS(origins []string) gin.HandlerFunc {
	allowed := map[string]bool{}
	for _, o := range origins {
		allowed[strings.TrimRight(o, "/")] = true
	}
	return func(c *gin.Context) {
		origin := c.GetHeader("Origin")
		if origin != "" && (allowed[origin] || allowed["*"]) {
			h := c.Writer.Header()
			h.Set("Access-Control-Allow-Origin", origin)
			h.Set("Access-Control-Allow-Credentials", "true")
			h.Add("Vary", "Origin")
			h.Set("Access-Control-Allow-Methods", "GET, POST, PUT, PATCH, DELETE, OPTIONS")
			h.Set("Access-Control-Allow-Headers", "Authorization, Content-Type, X-Requested-With, X-Request-ID")
			h.Set("Access-Control-Expose-Headers", "X-Request-ID, Retry-After")
			h.Set("Access-Control-Max-Age", "600")
		}
		if c.Request.Method == http.MethodOptions {
			c.AbortWithStatus(http.StatusNoContent)
			return
		}
		c.Next()
	}
}

// BodySizeLimit caps request bodies; multipart uploads get a larger limit.
func BodySizeLimit(defaultMax, uploadMax int64) gin.HandlerFunc {
	return func(c *gin.Context) {
		max := defaultMax
		if strings.HasPrefix(c.ContentType(), "multipart/form-data") {
			max = uploadMax
		}
		if c.Request.ContentLength > max {
			Abort(c, apperr.New(apperr.KindBadRequest, "payload_too_large", "request body too large"))
			return
		}
		c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, max)
		c.Next()
	}
}

// CSRF protects the two cookie-authenticated endpoints: the Origin must be
// allowed and a custom header must be present (§13.2).
func CSRF(origins []string) gin.HandlerFunc {
	allowed := map[string]bool{}
	for _, o := range origins {
		allowed[strings.TrimRight(o, "/")] = true
	}
	return func(c *gin.Context) {
		if c.GetHeader("X-Requested-With") != "XMLHttpRequest" || !allowed[c.GetHeader("Origin")] {
			Abort(c, apperr.Forbidden("csrf_rejected", "missing or invalid CSRF protections"))
			return
		}
		c.Next()
	}
}

// RateLimiter applies IP-keyed policies. failClosed policies reject the
// request when Redis is unavailable.
type RateLimiter struct {
	L       *ratelimit.Limiter
	Log     *slog.Logger
	Enabled bool
}

func (r *RateLimiter) ByIP(policy string, failClosed bool, limits ...ratelimit.Limit) gin.HandlerFunc {
	return func(c *gin.Context) {
		if !r.Enabled {
			c.Next()
			return
		}
		key := policy + ":ip:" + ratelimit.IPKey(c.ClientIP())
		r.apply(c, key, failClosed, limits)
	}
}

// ByUser keys the policy on the authenticated user (falls back to IP).
func (r *RateLimiter) ByUser(policy string, limits ...ratelimit.Limit) gin.HandlerFunc {
	return func(c *gin.Context) {
		if !r.Enabled {
			c.Next()
			return
		}
		key := policy + ":ip:" + ratelimit.IPKey(c.ClientIP())
		if a := rbac.ActorFrom(c.Request.Context()); a != nil {
			key = policy + ":user:" + a.UserID.String()
		}
		r.apply(c, key, false, limits)
	}
}

func (r *RateLimiter) apply(c *gin.Context, key string, failClosed bool, limits []ratelimit.Limit) {
	res, err := r.L.Allow(c.Request.Context(), key, limits...)
	if err != nil {
		r.Log.WarnContext(c.Request.Context(), "rate limiter unavailable", "error", err)
		if failClosed {
			Abort(c, apperr.Unavailable("rate_limit_unavailable", err))
			return
		}
		c.Next()
		return
	}
	if !res.Allowed {
		Abort(c, apperr.RateLimited(res.RetryAfter))
		return
	}
	c.Next()
}

// AdminIPAllowlist rejects admin traffic outside ADMIN_ALLOWED_CIDRS.
func AdminIPAllowlist(cidrs []*net.IPNet) gin.HandlerFunc {
	return func(c *gin.Context) {
		if len(cidrs) == 0 {
			c.Next()
			return
		}
		ip := net.ParseIP(c.ClientIP())
		for _, n := range cidrs {
			if ip != nil && n.Contains(ip) {
				c.Next()
				return
			}
		}
		Abort(c, apperr.Forbidden("ip_not_allowed", "access denied"))
	}
}

// Abort writes err as a Problem Details response and stops the chain.
func Abort(c *gin.Context, err error) {
	p := ToProblem(c.Request.Context(), err)
	p.Instance = c.Request.URL.Path
	if e, ok := apperr.As(err); ok && e.RetryAfter > 0 {
		c.Header("Retry-After", fmt.Sprint(int(e.RetryAfter.Seconds())+1))
	}
	c.Abort()
	problem.Write(c.Writer, p)
}

var kindStatus = map[apperr.Kind]int{
	apperr.KindInternal:     http.StatusInternalServerError,
	apperr.KindBadRequest:   http.StatusBadRequest,
	apperr.KindValidation:   http.StatusUnprocessableEntity,
	apperr.KindUnauthorized: http.StatusUnauthorized,
	apperr.KindForbidden:    http.StatusForbidden,
	apperr.KindNotFound:     http.StatusNotFound,
	apperr.KindConflict:     http.StatusConflict,
	apperr.KindRateLimited:  http.StatusTooManyRequests,
	apperr.KindUnavailable:  http.StatusServiceUnavailable,
	apperr.KindBadGateway:   http.StatusBadGateway,
}

// ToProblem is the single error mapper: internal details are logged, never returned.
func ToProblem(ctx context.Context, err error) *problem.Problem {
	meta := rbac.MetaFrom(ctx)
	e, ok := apperr.As(err)
	if !ok {
		e = apperr.Internal(err)
	}
	status := kindStatus[e.Kind]
	if status >= 500 {
		slog.ErrorContext(ctx, "request failed", "request_id", meta.RequestID, "code", e.Code, "error", err.Error())
	}
	p := problem.New(status, e.Code, e.Detail)
	if status >= 500 {
		p.Detail = "an unexpected error occurred"
		if e.Kind == apperr.KindBadGateway || e.Kind == apperr.KindUnavailable {
			p.Detail = e.Detail
		}
	}
	p.RequestID = meta.RequestID
	for _, f := range e.Fields {
		p.Errors = append(p.Errors, problem.FieldError{Field: f.Field, Code: f.Code})
	}
	return p
}
