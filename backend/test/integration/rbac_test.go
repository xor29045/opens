package integration

import (
	"net/http"
	"strings"
	"testing"

	"github.com/google/uuid"

	"opensex/backend/internal/models"
)

// Scenario 1: generated RBAC coverage of every route: no token → 401,
// wrong role → 403.
func TestRBACCoverage(t *testing.T) {
	e := newEnv(t)
	adm := e.adminAccount()
	m := e.register("manager", nil)
	c := e.register("client", nil)
	_, slug := e.readyManager()
	p := e.register("professional", map[string]any{"establishment_slug": slug})
	tokens := map[string]string{models.RoleAdmin: adm.Token, models.RoleManager: m.Token, models.RoleClient: c.Token, models.RoleProfessional: p.Token}

	checked := 0
	for _, rt := range e.routes {
		if len(rt.Access.Roles) == 0 {
			continue
		}
		path := rt.Path
		for _, param := range []string{":profileId", ":photoId", ":id"} {
			path = strings.ReplaceAll(path, param, uuid.NewString())
		}
		if r := e.send(req{method: rt.Method, path: path, body: map[string]any{}}); r.Code != http.StatusUnauthorized {
			t.Errorf("%s %s without token: %d", rt.Method, rt.Path, r.Code)
		}
		allowed := map[string]bool{}
		for _, role := range rt.Access.Roles {
			allowed[role] = true
		}
		for role, tok := range tokens {
			if allowed[role] {
				continue
			}
			if r := e.send(req{method: rt.Method, path: path, token: tok, body: map[string]any{}}); r.Code != http.StatusForbidden {
				t.Errorf("%s %s as %s: %d", rt.Method, rt.Path, role, r.Code)
			}
		}
		checked++
	}
	if checked < 80 {
		t.Fatalf("only %d protected routes checked", checked)
	}
}

// Scenario 5: a "role" field is ignored.
func TestRoleFieldIgnored(t *testing.T) {
	e := newEnv(t)
	r := e.post("/api/v1/auth/register/client", "", map[string]any{"username": "sneaky", "password": pwd, "accept_terms": true, "role": "ADMIN"}).expect(http.StatusCreated)
	if role := r.JSON()["user"].(map[string]any)["role"]; role != models.RoleClient {
		t.Fatalf("role = %v", role)
	}
	// Unknown fields are rejected.
	e.post("/api/v1/auth/register/client", "", map[string]any{"username": "x12", "password": pwd, "accept_terms": true, "is_admin": true}).
		expect(http.StatusUnprocessableEntity)
}

// Scenario 6: a manager cannot reach the resources of another establishment.
func TestCrossEstablishmentIDOR(t *testing.T) {
	e := newEnv(t)
	a, _ := e.readyManager()
	b, slugB := e.readyManager()
	_, idB := e.publishedProfile(b, slugB)
	e.subscribeAndPay(b)
	photos := e.get("/api/v1/manager/profiles/"+idB, b.Token).expect(200).JSON()["photos"].([]any)
	photoB := photos[0].(map[string]any)["id"].(string)
	pays := e.get("/api/v1/billing/payments", b.Token).expect(200).JSON()["items"].([]any)
	payB := pays[0].(map[string]any)["id"].(string)
	invs := e.get("/api/v1/billing/invoices", b.Token).expect(200).JSON()["items"].([]any)
	invB := invs[0].(map[string]any)["id"].(string)

	base := "/api/v1/manager/profiles/" + idB
	attempts := []req{
		{method: "GET", path: base},
		{method: "PUT", path: base, body: map[string]any{"display_name": "hacked"}},
		{method: "DELETE", path: base},
		{method: "PUT", path: base + "/attestation", body: map[string]any{"birth_date": "1990-01-01", "employment_reference": "x", "attested": true}},
		{method: "DELETE", path: base + "/attestation"},
		{method: "POST", path: base + "/activate"},
		{method: "POST", path: base + "/archive"},
		{method: "POST", path: base + "/reject", body: map[string]any{"reason": "x"}},
		{method: "PUT", path: base + "/rates", body: map[string]any{"rates": []any{map[string]any{"label": "x", "amount_cents": 100}}}},
		{method: "PATCH", path: base + "/photos/" + photoB, body: map[string]any{"is_primary": true}},
		{method: "DELETE", path: base + "/photos/" + photoB},
		{method: "GET", path: base + "/preview"},
		{method: "POST", path: base + "/account/suspend", body: map[string]any{"reason": "x"}},
		{method: "POST", path: base + "/account/password-reset", body: map[string]any{"reason": "x"}},
		{method: "GET", path: "/api/v1/billing/payments/" + payB},
		{method: "GET", path: "/api/v1/billing/invoices/" + invB + "/pdf"},
	}
	for _, at := range attempts {
		at.token = a.Token
		if r := e.send(at); r.Code != http.StatusNotFound {
			t.Errorf("%s %s by other manager: %d %s", at.method, at.path, r.Code, r.Body)
		}
	}
	// B's profile is untouched and still published.
	d := e.get(base, b.Token).expect(200).JSON()
	if d["display_name"] != "Lola" || !d["publication"].(map[string]any)["published"].(bool) {
		t.Fatal("profile of B was modified")
	}
}

