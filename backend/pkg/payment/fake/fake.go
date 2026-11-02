// Package fake is an in-memory payment.Provider for tests and local
// development. IPNs use the same signature scheme as NOWPayments.
package fake

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"

	"opensex/backend/pkg/payment"
)

type Provider struct {
	mu       sync.Mutex
	Secret   []byte
	BaseURL  string
	invoices map[string]payment.InvoiceRequest
	payments map[string]*payment.PaymentStatus
	seq      int
	FailNext bool
}

func New(secret string) *Provider {
	return &Provider{
		Secret:   []byte(secret),
		BaseURL:  "https://pay.fake.test",
		invoices: map[string]payment.InvoiceRequest{},
		payments: map[string]*payment.PaymentStatus{},
	}
}

func (p *Provider) Name() string { return "fake" }

func (p *Provider) CreateInvoice(_ context.Context, req payment.InvoiceRequest) (*payment.Invoice, error) {
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.FailNext {
		p.FailNext = false
		return nil, fmt.Errorf("fake provider failure")
	}
	p.seq++
	id := fmt.Sprintf("inv_%d", p.seq)
	p.invoices[id] = req
	return &payment.Invoice{ID: id, URL: fmt.Sprintf("%s/invoice/%s", p.BaseURL, id)}, nil
}

func (p *Provider) GetPayment(_ context.Context, id string) (*payment.PaymentStatus, error) {
	p.mu.Lock()
	defer p.mu.Unlock()
	ps, ok := p.payments[id]
	if !ok {
		return nil, payment.ErrNotFound
	}
	cp := *ps
	return &cp, nil
}

// ParseIPN verifies the signature. A correctly signed IPN for a payment the
// fake provider does not know yet becomes its provider-side state, so that
// payments can be simulated in development (`ops dev ipn`).
func (p *Provider) ParseIPN(body []byte, signature string) (*payment.IPN, error) {
	ipn, err := payment.ParseSignedIPN(p.Secret, body, signature)
	if err != nil {
		return nil, err
	}
	p.mu.Lock()
	defer p.mu.Unlock()
	if _, known := p.payments[ipn.PaymentID]; !known {
		str := func(k string) string {
			if v, ok := ipn.Payload[k]; ok && v != nil {
				return fmt.Sprint(v)
			}
			return ""
		}
		p.payments[ipn.PaymentID] = &payment.PaymentStatus{PaymentID: ipn.PaymentID, InvoiceID: str("invoice_id"), OrderID: ipn.OrderID,
			RawStatus: ipn.RawStatus, PriceAmount: str("price_amount"), PriceCurrency: str("price_currency"),
			PayAmount: str("pay_amount"), ActuallyPaid: str("actually_paid"), PayCurrency: str("pay_currency")}
	}
	return ipn, nil
}

// Invoice returns the request used to create an invoice.
func (p *Provider) Invoice(id string) (payment.InvoiceRequest, bool) {
	p.mu.Lock()
	defer p.mu.Unlock()
	r, ok := p.invoices[id]
	return r, ok
}

// SetPayment records the provider-side state of a payment (what GetPayment
// will answer during counter-verification).
func (p *Provider) SetPayment(ps payment.PaymentStatus) {
	p.mu.Lock()
	defer p.mu.Unlock()
	cp := ps
	p.payments[ps.PaymentID] = &cp
}

// SignedIPN builds a signed IPN body for the given payment state.
func (p *Provider) SignedIPN(ps payment.PaymentStatus) ([]byte, string) {
	body, _ := json.Marshal(map[string]any{
		"payment_id":     json.Number(ps.PaymentID),
		"invoice_id":     ps.InvoiceID,
		"payment_status": ps.RawStatus,
		"order_id":       ps.OrderID,
		"price_amount":   json.Number(ps.PriceAmount),
		"price_currency": ps.PriceCurrency,
		"pay_amount":     json.Number(ps.PayAmount),
		"actually_paid":  json.Number(ps.ActuallyPaid),
		"pay_currency":   ps.PayCurrency,
	})
	sig, _ := payment.SignIPN(p.Secret, body)
	return body, sig
}
