// Package integration runs the HTTP API against a real PostgreSQL database
// (TEST_DATABASE_URL), an in-memory Redis (miniredis), in-memory photo
// storage and the fake payment provider. Run with `make test-integration`.
package integration

import (
	"bytes"
	"context"
	"crypto/ed25519"
	"encoding/json"
	"fmt"
	"image"
	"image/color"
	"image/jpeg"
	"io"
	"log/slog"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/gin-gonic/gin"
	"github.com/redis/go-redis/v9"

	"opensex/backend/internal/config"
	"opensex/backend/internal/database"
	"opensex/backend/internal/repositories"
	"opensex/backend/internal/routes"
	"opensex/backend/internal/services"
	"opensex/backend/pkg/clock"
	"opensex/backend/pkg/crypto"
	"opensex/backend/pkg/jwt"
	"opensex/backend/pkg/password"
	"opensex/backend/pkg/payment"
	"opensex/backend/pkg/payment/fake"
	"opensex/backend/pkg/ratelimit"
	"opensex/backend/pkg/storage"
)

const origin = "https://opensex.test"

var (
	dbOnce   sync.Once
	sharedDB *database.DB
	dbErr    error
)

func testDB(t *testing.T) *database.DB {
	url := os.Getenv("TEST_DATABASE_URL")
	if url == "" {
		t.Skip("TEST_DATABASE_URL not set")
	}
	dbOnce.Do(func() {
		if dbErr = database.MigrateUp(url); dbErr != nil {
			return
		}
		sharedDB, dbErr = database.Open(database.Options{URL: url, MaxConns: 10})
	})
	if dbErr != nil {
		t.Fatalf("database: %v", dbErr)
	}
	return sharedDB
}

func resetDB(t *testing.T, db *database.DB) {
	t.Helper()
	err := db.Gorm.Exec(`TRUNCATE audit_logs, payment_provider_events, invoices, payments, subscriptions, favorites,
		professional_categories, profile_rates, profile_contacts, profile_photos, professional_profiles,
		establishment_contacts, establishments, recovery_codes, refresh_tokens, user_roles, users CASCADE`).Error
	if err != nil {
		t.Fatal(err)
	}
	db.Gorm.Exec(`UPDATE subscription_plans SET max_active_profiles = NULL, max_pending_profiles = 20`)
}

type env struct {
	t      *testing.T
	cfg    *config.Config
	svc    *services.Services
	engine *gin.Engine
	routes []routes.RouteSpec
	clock  *clock.Fake
	pay    *fake.Provider
	store  *storage.Memory
	mr     *miniredis.Miniredis
	db     *database.DB
}

type envOpt func(*config.Config, *routes.Deps)

func withRateLimit() envOpt {
	return func(_ *config.Config, d *routes.Deps) { d.RateLimitDisabled = false }
}

func newEnv(t *testing.T, opts ...envOpt) *env {
	t.Helper()
	gin.SetMode(gin.TestMode)
	db := testDB(t)
	resetDB(t, db)
	mr := miniredis.RunT(t)
	rdb := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	_, priv, _ := ed25519.GenerateKey(nil)
	cfg := &config.Config{
		Env: config.EnvTest, BaseURL: origin, TermsVersion: "test", JWTIssuer: origin + "/api", JWTAudience: "opensex.test-api",
		AccessTTL: 10 * time.Minute, AdminAccessTTL: 5 * time.Minute, RefreshTTL: 7 * 24 * time.Hour, RefreshAbsTTL: 30 * 24 * time.Hour,
		AdminRefreshTTL: 8 * time.Hour, AdminRefreshAbsTTL: 12 * time.Hour, CookieSecure: true,
		IPHashKey: crypto.RandomBytes(32), RecoveryCodeKey: crypto.RandomBytes(32),
		CORSAllowedOrigins: []string{origin}, UnderpayToleranceBPS: 100,
		NOWPaymentsIPNCallbackURL: origin + "/api/v1/webhooks/nowpayments",
		NOWPaymentsIPNSecret:      "ipn-secret",
		PlatformLegalName:         "Éditeur SRL", PlatformCompanyNumber: "0403.170.701",
	}
	enc, _ := crypto.NewFieldEncryptor("v1:" + strings.Repeat("A", 43) + "=")
	clk := clock.NewFake(time.Date(2026, 11, 2, 12, 0, 0, 0, time.UTC))
	pay := fake.New("ipn-secret")
	store := storage.NewMemory()
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	limiter := ratelimit.New(rdb)
	svc := services.New(services.Deps{
		Cfg: cfg, DB: db, Repos: repositories.New(db), Clock: clk,
		Hasher: password.NewHasher(password.Params{MemoryKiB: 1024, Iterations: 1, Parallelism: 1, SaltLen: 16, KeyLen: 32}, 4),
		Breach: password.LocalChecker{}, JWT: &jwt.Issuer{Keys: jwt.NewKeyRing("test", priv, nil), Issuer: cfg.JWTIssuer, Audience: cfg.JWTAudience, Now: clk.Now},
		Redis: rdb, Limiter: limiter, Storage: store, Payment: pay, Encryptor: enc, Logger: logger,
	})
	deps := routes.Deps{Cfg: cfg, Svc: svc, Logger: logger, Limiter: limiter, RateLimitDisabled: true}
	for _, o := range opts {
		o(cfg, &deps)
	}
	engine, specs := routes.New(deps)
	return &env{t: t, cfg: cfg, svc: svc, engine: engine, routes: specs, clock: clk, pay: pay, store: store, mr: mr, db: db}
}

