package integration

import (
	"bytes"
	"net/http"
	"strings"
	"testing"
)

func contactTypes(m map[string]any) map[string]bool {
	out := map[string]bool{}
	for _, c := range m["contacts"].([]any) {
		out[c.(map[string]any)["type"].(string)] = true
	}
	return out
}

// Scenario 8: contact visibility is filtered server-side. The catalogue is
// public and there are no client accounts (ADR-023).
func TestContactVisibility(t *testing.T) {
	e := newEnv(t)
	adm := e.adminAccount()
	m, slug := e.readyManager()
	p, id := e.publishedProfile(m, slug)
	e.subscribeAndPay(m)
	otherM, _ := e.readyManager()
	otherP, _ := e.hire(otherM, "Nina")
	profSlug := e.browse("/api/v1/profiles").expect(200).JSON()["items"].([]any)[0].(map[string]any)["slug"].(string)
	path := "/api/v1/profiles/" + profSlug

	for name, tok := range map[string]string{"visitor": "", "other manager": otherM.Token, "other pro": otherP.Token,
		"own manager": m.Token, "own pro": p.Token, "admin": adm.Token} {
		ct := contactTypes(e.get(path, tok).expect(200).JSON())
		if !ct["PHONE"] || !ct["WHATSAPP"] || ct["TELEGRAM"] {
			t.Errorf("%s sees %v", name, ct)
		}
	}
	// "Clients only" no longer exists.
	e.put("/api/v1/manager/profiles/"+id+"/contacts", m.Token, map[string]any{"contacts": []any{
		map[string]any{"type": "PHONE", "value": "+32470111111", "visibility": "CLIENTS"},
	}}).expect(http.StatusUnprocessableEntity)
	// Establishment page: verification phone and private address never exposed.
	est := e.browse("/api/v1/establishments/" + slug).expect(200)
	if bytes.Contains(est.Body, []byte("+32470000001")) || bytes.Contains(est.Body, []byte("Rue X")) || !bytes.Contains(est.Body, []byte("+32470000003")) {
		t.Fatalf("visitor view of establishment contacts: %s", est.Body)
	}
	// Stats count the views (aggregated).
	st := e.get("/api/v1/manager/stats", m.Token).expect(200).JSON()
	if st["views"].(float64) < 1 || st["contact_views"].(float64) < 1 {
		t.Fatalf("stats %v", st)
	}
}

