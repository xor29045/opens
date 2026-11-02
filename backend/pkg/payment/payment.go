// Package payment defines the PaymentProvider abstraction (§5.4) so that the
// billing domain never depends on a specific processor.
package payment

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"math/big"
	"strings"
	"time"

	"opensex/backend/pkg/crypto"
)

// Internal payment statuses (payments.status).
const (
	StatusPending    = "PENDING"
	StatusConfirming = "CONFIRMING"
	StatusPartial    = "PARTIAL"
	StatusPaid       = "PAID"
	StatusFailed     = "FAILED"
	StatusExpired    = "EXPIRED"
	StatusRefunded   = "REFUNDED"
)

var (
	ErrInvalidSignature = errors.New("invalid IPN signature")
	ErrNotFound         = errors.New("payment not found at provider")
)

type InvoiceRequest struct {
	OrderID        string
	AmountCents    int64
	Currency       string
	Description    string
	IPNCallbackURL string
	SuccessURL     string
	CancelURL      string
}

type Invoice struct {
	ID  string
	URL string
}

// PaymentStatus is the provider's view of a payment (the source of truth
// after counter-verification, RM-31).
type PaymentStatus struct {
	PaymentID     string
	InvoiceID     string
	OrderID       string
	RawStatus     string
	PriceAmount   string // fiat, decimal string
	PriceCurrency string
	PayAmount     string // crypto expected
	ActuallyPaid  string // crypto received
	PayCurrency   string
}

type IPN struct {
	PaymentID string
	RawStatus string
	OrderID   string
	EventKey  string
	Payload   map[string]any
}

type Provider interface {
	Name() string
	CreateInvoice(ctx context.Context, req InvoiceRequest) (*Invoice, error)
	GetPayment(ctx context.Context, paymentID string) (*PaymentStatus, error)
	// ParseIPN verifies the signature of a raw IPN body and decodes it.
	ParseIPN(body []byte, signature string) (*IPN, error)
}

// MapStatus converts a NOWPayments status into the internal status (§14.4).
// PARTIAL may be upgraded to PAID by the caller when within tolerance.
func MapStatus(raw string) (string, bool) {
	switch strings.ToLower(raw) {
	case "waiting":
		return StatusPending, true
	case "confirming", "confirmed", "sending":
		return StatusConfirming, true
	case "partially_paid":
		return StatusPartial, true
	case "finished":
		return StatusPaid, true
	case "failed":
		return StatusFailed, true
	case "expired":
		return StatusExpired, true
	case "refunded":
		return StatusRefunded, true
	}
	return "", false
}

// CanonicalJSON re-encodes a JSON document with sorted keys, numbers kept
// verbatim and without HTML escaping — the form signed by NOWPayments.
func CanonicalJSON(body []byte) ([]byte, map[string]any, error) {
	dec := json.NewDecoder(bytes.NewReader(body))
	dec.UseNumber()
	var m map[string]any
	if err := dec.Decode(&m); err != nil {
		return nil, nil, err
	}
	var buf bytes.Buffer
	enc := json.NewEncoder(&buf)
	enc.SetEscapeHTML(false)
	if err := enc.Encode(m); err != nil {
		return nil, nil, err
	}
	return bytes.TrimRight(buf.Bytes(), "\n"), m, nil
}

// SignIPN computes the NOWPayments signature of body.
func SignIPN(secret, body []byte) (string, error) {
	canon, _, err := CanonicalJSON(body)
	if err != nil {
		return "", err
	}
	return crypto.HMACSHA512Hex(secret, canon), nil
}

// ParseSignedIPN is the shared NOWPayments IPN verification (§14.3).
func ParseSignedIPN(secret, body []byte, signature string) (*IPN, error) {
	if signature == "" || len(secret) == 0 {
		return nil, ErrInvalidSignature
	}
	canon, m, err := CanonicalJSON(body)
	if err != nil {
		return nil, ErrInvalidSignature
	}
	expected := crypto.HMACSHA512Hex(secret, canon)
	if !crypto.ConstantTimeEqual(strings.ToLower(signature), expected) {
		return nil, ErrInvalidSignature
	}
	ipn := &IPN{Payload: m}
	ipn.PaymentID = scalar(m["payment_id"])
	ipn.RawStatus = scalar(m["payment_status"])
	ipn.OrderID = scalar(m["order_id"])
	if ipn.PaymentID == "" || ipn.RawStatus == "" {
		return nil, fmt.Errorf("IPN without payment_id or payment_status")
	}
	ipn.EventKey = ipn.PaymentID + ":" + ipn.RawStatus + ":" + scalar(m["actually_paid"]) + ":" + scalar(m["updated_at"])
	return ipn, nil
}

func scalar(v any) string {
	switch t := v.(type) {
	case nil:
		return ""
	case string:
		return t
	case json.Number:
		return t.String()
	default:
		return fmt.Sprint(t)
	}
}

// DecimalToCents converts a decimal fiat amount ("500", "500.00") to cents.
func DecimalToCents(s string) (int64, bool) {
	r, ok := new(big.Rat).SetString(strings.TrimSpace(s))
	if !ok {
		return 0, false
	}
	r.Mul(r, big.NewRat(100, 1))
	if !r.IsInt() {
		return 0, false
	}
	return r.Num().Int64(), true
}

// CentsToDecimal renders cents as a decimal string ("500.00").
func CentsToDecimal(c int64) string {
	sign := ""
	if c < 0 {
		sign, c = "-", -c
	}
	return fmt.Sprintf("%s%d.%02d", sign, c/100, c%100)
}

// WithinTolerance reports whether received ≥ expected × (1 − bps/10000).
func WithinTolerance(expected, received string, toleranceBPS int64) bool {
	e, ok1 := new(big.Rat).SetString(strings.TrimSpace(expected))
	r, ok2 := new(big.Rat).SetString(strings.TrimSpace(received))
	if !ok1 || !ok2 || e.Sign() <= 0 {
		return false
	}
	min := new(big.Rat).Mul(e, big.NewRat(10000-toleranceBPS, 10000))
	return r.Cmp(min) >= 0
}

// InvoiceTTL is how long an internal payment waits for a provider payment
// before being considered expired.
const InvoiceTTL = 24 * time.Hour