// ---------------------------------------------------------------- HTTP helpers

type resp struct {
	t       *testing.T
	Code    int
	Body    []byte
	Header  http.Header
	Cookies []*http.Cookie
}

func (r *resp) JSON() map[string]any {
	var m map[string]any
	if err := json.Unmarshal(r.Body, &m); err != nil {
		r.t.Fatalf("invalid JSON (%d): %s", r.Code, r.Body)
	}
	return m
}

func (r *resp) expect(code int) *resp {
	r.t.Helper()
	if r.Code != code {
		r.t.Fatalf("expected %d, got %d: %s", code, r.Code, r.Body)
	}
	return r
}

func (r *resp) problemCode() string {
	m := map[string]any{}
	_ = json.Unmarshal(r.Body, &m)
	s, _ := m["code"].(string)
	return s
}

type req struct {
	method, path string
	body         any
	token        string
	cookie       string
	headers      map[string]string
	raw          []byte
	contentType  string
	remoteAddr   string
}

func (e *env) send(r req) *resp {
	e.t.Helper()
	var rdr io.Reader
	ct := r.contentType
	switch {
	case r.raw != nil:
		rdr = bytes.NewReader(r.raw)
	case r.body != nil:
		b, _ := json.Marshal(r.body)
		rdr = bytes.NewReader(b)
		ct = "application/json"
	}
	hr := httptest.NewRequest(r.method, r.path, rdr)
	if ct != "" {
		hr.Header.Set("Content-Type", ct)
	}
	if r.token != "" {
		hr.Header.Set("Authorization", "Bearer "+r.token)
	}
	if r.cookie != "" {
		hr.AddCookie(&http.Cookie{Name: "__Host-rt", Value: r.cookie})
	}
	for k, v := range r.headers {
		hr.Header.Set(k, v)
	}
	if r.remoteAddr != "" {
		hr.RemoteAddr = r.remoteAddr
	}
	w := httptest.NewRecorder()
	e.engine.ServeHTTP(w, hr)
	res := w.Result()
	return &resp{t: e.t, Code: w.Code, Body: w.Body.Bytes(), Header: w.Header(), Cookies: res.Cookies()}
}

func (e *env) get(path, token string) *resp {
	return e.send(req{method: http.MethodGet, path: path, token: token})
}

func (e *env) post(path, token string, body any) *resp {
	return e.send(req{method: http.MethodPost, path: path, token: token, body: body})
}

func (e *env) put(path, token string, body any) *resp {
	return e.send(req{method: http.MethodPut, path: path, token: token, body: body})
}

func (e *env) patch(path, token string, body any) *resp {
	return e.send(req{method: http.MethodPatch, path: path, token: token, body: body})
}

func (e *env) del(path, token string, body any) *resp {
	return e.send(req{method: http.MethodDelete, path: path, token: token, body: body})
}

func refreshCookie(r *resp) string {
	for _, c := range r.Cookies {
		if c.Name == "__Host-rt" {
			return c.Value
		}
	}
	return ""
}

