package integration

import (
	"context"
	"testing"
	"time"

	"opensex/backend/internal/jobs"
)

func (e *env) subStatus(m account) map[string]any {
	tok := e.login(m.Username, m.Password).expect(200).JSON()["access_token"].(string)
	return e.get("/api/v1/billing/subscription", tok).expect(200).JSON()
}

// fresh returns m with a new access token (after the fake clock moved).
func (e *env) fresh(m account) account {
	m.Token = e.login(m.Username, m.Password).expect(200).JSON()["access_token"].(string)
	return m
}

// Scenario 11: IPN security and idempotence.
func TestIPN(t *testing.T) {
	e := newEnv(t)
	adm := e.adminAccount()
	m, _ := e.readyManager()

	p := e.post("/api/v1/billing/subscription", m.Token, map[string]any{}).expect(201).JSON()
	pid := p["id"].(string)
	if p["invoice_url"] == nil {
		t.Fatal("invoice URL expected")
	}
	// Second subscribe returns the same open invoice.
	if again := e.post("/api/v1/billing/subscription", m.Token, map[string]any{}).expect(201).JSON(); again["id"] != pid {
		t.Fatal("open invoice should be reused")
	}
	// Missing / invalid signature → 401 without effect.
	e.send(req{method: "POST", path: "/api/v1/webhooks/nowpayments", raw: []byte(`{"payment_id":1,"payment_status":"finished","order_id":"` + pid + `"}`), contentType: "application/json"}).expect(401)
	e.send(req{method: "POST", path: "/api/v1/webhooks/nowpayments", raw: []byte(`{"payment_id":1,"payment_status":"finished","order_id":"` + pid + `"}`),
		contentType: "application/json", headers: map[string]string{"x-nowpayments-sig": "deadbeef"}}).expect(401)
	if e.subStatus(m)["subscription"].(map[string]any)["status"] != "INCOMPLETE" {
		t.Fatal("must stay INCOMPLETE")
	}
	// IPN says finished but the API says waiting → no activation.
	claimed := paymentStatus(pid, "finished")
	body, sig := e.pay.SignedIPN(claimed)
	actual := claimed
	actual.RawStatus = "waiting"
	e.pay.SetPayment(actual)
	e.send(req{method: "POST", path: "/api/v1/webhooks/nowpayments", raw: body, contentType: "application/json", headers: map[string]string{"x-nowpayments-sig": sig}}).expect(200)
	if e.subStatus(m)["entitled"].(bool) {
		t.Fatal("activated on an unverified status")
	}
	// Under-payment beyond tolerance → PARTIAL, no activation.
	e.payIPN(pid, "partially_paid", "1.0", "0.9").expect(200)
	pay := e.get("/api/v1/billing/payments/"+pid, m.Token).JSON()
	if pay["status"] != "PARTIAL" || e.subStatus(m)["entitled"].(bool) {
		t.Fatalf("partial payment: %v", pay["status"])
	}
	// Admin regularisation of the partial payment.
	e.post("/api/v1/admin/payments/"+pid+"/reconcile", adm.Token, map[string]any{"reason": "complément reçu", "force_paid": true}).expect(200)
	st := e.subStatus(m)
	if !st["entitled"].(bool) || st["subscription"].(map[string]any)["status"] != "ACTIVE" {
		t.Fatalf("not activated: %v", st)
	}
	// Replays and out-of-order notifications have no effect.
	e.payIPN(pid, "finished", "1.0", "1.0").expect(200)
	e.payIPN(pid, "finished", "1.0", "1.0").expect(200)
	e.payIPN(pid, "confirming", "1.0", "1.0").expect(200)
	if e.get("/api/v1/billing/payments/"+pid, m.Token).JSON()["status"] != "PAID" {
		t.Fatal("status went backwards")
	}
	var invoices int64
	e.db.Gorm.Raw(`SELECT count(*) FROM invoices`).Scan(&invoices)
	if invoices != 1 {
		t.Fatalf("%d invoices", invoices)
	}
	// Mismatched amount is rejected and flagged.
	p2 := e.post("/api/v1/billing/subscription/renew", m.Token, nil).expect(201).JSON()
	bad := paymentStatus(p2["id"].(string), "finished")
	bad.PriceAmount = "1"
	e.pay.SetPayment(bad)
	body, sig = e.pay.SignedIPN(bad)
	e.send(req{method: "POST", path: "/api/v1/webhooks/nowpayments", raw: body, contentType: "application/json", headers: map[string]string{"x-nowpayments-sig": sig}}).expect(200)
	if e.get("/api/v1/billing/payments/"+p2["id"].(string), m.Token).JSON()["status"] != "PENDING" {
		t.Fatal("mismatched payment applied")
	}
}