// Scenario 9: every publication condition (RM-22).
func TestPublicationConditions(t *testing.T) {
	e := newEnv(t)
	adm := e.adminAccount()
	m, slug := e.readyManager()
	p, id := e.publishedProfile(m, slug)
	e.subscribeAndPay(m)
	published := func() bool {
		return e.get("/api/v1/manager/profiles/"+id, m.Token).expect(200).JSON()["publication"].(map[string]any)["published"].(bool)
	}
	if !published() {
		t.Fatal("baseline should be published")
	}
	// Hidden by admin.
	e.post("/api/v1/admin/profiles/"+id+"/hide", adm.Token, map[string]any{"reason": "contrôle"}).expect(204)
	if published() {
		t.Fatal("hidden profile published")
	}
	e.post("/api/v1/admin/profiles/"+id+"/unhide", adm.Token, map[string]any{}).expect(204)
	// No age attestation any more: an unhidden profile is published again.
	if !published() {
		t.Fatal("unhidden profile should be published again")
	}
	// Linked account suspended by the manager.
	e.post("/api/v1/manager/profiles/"+id+"/account/suspend", m.Token, map[string]any{"reason": "absence"}).expect(204)
	if published() {
		t.Fatal("profile of suspended account published")
	}
	e.get("/api/v1/professional/me", p.Token).expect(401)
	e.post("/api/v1/manager/profiles/"+id+"/account/restore", m.Token, map[string]any{}).expect(204)
	if !published() {
		t.Fatal("restored")
	}
	// Primary photo removed by admin.
	photo := e.get("/api/v1/manager/profiles/"+id, m.Token).JSON()["photos"].([]any)[0].(map[string]any)["id"].(string)
	e.del("/api/v1/admin/profiles/"+id+"/photos/"+photo, adm.Token, map[string]any{"reason": "non conforme"}).expect(204)
	if published() {
		t.Fatal("profile without photo published")
	}
	e.upload("/api/v1/manager/profiles/"+id+"/photos", m.Token, testJPEG()).expect(201)
	if !published() {
		t.Fatal("new photo should republish")
	}
	// Editing the establishment page never unpublishes.
	est := e.get("/api/v1/manager/establishment", m.Token).JSON()
	region := est["region"].(map[string]any)["id"]
	city := est["city"].(map[string]any)["id"]
	e.put("/api/v1/manager/establishment", m.Token, map[string]any{"display_name": est["display_name"], "legal_name": "Autre SRL",
		"region_id": region, "city_id": city}).expect(200)
	if !published() {
		t.Fatal("an establishment edit must not unpublish")
	}
	estID := est["id"].(string)
	// Quota of active profiles.
	e.db.Gorm.Exec(`UPDATE subscription_plans SET max_active_profiles = 1`)
	_, id2 := e.publishedProfile(m, slug)
	d2 := e.get("/api/v1/manager/profiles/"+id2, m.Token).JSON()
	if d2["publication"].(map[string]any)["published"].(bool) || !contains(d2["publication"].(map[string]any)["reasons"], "ACTIVE_PROFILES_QUOTA_EXCEEDED") {
		t.Fatalf("quota not enforced: %v", d2["publication"])
	}
	// Archive suspends the linked account and unpublishes the profile.
	e.post("/api/v1/manager/profiles/"+id+"/archive", m.Token, nil).expect(204)
	if published() {
		t.Fatal("archived profile published")
	}
	e.login(p.Username, p.Password).expect(401)
	if !e.get("/api/v1/manager/profiles/"+id2, m.Token).JSON()["publication"].(map[string]any)["published"].(bool) {
		t.Fatal("the freed quota slot should go to the second profile")
	}
	// Hidden establishment.
	e.post("/api/v1/admin/establishments/"+estID+"/hide", adm.Token, map[string]any{"reason": "contrôle"}).expect(204)
	if e.browse("/api/v1/profiles").JSON()["total"].(float64) != 0 {
		t.Fatal("hidden establishment profiles published")
	}
	e.browse("/api/v1/establishments/" + slug).expect(404)
}

// Scenario 13: uploads are validated and re-encoded without EXIF.
func TestUploads(t *testing.T) {
	e := newEnv(t)
	m, _ := e.readyManager()
	p, _ := e.hire(m, "Lola")
	for name, data := range map[string][]byte{
		"svg":  []byte(`<svg xmlns="http://www.w3.org/2000/svg"><script>alert(1)</script></svg>`),
		"html": []byte("<html><script>alert(1)</script></html>"),
		"poly": append([]byte{0xFF, 0xD8, 0xFF, 0xE0}, []byte("<script>")...),
	} {
		if r := e.upload("/api/v1/professional/me/photos", p.Token, data); r.Code != http.StatusUnprocessableEntity {
			t.Errorf("%s: %d", name, r.Code)
		}
	}
	jpg := testJPEG()
	exif := append([]byte{0xFF, 0xD8, 0xFF, 0xE1, 0x00, 0x20}, append([]byte("Exif\x00\x00GPSLatitude=50.85;GPS"), jpg[2:]...)...)
	r := e.upload("/api/v1/professional/me/photos", p.Token, exif).expect(201)
	if r.JSON()["is_primary"] != true {
		t.Fatal("first photo should be primary")
	}
	var key string
	e.db.Gorm.Raw(`SELECT storage_key FROM profile_photos LIMIT 1`).Scan(&key)
	data, err := e.store.Get(t.Context(), key)
	if err != nil || bytes.Contains(data, []byte("GPSLatitude")) || !strings.HasSuffix(key, ".jpg") {
		t.Fatal("EXIF must be stripped")
	}
	for i := 0; i < 11; i++ {
		e.upload("/api/v1/professional/me/photos", p.Token, jpg).expect(201)
	}
	e.upload("/api/v1/professional/me/photos", p.Token, jpg).expect(422)
}
