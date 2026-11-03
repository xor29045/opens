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
	if r.Establishments != 6 || r.Clients != 8 || r.Profiles < 20 || r.Pending < 4 || r.Favorites == 0 {
		t.Fatalf("unexpected report %+v", r)
	}
	if _, err := devseed.Run(context.Background(), e.svc); !errors.Is(err, devseed.ErrAlreadySeeded) {
		t.Fatalf("second run: %v", err)
	}

	// Public catalogue and establishment pages.
	cat := e.get("/api/v1/profiles?page_size=50", "").expect(http.StatusOK).JSON()
	if cat["total"].(float64) < 15 {
		t.Fatalf("catalogue too small: %v", cat["total"])
	}
	e.get("/api/v1/profiles?city=anvers", "").expect(http.StatusOK)

	// Manager dashboards: overview, pending, stats, billing.
	m := e.login("rose_gerant", devseed.Password).expect(http.StatusOK).JSON()["access_token"].(string)
	ov := e.get("/api/v1/manager/publication-status", m).expect(http.StatusOK).JSON()
	if ov["pending_count"].(float64) != 2 || !ov["entitled"].(bool) {
		t.Fatalf("overview %v", ov)
	}
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

	// Client space: favorites and deck.
	c := e.login("client_alex", devseed.Password).expect(http.StatusOK).JSON()["access_token"].(string)
	if fav := e.get("/api/v1/client/favorites", c).expect(http.StatusOK).JSON(); fav["total"].(float64) == 0 {
		t.Fatal("client should have favorites")
	}
	e.get("/api/v1/client/discover", c).expect(http.StatusOK)
	e.login("client_ray", devseed.Password).expect(http.StatusUnauthorized) // suspended

	// Admin dashboards.
	kpi := e.get("/api/v1/admin/stats", adm.Token).expect(http.StatusOK).JSON()
	if kpi["mrr_cents"].(float64) == 0 || kpi["published_profiles"].(float64) < 15 {
		t.Fatalf("kpis %v", kpi)
	}
	if hidden := e.get("/api/v1/admin/profiles?hidden=true", adm.Token).expect(http.StatusOK).JSON(); hidden["total"].(float64) != 1 {
		t.Fatalf("hidden profiles %v", hidden["total"])
	}
}