// ---------------------------------------------------------------- domain helpers

type account struct {
	ID       string
	Username string
	Password string
	Token    string
	Cookie   string
	Codes    []string
}

const pwd = "correct horse battery staple 42"

var seq int

func uniq(prefix string) string {
	seq++
	return fmt.Sprintf("%s%d", prefix, seq)
}

func (e *env) register(kind string, extra map[string]any) account {
	e.t.Helper()
	name := uniq(kind)
	body := map[string]any{"username": name, "password": pwd, "accept_terms": true}
	if kind == "manager" {
		body["accept_cgv"], body["accept_dpa"] = true, true
	}
	for k, v := range extra {
		body[k] = v
	}
	r := e.post("/api/v1/auth/register/"+kind, "", body).expect(http.StatusCreated)
	m := r.JSON()
	acc := account{Username: name, Password: pwd, Token: m["access_token"].(string), Cookie: refreshCookie(r)}
	acc.ID = m["user"].(map[string]any)["id"].(string)
	for _, c := range m["recovery_codes"].([]any) {
		acc.Codes = append(acc.Codes, c.(string))
	}
	return acc
}

func (e *env) login(username, password string) *resp {
	return e.post("/api/v1/auth/login", "", map[string]any{"username": username, "password": password})
}

func regionCity(e *env) (string, string) {
	var region, city struct{ ID string }
	e.db.Gorm.Raw(`SELECT id::text AS id FROM regions WHERE slug = 'bruxelles-capitale'`).Scan(&region)
	e.db.Gorm.Raw(`SELECT id::text AS id FROM regions WHERE slug = 'ixelles'`).Scan(&city)
	return region.ID, city.ID
}

// readyManager registers a manager and fills the establishment page (no
// verification step anymore). It returns the manager and the establishment slug.
func (e *env) readyManager() (account, string) {
	e.t.Helper()
	m := e.register("manager", nil)
	region, city := regionCity(e)
	est := e.put("/api/v1/manager/establishment", m.Token, map[string]any{
		"display_name": uniq("Maison "), "legal_name": "Maison SRL",
		"description": "Maison", "region_id": region, "city_id": city, "address": "Rue X 1, 1050 Ixelles", "address_public": false,
		"opening_hours": map[string]any{"mon": []any{map[string]any{"open": "10:00", "close": "02:00"}}},
	}).expect(http.StatusOK).JSON()
	e.put("/api/v1/manager/establishment/contacts", m.Token, map[string]any{"contacts": []any{
		map[string]any{"type": "PHONE", "value": "+32 470 00 00 01", "visibility": "HIDDEN", "is_verification_phone": true},
		map[string]any{"type": "PHONE", "value": "+32 470 00 00 02", "visibility": "PUBLIC"},
		map[string]any{"type": "WHATSAPP", "value": "+32 470 00 00 03", "visibility": "CLIENTS"},
	}}).expect(http.StatusOK)
	return m, est["slug"].(string)
}

// adminAccount creates an admin through the bootstrap and changes its password.
func (e *env) adminAccount() account {
	e.t.Helper()
	name := uniq("root")
	e.cfg.AdminBootstrapUsername, e.cfg.AdminBootstrapPassword = name, "initial admin passphrase 2026!"
	if err := e.svc.Bootstrap.EnsureBootstrapAdmins(context.Background()); err != nil {
		e.t.Fatal(err)
	}
	r := e.post("/api/v1/auth/admin/login", "", map[string]any{"username": name, "password": "initial admin passphrase 2026!"}).expect(http.StatusOK)
	tok := r.JSON()["access_token"].(string)
	e.put("/api/v1/auth/password", tok, map[string]any{"current_password": "initial admin passphrase 2026!", "new_password": "a brand new admin passphrase!"}).expect(http.StatusNoContent)
	return account{Username: name, Password: "a brand new admin passphrase!", Token: tok}
}

func testJPEG() []byte {
	img := image.NewRGBA(image.Rect(0, 0, 40, 30))
	for x := 0; x < 40; x++ {
		img.Set(x, x%30, color.RGBA{10, 200, 10, 255})
	}
	var b bytes.Buffer
	_ = jpeg.Encode(&b, img, nil)
	return b.Bytes()
}

