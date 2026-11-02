// Package nowpayments implements payment.Provider for NOWPayments.
// Endpoints and payloads must be confirmed against the sandbox (T5.15).
package nowpayments

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"opensex/backend/pkg/payment"
)

type Client struct {
	BaseURL   string
	APIKey    string
	IPNSecret []byte
	HTTP      *http.Client
}

func New(baseURL, apiKey, ipnSecret string) *Client {
	return &Client{
		BaseURL:   strings.TrimRight(baseURL, "/"),
		APIKey:    apiKey,
		IPNSecret: []byte(ipnSecret),
		HTTP:      &http.Client{Timeout: 15 * time.Second},
	}
}

func (c *Client) Name() string { return "nowpayments" }

type invoiceRequest struct {
	PriceAmount      json.Number `json:"price_amount"`
	PriceCurrency    string      `json:"price_currency"`
	OrderID          string      `json:"order_id"`
	OrderDescription string      `json:"order_description"`
	IPNCallbackURL   string      `json:"ipn_callback_url"`
	SuccessURL       string      `json:"success_url,omitempty"`
	CancelURL        string      `json:"cancel_url,omitempty"`
	IsFixedRate      bool        `json:"is_fixed_rate"`
	IsFeePaidByUser  bool        `json:"is_fee_paid_by_user"`
}

type invoiceResponse struct {
	ID         json.Number `json:"id"`
	InvoiceURL string      `json:"invoice_url"`
}

func (c *Client) CreateInvoice(ctx context.Context, req payment.InvoiceRequest) (*payment.Invoice, error) {
	body, _ := json.Marshal(invoiceRequest{
		PriceAmount:      json.Number(payment.CentsToDecimal(req.AmountCents)),
		PriceCurrency:    strings.ToLower(req.Currency),
		OrderID:          req.OrderID,
		OrderDescription: req.Description,
		IPNCallbackURL:   req.IPNCallbackURL,
		SuccessURL:       req.SuccessURL,
		CancelURL:        req.CancelURL,
		IsFixedRate:      true,
	})
	var out invoiceResponse
	if err := c.do(ctx, http.MethodPost, "/invoice", body, &out); err != nil {
		return nil, err
	}
	if out.ID == "" || out.InvoiceURL == "" {
		return nil, fmt.Errorf("nowpayments: incomplete invoice response")
	}
	return &payment.Invoice{ID: out.ID.String(), URL: out.InvoiceURL}, nil
}

type paymentResponse struct {
	PaymentID     json.Number `json:"payment_id"`
	InvoiceID     json.Number `json:"invoice_id"`
	PaymentStatus string      `json:"payment_status"`
	PriceAmount   json.Number `json:"price_amount"`
	PriceCurrency string      `json:"price_currency"`
	PayAmount     json.Number `json:"pay_amount"`
	ActuallyPaid  json.Number `json:"actually_paid"`
	PayCurrency   string      `json:"pay_currency"`
	OrderID       string      `json:"order_id"`
}

func (c *Client) GetPayment(ctx context.Context, paymentID string) (*payment.PaymentStatus, error) {
	var out paymentResponse
	if err := c.do(ctx, http.MethodGet, "/payment/"+url.PathEscape(paymentID), nil, &out); err != nil {
		return nil, err
	}
	return &payment.PaymentStatus{
		PaymentID:     out.PaymentID.String(),
		InvoiceID:     out.InvoiceID.String(),
		OrderID:       out.OrderID,
		RawStatus:     out.PaymentStatus,
		PriceAmount:   out.PriceAmount.String(),
		PriceCurrency: strings.ToUpper(out.PriceCurrency),
		PayAmount:     out.PayAmount.String(),
		ActuallyPaid:  out.ActuallyPaid.String(),
		PayCurrency:   strings.ToUpper(out.PayCurrency),
	}, nil
}

func (c *Client) ParseIPN(body []byte, signature string) (*payment.IPN, error) {
	return payment.ParseSignedIPN(c.IPNSecret, body, signature)
}

func (c *Client) do(ctx context.Context, method, path string, body []byte, out any) error {
	var rdr io.Reader
	if body != nil {
		rdr = bytes.NewReader(body)
	}
	req, err := http.NewRequestWithContext(ctx, method, c.BaseURL+path, rdr)
	if err != nil {
		return err
	}
	req.Header.Set("x-api-key", c.APIKey)
	req.Header.Set("Accept", "application/json")
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	resp, err := c.HTTP.Do(req)
	if err != nil {
		return fmt.Errorf("nowpayments %s %s: %w", method, path, err)
	}
	defer resp.Body.Close()
	data, _ := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if resp.StatusCode == http.StatusNotFound {
		return payment.ErrNotFound
	}
	if resp.StatusCode >= 300 {
		return fmt.Errorf("nowpayments %s %s: status %d", method, path, resp.StatusCode)
	}
	dec := json.NewDecoder(bytes.NewReader(data))
	dec.UseNumber()
	return dec.Decode(out)
}
