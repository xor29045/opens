package integration

import (
	"net/http"
	"testing"
)

// End-to-end: establishment → verification → subscription (signed IPN) →
// professional registration → attestation → publication.
func TestPublicationFlow(t *testing.T) {
	e := newEnv(t)
	m, slug := e.readyManager()
	p := e.register("professional", map[string]any{"establishment_slug": slug, "display_name": "Lola"})
	id := e.get("/api/v1/professional/me", p.Token).expect(http.StatusOK).JSON()["id"].(string)
	e.upload("/api/v1/professional/me/photos", p.Token, testJPEG()).expect(http.StatusCreated)
	e.put("/api/v1/professional/me/rates", p.Token, map[string]any{"rates": []any{map[string]any{"label": "1 heure", "duration_minutes": 60, "amount_cents": 15000}}}).expect(http.StatusOK)
	e.put("/api/v1/manager/profiles/"+id+"/attestation", m.Token, map[string]any{"birth_date": "1995-03-14", "attested": true}).expect(http.StatusOK)

	// Without a subscription the house cannot put the professional in service.
	r := e.post("/api/v1/manager/profiles/"+id+"/activate", m.Token, nil).expect(http.StatusUnprocessableEntity)
	if r.problemCode() != "subscription_required" {
		t.Fatalf("expected subscription_required, got %s", r.Body)
	}
	st := e.get("/api/v1/manager/profiles/"+id, m.Token).expect(http.StatusOK).JSON()
	if st["status"] == "ACTIVE" || st["publication"].(map[string]any)["published"].(bool) {
		t.Fatal("profile in service without subscription")
	}
	// Paying the subscription puts the attested profile in service at once.
	e.subscribeAndPay(m)
	st = e.get("/api/v1/manager/profiles/"+id, m.Token).expect(http.StatusOK).JSON()
	if st["status"] != "ACTIVE" {
		t.Fatalf("attested profile should be in service after payment, status=%v", st["status"])
	}
	if !st["publication"].(map[string]any)["published"].(bool) {
		t.Fatalf("expected publication, reasons=%v", st["publication"])
	}
	list := e.browse("/api/v1/profiles").expect(http.StatusOK).JSON()
	if list["total"].(float64) != 1 {
		t.Fatalf("catalog total %v", list["total"])
	}
	card := list["items"].([]any)[0].(map[string]any)
	if card["age"] == nil || card["photo_url"] == nil {
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
	if e.browse("/api/v1/profiles?city=ixelles&age_min=25&price_max=200").expect(200).JSON()["total"].(float64) != 1 {
		t.Fatal("filters should match")
	}
	if e.browse("/api/v1/profiles?city=anvers").expect(200).JSON()["total"].(float64) != 0 {
		t.Fatal("city filter")
	}
	e.browse("/api/v1/profiles?age_min=12").expect(http.StatusUnprocessableEntity)
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

// ADR-020: with a paid subscription there is no extra validation step, but a
// professional's registration still waits for the manager's age attestation.
func TestNoWaitOnceSubscribed(t *testing.T) {
	e := newEnv(t)
	m, slug := e.readyManager()
	e.subscribeAndPay(m)
	p := e.register("professional", map[string]any{"establishment_slug": slug, "display_name": "Mia"})
	id := e.get("/api/v1/professional/me", p.Token).expect(http.StatusOK).JSON()["id"].(string)
	e.upload("/api/v1/professional/me/photos", p.Token, testJPEG()).expect(http.StatusCreated)

	// Never published before the attestation, even with a paid subscription.
	st := e.get("/api/v1/professional/me/publication-status", p.Token).expect(http.StatusOK).JSON()
	if st["published"].(bool) || st["status"] != "PENDING_APPROVAL" {
		t.Fatalf("registration must wait for the age attestation: %v", st)
	}
	// The attestation alone puts the profile in service and publishes it.
	d := e.put("/api/v1/manager/profiles/"+id+"/attestation", m.Token, map[string]any{"birth_date": "1998-06-01", "attested": true}).expect(http.StatusOK).JSON()
	if d["status"] != "ACTIVE" || !d["publication"].(map[string]any)["published"].(bool) {
		t.Fatalf("attested profile should be in service immediately: %v %v", d["status"], d["publication"])
	}
}
