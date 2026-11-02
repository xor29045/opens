package payment

import "testing"

func TestSignatureSortedKeys(t *testing.T) {
	secret := []byte("ipn-secret")
	a := []byte(`{"payment_status":"finished","payment_id":123,"order_id":"o/1<x>","actually_paid":0.0105}`)
	b := []byte(`{"order_id":"o/1<x>","actually_paid":0.0105,"payment_id":123,"payment_status":"finished"}`)
	sigA, _ := SignIPN(secret, a)
	sigB, _ := SignIPN(secret, b)
	if sigA != sigB {
		t.Fatal("signature must not depend on key order")
	}
	canon, _, _ := CanonicalJSON(a)
	if string(canon) != `{"actually_paid":0.0105,"order_id":"o/1<x>","payment_id":123,"payment_status":"finished"}` {
		t.Fatalf("unexpected canonical form %s", canon)
	}
	ipn, err := ParseSignedIPN(secret, a, sigA)
	if err != nil || ipn.PaymentID != "123" || ipn.RawStatus != "finished" || ipn.OrderID != "o/1<x>" {
		t.Fatalf("parse failed: %v %+v", err, ipn)
	}
	if _, err := ParseSignedIPN(secret, a, ""); err == nil {
		t.Fatal("missing signature accepted")
	}
	if _, err := ParseSignedIPN([]byte("other"), a, sigA); err == nil {
		t.Fatal("bad signature accepted")
	}
	tampered := []byte(`{"payment_status":"finished","payment_id":124,"order_id":"o/1<x>","actually_paid":0.0105}`)
	if _, err := ParseSignedIPN(secret, tampered, sigA); err == nil {
		t.Fatal("tampered body accepted")
	}
}

func TestAmounts(t *testing.T) {
	if c, ok := DecimalToCents("500"); !ok || c != 50000 {
		t.Fatal(c)
	}
	if c, ok := DecimalToCents("500.5"); !ok || c != 50050 {
		t.Fatal(c)
	}
	if _, ok := DecimalToCents("1.001"); ok {
		t.Fatal("sub-cent accepted")
	}
	if CentsToDecimal(50000) != "500.00" {
		t.Fatal(CentsToDecimal(50000))
	}
	if !WithinTolerance("1.0", "0.995", 100) || WithinTolerance("1.0", "0.98", 100) {
		t.Fatal("tolerance")
	}
}

func TestMapStatus(t *testing.T) {
	for raw, want := range map[string]string{"waiting": StatusPending, "sending": StatusConfirming, "finished": StatusPaid, "partially_paid": StatusPartial, "expired": StatusExpired} {
		if got, _ := MapStatus(raw); got != want {
			t.Errorf("%s → %s, want %s", raw, got, want)
		}
	}
	if _, ok := MapStatus("bogus"); ok {
		t.Fatal("unknown status mapped")
	}
}