func (e *env) upload(path, token string, data []byte) *resp {
	var buf bytes.Buffer
	w := multipart.NewWriter(&buf)
	fw, _ := w.CreateFormFile("file", "photo.jpg")
	_, _ = fw.Write(data)
	_ = w.Close()
	return e.send(req{method: http.MethodPost, path: path, token: token, raw: buf.Bytes(), contentType: w.FormDataContentType()})
}

// subscribeAndPay subscribes the manager's establishment and simulates a
// confirmed payment through a signed IPN.
// subscribeAndPay subscribes the manager's establishment and simulates a
// confirmed payment. It does nothing when the establishment is already entitled.
func (e *env) subscribeAndPay(m account) {
	e.t.Helper()
	if e.get("/api/v1/billing/subscription", m.Token).expect(http.StatusOK).JSON()["entitled"] == true {
		return
	}
	p := e.post("/api/v1/billing/subscription", m.Token, map[string]any{}).expect(http.StatusCreated).JSON()
	e.payIPN(p["id"].(string), "finished", "1.0", "1.0")
}

var providerPaymentSeq = 1000

func (e *env) payIPN(paymentID, status, expected, received string) *resp {
	e.t.Helper()
	var row struct{ ProviderInvoiceID, ProviderPaymentID *string }
	e.db.Gorm.Raw(`SELECT provider_invoice_id, provider_payment_id FROM payments WHERE id = ?`, paymentID).Scan(&row)
	pid := ""
	if row.ProviderPaymentID != nil {
		pid = *row.ProviderPaymentID
	} else {
		providerPaymentSeq++
		pid = fmt.Sprint(providerPaymentSeq)
	}
	inv := ""
	if row.ProviderInvoiceID != nil {
		inv = *row.ProviderInvoiceID
	}
	ps := payment.PaymentStatus{PaymentID: pid, InvoiceID: inv, OrderID: paymentID, RawStatus: status, PriceAmount: "500",
		PriceCurrency: "eur", PayAmount: expected, ActuallyPaid: received, PayCurrency: "btc"}
	e.pay.SetPayment(ps)
	body, sig := e.pay.SignedIPN(ps)
	return e.send(req{method: http.MethodPost, path: "/api/v1/webhooks/nowpayments", raw: body, contentType: "application/json",
		headers: map[string]string{"x-nowpayments-sig": sig}})
}

// publishedProfile creates a fully publishable profile for the manager:
// professional registration, attestation, photo, activation.
func (e *env) publishedProfile(m account, estSlug string) (account, string) {
	e.t.Helper()
	p := e.register("professional", map[string]any{"establishment_slug": estSlug, "display_name": "Lola"})
	me := e.get("/api/v1/professional/me", p.Token).expect(http.StatusOK).JSON()
	id := me["id"].(string)
	e.upload("/api/v1/professional/me/photos", p.Token, testJPEG()).expect(http.StatusCreated)
	e.put("/api/v1/professional/me/rates", p.Token, map[string]any{"rates": []any{map[string]any{"label": "1 heure", "duration_minutes": 60, "amount_cents": 15000}}}).expect(http.StatusOK)
	e.put("/api/v1/professional/me/contacts", p.Token, map[string]any{"contacts": []any{
		map[string]any{"type": "PHONE", "value": "+32470111111", "visibility": "PUBLIC"},
		map[string]any{"type": "WHATSAPP", "value": "+32470222222", "visibility": "CLIENTS"},
		map[string]any{"type": "TELEGRAM", "value": "@lola_private", "visibility": "HIDDEN"},
	}}).expect(http.StatusOK)
	e.put("/api/v1/manager/profiles/"+id+"/attestation", m.Token, map[string]any{"birth_date": "1995-03-14", "attested": true}).expect(http.StatusOK)
	// Attested + paid subscription → in service without a manual step (ADR-020).
	e.subscribeAndPay(m)
	return p, id
}

func paymentStatus(orderID, status string) payment.PaymentStatus {
	providerPaymentSeq++
	return payment.PaymentStatus{PaymentID: fmt.Sprint(providerPaymentSeq), OrderID: orderID, RawStatus: status,
		PriceAmount: "500", PriceCurrency: "eur", PayAmount: "1.0", ActuallyPaid: "1.0", PayCurrency: "btc"}
}
