package integration

import (
	"context"
	"errors"
	"net/http"
	"testing"

	"opensex/backend/internal/devseed"
)

// The demo data set loads through the business services and gives every
// dashboard something to show. A second run is refused.
func TestDevSeed(t *testing.T) {
	e := newEnv(t)
	adm := e.adminAccount()
	r, err := devseed.Run(context.Background(), e.svc)
	if err != nil {
		t.Fatal(err)
	}
	if r.Establishments != 6 || r.Profiles < 20 || r.Drafts < 4 {
		t.Fatalf("unexpected report %+v", r)
	}
	if _, err := devseed.Run(context.Background(), e.svc); !errors.Is(err, devseed.ErrAlreadySeeded) {
		t.Fatalf("second run: %v", err)
	}

	// Public catalogue and establishment pages.
	cat := e.browse("/api/v1/profiles?page_size=50").expect(http.StatusOK).JSON()
	if cat["total"].(float64) < 15 {
		t.Fatalf("catalogue too small: %v", cat["total"])
	}
	e.browse("/api/v1/profiles?city=13-bouches-du-rhone").expect(http.StatusOK)

	// Manager dashboards: overview, stats, billing.
	m := e.login("rose_gerant", devseed.Password).expect(http.StatusOK).JSON()["access_token"].(string)
	ov := e.get("/api/v1/manager/publication-status", m).expect(http.StatusOK).JSON()
	if !ov["entitled"].(bool) {
		t.Fatalf("overview %v", ov)
	}
	// Seeded professionals sign in with the login created by their house.
	var proName string
	e.db.Gorm.Raw(`SELECT u.username FROM users u JOIN professional_profiles p ON p.user_id = u.id LIMIT 1`).Scan(&proName)
	pro := e.login(proName, devseed.Password).expect(http.StatusOK).JSON()["access_token"].(string)
	e.get("/api/v1/professional/me", pro).expect(http.StatusOK)
	if st := e.get("/api/v1/manager/stats", m).expect(http.StatusOK).JSON(); st["views"].(float64) == 0 {
		t.Fatal("stats should be filled")
	}
	if inv := e.get("/api/v1/billing/invoices", m).expect(http.StatusOK).JSON(); inv["total"].(float64) != 1 {
		t.Fatalf("invoices %v", inv["total"])
	}
	opale := e.login("opale_gerant", devseed.Password).expect(http.StatusOK).JSON()["access_token"].(string)
	if sub := e.get("/api/v1/billing/subscription", opale).expect(http.StatusOK).JSON(); sub["subscription"].(map[string]any)["status"] != "PAST_DUE" {
		t.Fatalf("opale should be past due: %v", sub["subscription"])
	}
	velours := e.login("velours_gerant", devseed.Password).expect(http.StatusOK).JSON()["access_token"].(string)
	if sub := e.get("/api/v1/billing/subscription", velours).expect(http.StatusOK).JSON(); sub["open_payment"] == nil {
		t.Fatal("velours should have an open renewal invoice")
	}

	// Anonymous deck.
	if d := e.post("/api/v1/discover", "", map[string]any{}).expect(http.StatusOK).JSON(); len(d["items"].([]any)) == 0 {
		t.Fatal("deck should be filled")
	}

	// Admin dashboards.
	kpi := e.get("/api/v1/admin/stats", adm.Token).expect(http.StatusOK).JSON()
	if kpi["mrr_cents"].(float64) == 0 || kpi["published_profiles"].(float64) < 15 {
		t.Fatalf("kpis %v", kpi)
	}
	if hidden := e.get("/api/v1/admin/profiles?hidden=true", adm.Token).expect(http.StatusOK).JSON(); hidden["total"].(float64) != 1 {
		t.Fatalf("hidden profiles %v", hidden["total"])
	}
}
