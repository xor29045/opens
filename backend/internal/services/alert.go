package services

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"time"

	"opensex/backend/internal/rbac"
)

// AlertService notifies the platform admins of sensitive security and
// billing events (§13.6): a structured WARN log line and, when configured,
// a POST to the admin webhook. It never blocks the business operation.
type AlertService struct{ *Services }

func (a *AlertService) Send(ctx context.Context, event, severity, targetType, targetID string) {
	requestID := rbac.MetaFrom(ctx).RequestID
	a.Logger.WarnContext(ctx, "admin alert", "event", event, "severity", severity,
		"target_type", targetType, "target_id", targetID, "request_id", requestID)
	url := a.Cfg.AdminAlertWebhookURL
	if url == "" {
		return
	}
	body, _ := json.Marshal(map[string]any{
		"event":       event,
		"severity":    severity,
		"target_type": targetType,
		"target_id":   targetID,
		"occurred_at": a.now().Format(time.RFC3339),
		"request_id":  requestID,
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
