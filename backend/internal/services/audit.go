package services

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"strings"
	"time"

	"opensex/backend/internal/models"
	"opensex/backend/internal/rbac"
)

type AuditService struct{ *Services }

type AuditEvent struct {
	Action     string
	Category   string
	Severity   string
	TargetType string
	TargetID   string
	Metadata   map[string]any
	// ActorID/ActorRole override the context actor (e.g. login before a token exists).
	ActorID   *string
	ActorRole string
	// Alert also notifies the admin webhook (§13.6).
	Alert bool
}

var forbiddenMetaKeys = []string{"password", "token", "secret", "code", "recovery", "authorization", "cookie", "signature", "hash"}

func sanitizeMeta(m map[string]any) map[string]any {
	out := map[string]any{}
	for k, v := range m {
		lk := strings.ToLower(k)
		skip := false
		for _, f := range forbiddenMetaKeys {
			if strings.Contains(lk, f) {
				skip = true
				break
			}
		}
		if !skip {
			out[k] = v
		}
	}
	return out
}

// Record appends an audit entry. It joins the caller's transaction when
// present so that the entry is committed atomically with the change.
// Failures are logged but never block the business operation.
func (a *AuditService) Record(ctx context.Context, e AuditEvent) {
	meta := rbac.MetaFrom(ctx)
	actor := rbac.ActorFrom(ctx)
	if e.Severity == "" {
		e.Severity = models.SeverityInfo
	}
	metaJSON, _ := json.Marshal(sanitizeMeta(e.Metadata))
	l := &models.AuditLog{
		OccurredAt: a.now(),
		Action:     e.Action,
		Category:   e.Category,
		Severity:   e.Severity,
		Metadata:   metaJSON,
	}
	if actor != nil {
		id := actor.UserID
		l.ActorUserID = &id
		l.ActorRole = ptr(actor.Role)
	}
	if e.ActorID != nil {
		if id, err := parseUUID(*e.ActorID); err == nil {
			l.ActorUserID = &id
		}
	}
	if e.ActorRole != "" {
		l.ActorRole = ptr(e.ActorRole)
	}
	if e.TargetType != "" {
		l.TargetType = ptr(e.TargetType)
	}
	if e.TargetID != "" {
		l.TargetID = ptr(e.TargetID)
	}
	if meta.IPHash != "" {
		l.IPHash = ptr(meta.IPHash)
	}
	if meta.UserAgent != "" {
		l.UserAgent = ptr(truncate(meta.UserAgent, 255))
	}
	if meta.RequestID != "" {
		l.RequestID = ptr(meta.RequestID)
	}
	if err := a.Repos.Audit.Insert(ctx, l); err != nil {
		a.Logger.ErrorContext(ctx, "audit insert failed", "action", e.Action, "error", err)
	}
	if e.Alert {
		a.alert(ctx, e)
	}
}

// Security records a SECURITY category event.
func (a *AuditService) Security(ctx context.Context, action, severity string, meta map[string]any) {
	a.Record(ctx, AuditEvent{Action: action, Category: models.AuditSecurity, Severity: severity, Metadata: meta})
}

func (a *AuditService) alert(ctx context.Context, e AuditEvent) {
	a.Logger.WarnContext(ctx, "admin alert", "action", e.Action, "target", e.TargetID)
	url := a.Cfg.AdminAlertWebhookURL
	if url == "" {
		return
	}
	body, _ := json.Marshal(map[string]any{
		"event":       e.Action,
		"severity":    e.Severity,
		"target_type": e.TargetType,
		"target_id":   e.TargetID,
		"occurred_at": a.now().Format(time.RFC3339),
		"request_id":  rbac.MetaFrom(ctx).RequestID,
	})
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(body))
		if err != nil {
			return
		}
		req.Header.Set("Content-Type", "application/json")
		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			a.Logger.Error("alert webhook failed", "error", err)
			return
		}
		resp.Body.Close()
	}()
}

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n]
}
