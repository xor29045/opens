package integration

import (
	"net/http"
	"testing"
)

// End-to-end: establishment → manager creates the professional (ADR-022) →
// subscription (signed IPN) → publication.
func TestPublicationFlow(t *testing.T) {
	e := newEnv(t)
	m, slug := e.readyManager()
	p, id := e.hire(m, "Lola")
	e.upload("/api/v1/professional/me/photos", p.Token, testJPEG()).expect(http.StatusCreated)
	e.put("/api/v1/professional/me/rates", p.Token, map[string]any{"rates": []any{map[string]any{"label": "1 heure", "duration_minutes": 60, "amount_cents": 15000}}}).expect(http.StatusOK)

	// Without a subscription the house cannot put the professional in service.
	r := e.post("/api/v1/manager/profiles/"+id+"/activate", m.Token, nil).expect(http.StatusUnprocessableEntity)
	if r.problemCode() != "subscription_required" {
		t.Fatalf("expected subscription_required, got %s", r.Body)
	}
	st := e.get("/api/v1/manager/profiles/"+id, m.Token).expect(http.StatusOK).JSON()
	if st["status"] == "ACTIVE" || st["publication"].(map[string]any)["published"].(bool) {
		t.Fatal("profile in service without subscription")
	}
	// Paying the subscription puts the profile in service at once.
	e.subscribeAndPay(m)
	st = e.get("/api/v1/manager/profiles/"+id, m.Token).expect(http.StatusOK).JSON()
	if st["status"] != "ACTIVE" {
		t.Fatalf("profile should be in service after payment, status=%v", st["status"])
	}
	if !st["publication"].(map[string]any)["published"].(bool) {
		t.Fatalf("expected publication, reasons=%v", st["publication"])
	}
	list := e.browse("/api/v1/profiles").expect(http.StatusOK).JSON()
	if list["total"].(float64) != 1 {
		t.Fatalf("catalog total %v", list["total"])
	}
	card := list["items"].([]any)[0].(map[string]any)
	if card["photo_url"] == nil {
		t.Fatalf("card incomplete: %v", card)
	}
	profSlug := card["slug"].(string)
	detail := e.browse("/api/v1/profiles/" + profSlug).expect(http.StatusOK).JSON()
	est := detail["establishment"].(map[string]any)
	if _, ok := est["approval_number"]; ok {
		t.Fatal("approval number must no longer be exposed")
	}
	if _, ok := detail["birth_date"]; ok {
		t.Fatal("birth date exposed")
	}
	e.browse("/api/v1/establishments/" + slug).expect(http.StatusOK)
	// Filters
	if e.browse("/api/v1/profiles?city=75-paris&price_max=200").expect(200).JSON()["total"].(float64) != 1 {
		t.Fatal("filters should match")
	}
	if e.browse("/api/v1/profiles?city=13-bouches-du-rhone").expect(200).JSON()["total"].(float64) != 0 {
		t.Fatal("city filter")
	}
	e.browse("/api/v1/profiles?sort=price%3BDROP").expect(http.StatusUnprocessableEntity)

	// Invoice issued
	inv := e.get("/api/v1/billing/invoices", m.Token).expect(200).JSON()
	if inv["total"].(float64) != 1 {
		t.Fatal("invoice missing")
	}
	invID := inv["items"].([]any)[0].(map[string]any)["id"].(string)
	pdf := e.get("/api/v1/billing/invoices/"+invID+"/pdf", m.Token).expect(200)
	if string(pdf.Body[:5]) != "%PDF-" {
		t.Fatal("not a PDF")
	}
}

// ADR-021/022: no validation step once subscribed. A profile created by a
// paid house is in service at once and published as soon as it has a photo.
func TestNoWaitOnceSubscribed(t *testing.T) {
	e := newEnv(t)
	m, _ := e.readyManager()
	e.subscribeAndPay(m)
	p, _ := e.hire(m, "Mia")
	st := e.get("/api/v1/professional/me/publication-status", p.Token).expect(http.StatusOK).JSON()
	if st["status"] != "ACTIVE" || !st["published"].(bool) {
		t.Fatalf("profile of a paid house should be in service and published at once, even without photo: %v", st)
	}
	e.upload("/api/v1/professional/me/photos", p.Token, testJPEG()).expect(http.StatusCreated)
	st = e.get("/api/v1/professional/me/publication-status", p.Token).expect(http.StatusOK).JSON()
	if !st["published"].(bool) {
		t.Fatalf("profile should be published: %v", st)
	}
}
