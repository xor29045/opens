package integration

import (
	"net/http"
	"testing"
)

// End-to-end: establishment → verification → subscription (signed IPN) →
// professional registration → attestation → publication.
func TestPublicationFlow(t *testing.T) {
	e := newEnv(t)
	adm := e.adminAccount()
	m, slug := e.verifiedManager(adm.Token)
	_, id := e.publishedProfile(m, slug)

	// Not yet published: no subscription.
	st := e.get("/api/v1/manager/profiles/"+id, m.Token).expect(http.StatusOK).JSON()
	pub := st["publication"].(map[string]any)
	if pub["published"].(bool) {
		t.Fatal("published without subscription")
	}
	e.subscribeAndPay(m)
	st = e.get("/api/v1/manager/profiles/"+id, m.Token).expect(http.StatusOK).JSON()
	if !st["publication"].(map[string]any)["published"].(bool) {
		t.Fatalf("expected publication, reasons=%v", st["publication"])
	}
	list := e.get("/api/v1/profiles", "").expect(http.StatusOK).JSON()
	if list["total"].(float64) != 1 {
		t.Fatalf("catalog total %v", list["total"])
	}
	card := list["items"].([]any)[0].(map[string]any)
	if card["age"] == nil || card["photo_url"] == nil {
		t.Fatalf("card incomplete: %v", card)
	}
	profSlug := card["slug"].(string)
	detail := e.get("/api/v1/profiles/"+profSlug, "").expect(http.StatusOK).JSON()
	est := detail["establishment"].(map[string]any)
	if est["approval_number"] == "" || est["legal_name"] == "" {
		t.Fatal("approval number and legal name must be displayed (RM-12)")
	}
	if _, ok := detail["birth_date"]; ok {
		t.Fatal("birth date exposed")
	}
	e.get("/api/v1/establishments/"+slug, "").expect(http.StatusOK)
	// Filters
	if e.get("/api/v1/profiles?city=ixelles&age_min=25&price_max=200", "").expect(200).JSON()["total"].(float64) != 1 {
		t.Fatal("filters should match")
	}
	if e.get("/api/v1/profiles?city=anvers", "").expect(200).JSON()["total"].(float64) != 0 {
		t.Fatal("city filter")
	}
	e.get("/api/v1/profiles?age_min=12", "").expect(http.StatusUnprocessableEntity)
	e.get("/api/v1/profiles?sort=price%3BDROP", "").expect(http.StatusUnprocessableEntity)

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