// T5.17: subscription life cycle with a simulated clock, J-5 → J+14.
func TestSubscriptionLifecycle(t *testing.T) {
	e := newEnv(t)
	m, slug := e.readyManager()
	e.publishedProfile(m, slug)
	e.subscribeAndPay(m)
	ctx := context.Background()
	run := func(name string) {
		for _, j := range jobs.Jobs(e.svc) {
			if j.Name == name {
				if _, _, err := jobs.RunOnce(ctx, e.svc, j); err != nil {
					t.Fatal(err)
				}
			}
		}
	}
	catalog := func() float64 { return e.browse("/api/v1/profiles").JSON()["total"].(float64) }

	// J-5: renewal invoice + reminder.
	e.clock.Advance(25*24*time.Hour + time.Hour)
	run("renewal_invoices")
	run("renewal_invoices") // no duplicate
	var open int64
	e.db.Gorm.Raw(`SELECT count(*) FROM payments WHERE status = 'PENDING'`).Scan(&open)
	if open != 1 {
		t.Fatalf("%d open renewal payments", open)
	}
	if e.subStatus(m)["reminder"].(map[string]any)["level"] != "renewal_due" {
		t.Fatal("renewal reminder expected")
	}
	// J+1: PAST_DUE, still published (grace).
	e.clock.Advance(6 * 24 * time.Hour)
	run("expire_periods")
	st := e.subStatus(m)
	if st["subscription"].(map[string]any)["status"] != "PAST_DUE" || st["reminder"].(map[string]any)["level"] != "past_due" || catalog() != 1 {
		t.Fatalf("J+1: %v", st)
	}
	// J+4: grace over → unpublished.
	e.clock.Advance(3 * 24 * time.Hour)
	run("expire_periods")
	if catalog() != 0 {
		t.Fatal("profiles must be unpublished after the grace period")
	}
	// J+14: canceled.
	e.clock.Advance(10 * 24 * time.Hour)
	run("expire_periods")
	if e.subStatus(m)["subscription"].(map[string]any)["status"] != "CANCELED" {
		t.Fatal("subscription should be canceled at J+14")
	}
	// New subscription works again.
	m = e.fresh(m)
	e.subscribeAndPay(m)
	if catalog() != 1 {
		t.Fatal("republished after new subscription")
	}
	// Cancellation at period end keeps the profiles until the end.
	m = e.fresh(m)
	e.post("/api/v1/billing/subscription/cancel", m.Token, nil).expect(200)
	if catalog() != 1 {
		t.Fatal("still published until period end")
	}
	e.post("/api/v1/billing/subscription/resume", m.Token, nil).expect(200)
	e.post("/api/v1/billing/subscription/cancel", m.Token, nil).expect(200)
	e.clock.Advance(31 * 24 * time.Hour)
	run("expire_periods")
	if catalog() != 0 || e.subStatus(m)["subscription"].(map[string]any)["status"] != "CANCELED" {
		t.Fatal("canceled at period end")
	}
	// Two workers: the advisory lock prevents concurrent runs.
	release, ok, err := e.db.TryLock(ctx, "job:expire_periods")
	if err != nil || !ok {
		t.Fatal(err)
	}
	for _, j := range jobs.Jobs(e.svc) {
		if j.Name == "expire_periods" {
			if _, skipped, _ := jobs.RunOnce(ctx, e.svc, j); !skipped {
				t.Fatal("second worker should skip")
			}
		}
	}
	release()
}
