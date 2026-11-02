package integration

import (
	"net/url"
	"os"
	"strings"
	"testing"

	"opensex/backend/internal/database"
)

// T5.1: the application role cannot modify the audit trail.
func TestAppRoleCannotModifyAudit(t *testing.T) {
	e := newEnv(t)
	raw := os.Getenv("TEST_DATABASE_URL")
	e.db.Gorm.Exec(`DO $$ BEGIN IF NOT EXISTS (SELECT 1 FROM pg_roles WHERE rolname = 'app_rw') THEN CREATE ROLE app_rw LOGIN PASSWORD 'app_rw_test'; END IF; END $$`)
	if err := database.MigrateDown(raw, 1); err != nil {
		t.Fatal(err)
	}
	if err := database.MigrateUp(raw); err != nil {
		t.Fatal(err)
	}
	u, _ := url.Parse(raw)
	u.User = url.UserPassword("app_rw", "app_rw_test")
	rw, err := database.Open(database.Options{URL: u.String(), MaxConns: 2})
	if err != nil {
		t.Fatal(err)
	}
	defer rw.Close()
	if err := rw.Gorm.Exec(`INSERT INTO audit_logs (action, category, severity) VALUES ('test', 'ADMIN', 'INFO')`).Error; err != nil {
		t.Fatalf("insert should be allowed: %v", err)
	}
	for _, q := range []string{`UPDATE audit_logs SET action = 'x'`, `DELETE FROM audit_logs`, `TRUNCATE audit_logs`} {
		err := rw.Gorm.Exec(q).Error
		if err == nil || !strings.Contains(err.Error(), "permission denied") {
			t.Errorf("%s: expected permission denied, got %v", q, err)
		}
	}
	if err := rw.Gorm.Exec(`UPDATE users SET updated_at = now() WHERE false`).Error; err != nil {
		t.Fatalf("app_rw must write business tables: %v", err)
	}
}

// Scenario 14: error responses never leak internals.
func TestNoInternalLeak(t *testing.T) {
	e := newEnv(t)
	e.db.Gorm.Exec(`ALTER TABLE categories RENAME TO categories_tmp`)
	defer e.db.Gorm.Exec(`ALTER TABLE categories_tmp RENAME TO categories`)
	r := e.get("/api/v1/categories", "").expect(500)
	body := strings.ToLower(string(r.Body))
	for _, leak := range []string{"sql", "categories_tmp", "relation", "goroutine", ".go:"} {
		if strings.Contains(body, leak) {
			t.Fatalf("500 leaks %q: %s", leak, r.Body)
		}
	}
	if r.Header.Get("Content-Type") != "application/problem+json" || r.JSON()["request_id"] == "" {
		t.Fatal("problem details with request_id expected")
	}
	e.get("/api/v1/does-not-exist", "").expect(404)
	h := e.get("/healthz", "").expect(200).Header
	if h.Get("X-Content-Type-Options") != "nosniff" || !strings.Contains(h.Get("Content-Security-Policy"), "frame-ancestors 'none'") {
		t.Fatal("security headers missing")
	}
}
