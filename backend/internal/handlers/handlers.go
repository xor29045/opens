// Package handlers parses and validates HTTP input, calls the services and
// maps their results. No SQL and no business rule lives here.
package handlers

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"

	"opensex/backend/internal/apperr"
	"opensex/backend/internal/config"
	"opensex/backend/internal/middlewares"
	"opensex/backend/internal/models"
	"opensex/backend/internal/rbac"
	"opensex/backend/internal/repositories"
	"opensex/backend/internal/services"
)

type Handler struct {
	Svc *services.Services
	Cfg *config.Config
}

func New(svc *services.Services, cfg *config.Config) *Handler { return &Handler{Svc: svc, Cfg: cfg} }

func actor(c *gin.Context) *rbac.Actor { return rbac.ActorFrom(c.Request.Context()) }

func fail(c *gin.Context, err error) { middlewares.Abort(c, err) }

// bind decodes a JSON body strictly. A "role" field is never honoured: it is
// stripped, the attempt is recorded as a security event and the request
// continues (RM-01).
func (h *Handler) bind(c *gin.Context, dst any) bool {
	body, err := io.ReadAll(c.Request.Body)
	if err != nil {
		var mbe *http.MaxBytesError
		if errors.As(err, &mbe) {
			fail(c, apperr.New(apperr.KindBadRequest, "payload_too_large", "request body too large"))
		} else {
			fail(c, apperr.BadRequest("invalid_body", "cannot read request body"))
		}
		return false
	}
	if len(bytes.TrimSpace(body)) == 0 {
		body = []byte("{}")
	}
	var probe map[string]json.RawMessage
	if err := json.Unmarshal(body, &probe); err != nil {
		fail(c, apperr.BadRequest("invalid_json", "request body must be a JSON object"))
		return false
	}
	if _, ok := probe["role"]; ok {
		delete(probe, "role")
		body, _ = json.Marshal(probe)
		h.Svc.Audit.Record(c.Request.Context(), services.AuditEvent{Action: "security.role_field_ignored", Category: models.AuditSecurity,
			Severity: models.SeverityWarn, Metadata: map[string]any{"path": c.FullPath()}})
	}
	dec := json.NewDecoder(bytes.NewReader(body))
	dec.DisallowUnknownFields()
	if err := dec.Decode(dst); err != nil {
		var typeErr *json.UnmarshalTypeError
		switch {
		case errors.As(err, &typeErr):
			fail(c, apperr.Invalid(typeErr.Field, "invalid_type"))
		case strings.HasPrefix(err.Error(), "json: unknown field "):
			field := strings.Trim(strings.TrimPrefix(err.Error(), "json: unknown field "), `"`)
			fail(c, apperr.Validation(apperr.FieldError{Field: field, Code: "unknown_field"}))
		default:
			fail(c, apperr.BadRequest("invalid_json", "malformed JSON body"))
		}
		return false
	}
	return true
}

func uuidParam(c *gin.Context, name string) (uuid.UUID, bool) {
	id, err := uuid.Parse(c.Param(name))
	if err != nil {
		fail(c, apperr.NotFound("resource"))
		return uuid.Nil, false
	}
	return id, true
}

func page(c *gin.Context) repositories.Page {
	p, _ := strconv.Atoi(c.Query("page"))
	s, _ := strconv.Atoi(c.Query("page_size"))
	return repositories.NormalizePage(p, s)
}

func optBool(c *gin.Context, name string) *bool {
	v := c.Query(name)
	if v == "" {
		return nil
	}
	b, err := strconv.ParseBool(v)
	if err != nil {
		return nil
	}
	return &b
}

func ok(c *gin.Context, v any)      { c.JSON(http.StatusOK, v) }
func created(c *gin.Context, v any) { c.JSON(http.StatusCreated, v) }
func noContent(c *gin.Context)      { c.Status(http.StatusNoContent) }

func (h *Handler) now() time.Time { return h.Svc.Clock.Now() }

// ---------------------------------------------------------------- health

func (h *Handler) Healthz(c *gin.Context) { ok(c, gin.H{"status": "ok"}) }

func (h *Handler) Readyz(c *gin.Context) {
	ctx := c.Request.Context()
	checks := gin.H{"database": "ok", "redis": "ok"}
	status := http.StatusOK
	if err := h.Svc.DB.Ping(ctx); err != nil {
		checks["database"], status = "unavailable", http.StatusServiceUnavailable
	}
	if err := h.Svc.Redis.Ping(ctx).Err(); err != nil {
		checks["redis"], status = "unavailable", http.StatusServiceUnavailable
	}
	c.JSON(status, checks)
}

func parseID(s string) (uuid.UUID, error) { return uuid.Parse(s) }