// Scenario 6b: a professional only reaches their own profile.
func TestProfessionalIsolation(t *testing.T) {
	e := newEnv(t)
	m, slug := e.readyManager()
	p1, _ := e.publishedProfile(m, slug)
	_, id2 := e.publishedProfile(m, slug)
	photos := e.get("/api/v1/manager/profiles/"+id2, m.Token).expect(200).JSON()["photos"].([]any)
	photo2 := photos[0].(map[string]any)["id"].(string)

	e.del("/api/v1/professional/me/photos/"+photo2, p1.Token, nil).expect(http.StatusNotFound)
	e.patch("/api/v1/professional/me/photos/"+photo2, p1.Token, map[string]any{"is_primary": true}).expect(http.StatusNotFound)
	for _, path := range []string{"/api/v1/manager/profiles", "/api/v1/manager/profiles/" + id2, "/api/v1/billing/subscription", "/api/v1/billing/invoices"} {
		e.get(path, p1.Token).expect(http.StatusForbidden)
	}
	e.put("/api/v1/manager/profiles/"+id2+"/attestation", p1.Token, map[string]any{}).expect(http.StatusForbidden)
	// The manager can edit content (H-6), the edit is attributed.
	d := e.put("/api/v1/manager/profiles/"+id2, m.Token, map[string]any{"display_name": "Mia", "headline": "Bonjour"}).expect(200).JSON()
	if *strPtr(d["last_edit"].(map[string]any)["by"]) != m.Username {
		t.Fatal("last editor should be the manager")
	}
}

func strPtr(v any) *string {
	s, _ := v.(string)
	return &s
}

// Scenario 6c: professional registration lifecycle.
func TestProfessionalRegistration(t *testing.T) {
	e := newEnv(t)
	m, slug := e.readyManager()

	// A house without public name is not selectable (no verification step).
	draft := e.register("manager", nil)
	draftSlug := e.get("/api/v1/manager/establishment", draft.Token).expect(200).JSON()["slug"].(string)
	e.post("/api/v1/auth/register/professional", "", map[string]any{"username": "pro_x1", "password": pwd, "accept_terms": true,
		"establishment_slug": draftSlug}).expect(http.StatusUnprocessableEntity)
	list := e.get("/api/v1/auth/register/establishments", "").expect(200).JSON()["items"].([]any)
	if len(list) != 1 || list[0].(map[string]any)["slug"] != slug {
		t.Fatalf("selectable establishments: %v", list)
	}

	p := e.register("professional", map[string]any{"establishment_slug": slug})
	me := e.get("/api/v1/professional/me", p.Token).expect(200).JSON()
	if me["status"] != models.ProfilePendingApproval {
		t.Fatal("profile should be pending")
	}
	if _, ok := me["linked_account"]; ok {
		t.Fatal("professional must not see manager-only fields")
	}
	// Cannot attest or activate themselves.
	e.put("/api/v1/manager/profiles/"+me["id"].(string)+"/attestation", p.Token, map[string]any{}).expect(http.StatusForbidden)
	st := e.get("/api/v1/professional/me/publication-status", p.Token).expect(200).JSON()
	if st["published"].(bool) || !contains(st["reasons"], "PENDING_MANAGER_APPROVAL") {
		t.Fatalf("status %v", st)
	}
	pending := e.get("/api/v1/manager/pending", m.Token).expect(200).JSON()
	if pending["total"].(float64) != 1 {
		t.Fatal("pending registration not visible to the manager")
	}
	// Under-age attestation → 422.
	r := e.put("/api/v1/manager/profiles/"+me["id"].(string)+"/attestation", m.Token,
		map[string]any{"birth_date": "2010-01-01", "employment_reference": "CT", "attested": true}).expect(http.StatusUnprocessableEntity)
	if !strings.Contains(string(r.Body), "under_18") {
		t.Fatal("expected under_18")
	}
	// Rejection deletes profile and account and revokes sessions.
	e.post("/api/v1/manager/profiles/"+me["id"].(string)+"/reject", m.Token, map[string]any{"reason": "ne travaille pas ici"}).expect(http.StatusNoContent)
	e.get("/api/v1/professional/me", p.Token).expect(http.StatusUnauthorized)
	e.login(p.Username, p.Password).expect(http.StatusUnauthorized)

	// Quota of pending registrations.
	e.db.Gorm.Exec(`UPDATE subscription_plans SET max_pending_profiles = 1`)
	e.register("professional", map[string]any{"establishment_slug": slug})
	r = e.post("/api/v1/auth/register/professional", "", map[string]any{"username": "pro_quota", "password": pwd, "accept_terms": true, "establishment_slug": slug})
	if r.Code != http.StatusUnprocessableEntity || r.problemCode() != "establishment_pending_quota" {
		t.Fatalf("quota: %d %s", r.Code, r.Body)
	}
}

func contains(list any, v string) bool {
	for _, x := range list.([]any) {
		if x == v {
			return true
		}
	}
	return false
}
