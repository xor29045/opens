package integration

import (
	"context"
	"net/http"
	"testing"
	"time"

	"opensex/backend/internal/models"
)

func (e *env) refresh(cookie string) *resp {
	return e.send(req{method: http.MethodPost, path: "/api/v1/auth/refresh", cookie: cookie,
		headers: map[string]string{"Origin": origin, "X-Requested-With": "XMLHttpRequest"}})
}

// Scenario 3: refresh token rotation, reuse detection and revocations.
func TestRefreshScenarios(t *testing.T) {
	e := newEnv(t)
	c := e.register("client", nil)

	// CSRF protections.
	e.send(req{method: http.MethodPost, path: "/api/v1/auth/refresh", cookie: c.Cookie}).expect(http.StatusForbidden)
	e.send(req{method: http.MethodPost, path: "/api/v1/auth/refresh", cookie: c.Cookie,
		headers: map[string]string{"Origin": "https://evil.test", "X-Requested-With": "XMLHttpRequest"}}).expect(http.StatusForbidden)

	r1 := e.refresh(c.Cookie).expect(http.StatusOK)
	c2 := refreshCookie(r1)
	if c2 == "" || c2 == c.Cookie {
		t.Fatal("refresh must rotate the cookie")
	}
	// Concurrent reuse within the grace window: rejected without revoking the family.
	e.refresh(c.Cookie).expect(http.StatusUnauthorized)
	r2 := e.refresh(c2).expect(http.StatusOK)
	c3 := refreshCookie(r2)
	// Reuse after the grace window revokes the whole family.
	e.clock.Advance(time.Minute)
	e.refresh(c2).expect(http.StatusUnauthorized)
	e.refresh(c3).expect(http.StatusUnauthorized)
	var n int64
	e.db.Gorm.Raw(`SELECT count(*) FROM audit_logs WHERE action = 'auth.refresh.reuse_detected' AND severity = 'CRITICAL'`).Scan(&n)
	if n != 1 {
		t.Fatal("reuse must be audited as CRITICAL")
	}
	// The access token of the revoked family is denied.
	e.get("/api/v1/auth/me", r2.JSON()["access_token"].(string)).expect(http.StatusUnauthorized)

	// Expired.
	l := e.login(c.Username, c.Password).expect(200)
	e.clock.Advance(8 * 24 * time.Hour)
	e.refresh(refreshCookie(l)).expect(http.StatusUnauthorized)

	// Password change revokes the other sessions but keeps the current one.
	a := e.login(c.Username, c.Password).expect(200)
	b := e.login(c.Username, c.Password).expect(200)
	e.put("/api/v1/auth/password", a.JSON()["access_token"].(string), map[string]any{"current_password": pwd, "new_password": "another long passphrase 99"}).expect(http.StatusNoContent)
	e.refresh(refreshCookie(b)).expect(http.StatusUnauthorized)
	e.get("/api/v1/auth/me", b.JSON()["access_token"].(string)).expect(http.StatusUnauthorized)
	e.get("/api/v1/auth/me", a.JSON()["access_token"].(string)).expect(http.StatusOK)
	e.refresh(refreshCookie(a)).expect(http.StatusOK)

	// Logout revokes the current session.
	x := e.login(c.Username, "another long passphrase 99").expect(200)
	e.send(req{method: http.MethodPost, path: "/api/v1/auth/logout", cookie: refreshCookie(x), token: x.JSON()["access_token"].(string),
		headers: map[string]string{"Origin": origin, "X-Requested-With": "XMLHttpRequest"}}).expect(http.StatusNoContent)
	e.refresh(refreshCookie(x)).expect(http.StatusUnauthorized)
	e.get("/api/v1/auth/me", x.JSON()["access_token"].(string)).expect(http.StatusUnauthorized)
}

// Scenario 3 (continued): admin password reset and suspension revoke sessions.
func TestRevocationByAdmin(t *testing.T) {
	e := newEnv(t)
	adm := e.adminAccount()
	m, slug := e.verifiedManager(adm.Token)
	p, _ := e.publishedProfile(m, slug)
	e.subscribeAndPay(m)

	r := e.post("/api/v1/admin/users/"+m.ID+"/password-reset", adm.Token, map[string]any{"reason": "rappel au numéro de vérification"}).expect(200)
	temp := r.JSON()["temporary_password"].(string)
	if len(temp) != 20 {
		t.Fatal("temporary password must be 20 chars")
	}
	e.refresh(m.Cookie).expect(http.StatusUnauthorized)
	e.get("/api/v1/auth/me", m.Token).expect(http.StatusUnauthorized)
	l := e.login(m.Username, temp).expect(200)
	tok := l.JSON()["access_token"].(string)
	e.get("/api/v1/manager/establishment", tok).expect(http.StatusForbidden) // must change password
	e.get("/api/v1/auth/me", tok).expect(200)
	e.put("/api/v1/auth/password", tok, map[string]any{"current_password": temp, "new_password": "new manager passphrase 1"}).expect(204)
	e.get("/api/v1/manager/establishment", tok).expect(200)

	// Admin cannot reset professionals or clients.
	e.post("/api/v1/admin/users/"+p.ID+"/password-reset", adm.Token, map[string]any{"reason": "x"}).expect(http.StatusForbidden)

	// Suspending the manager unpublishes everything and cuts the professionals (RM-63).
	if e.get("/api/v1/profiles", "").JSON()["total"].(float64) != 1 {
		t.Fatal("profile should be published")
	}
	e.post("/api/v1/admin/users/"+m.ID+"/suspend", adm.Token, map[string]any{"reason": "contrôle"}).expect(204)
	e.get("/api/v1/manager/establishment", tok).expect(http.StatusUnauthorized)
	e.get("/api/v1/professional/me", p.Token).expect(http.StatusUnauthorized)
	e.refresh(p.Cookie).expect(http.StatusUnauthorized)
	if e.get("/api/v1/profiles", "").JSON()["total"].(float64) != 0 {
		t.Fatal("profiles must be unpublished")
	}
	e.post("/api/v1/admin/users/"+m.ID+"/restore", adm.Token, map[string]any{}).expect(204)
	if e.get("/api/v1/profiles", "").JSON()["total"].(float64) != 1 {
		t.Fatal("profiles must be republished after restore")
	}
	pl := e.login(p.Username, p.Password).expect(200)
	e.get("/api/v1/professional/me", pl.JSON()["access_token"].(string)).expect(200)
}

// Scenario 4: recovery codes.
func TestRecoveryCodes(t *testing.T) {
	e := newEnv(t)
	c := e.register("client", nil)
	other := e.register("client", nil)
	if len(c.Codes) != 10 {
		t.Fatal("10 codes expected")
	}
	st := e.get("/api/v1/auth/recovery-codes/status", c.Token).expect(200).JSON()
	if st["remaining"].(float64) != 10 || st["confirmed"].(bool) {
		t.Fatalf("status %v", st)
	}
	e.post("/api/v1/auth/recovery-codes/confirm", c.Token, nil).expect(204)

	bad := func(code string) *resp {
		return e.post("/api/v1/auth/password/recover", "", map[string]any{"username": c.Username, "recovery_code": code, "new_password": "recovered passphrase 1"})
	}
	r1 := bad("AAAA-BBBB-CCCC-DDDD").expect(401)
	r2 := bad(other.Codes[0]).expect(401)
	r3 := e.post("/api/v1/auth/password/recover", "", map[string]any{"username": "nobody_here", "recovery_code": c.Codes[0], "new_password": "recovered passphrase 1"}).expect(401)
	if string(r1.JSON()["detail"].(string)) != r2.JSON()["detail"] || r2.JSON()["code"] != r3.JSON()["code"] {
		t.Fatal("errors must be identical")
	}
	// Valid code (lower case, without dashes) works once and revokes sessions.
	code := c.Codes[1]
	e.post("/api/v1/auth/password/recover", "", map[string]any{"username": c.Username, "recovery_code": code, "new_password": "recovered passphrase 1"}).expect(204)
	e.refresh(c.Cookie).expect(401)
	e.post("/api/v1/auth/password/recover", "", map[string]any{"username": c.Username, "recovery_code": code, "new_password": "recovered passphrase 2"}).expect(401)
	e.login(c.Username, "recovered passphrase 1").expect(200)

	// The codes never appear again.
	me := e.login(c.Username, "recovered passphrase 1").JSON()
	if _, ok := me["recovery_codes"]; ok {
		t.Fatal("codes leaked after the initial display")
	}
	// Blocked after 10 failures for 24 h.
	for i := 0; i < 8; i++ {
		bad("ZZZZ-ZZZZ-ZZZZ-ZZZZ")
	}
	r := bad(c.Codes[2])
	if r.Code != http.StatusTooManyRequests {
		t.Fatalf("expected block, got %d", r.Code)
	}
}

// Scenario 10: login rate limit and progressive lockout.
func TestLoginRateLimitAndLockout(t *testing.T) {
	e := newEnv(t, withRateLimit())
	c := e.register("client", nil)
	other := e.register("client", nil)
	for i := 0; i < 5; i++ {
		e.login(c.Username, "wrong password value").expect(401)
	}
	r := e.login(c.Username, "wrong password value").expect(429)
	if r.Header.Get("Retry-After") == "" {
		t.Fatal("Retry-After expected")
	}
	e.login(other.Username, other.Password).expect(200)

	// Progressive lockout after 10 failures (rate limiter cleared to isolate it).
	e.mr.FlushAll()
	e.db.Gorm.Exec(`UPDATE users SET failed_login_count = 9 WHERE id = ?`, c.ID)
	e.login(c.Username, "wrong password value").expect(401)
	e.login(c.Username, c.Password).expect(429)
	e.clock.Advance(61 * time.Second)
	e.login(c.Username, c.Password).expect(200)

	// X-Forwarded-For from an untrusted client is ignored.
	e.mr.FlushAll()
	for i := 0; i < 20; i++ {
		e.send(req{method: "POST", path: "/api/v1/auth/login", body: map[string]any{"username": uniq("u"), "password": "x"},
			headers: map[string]string{"X-Forwarded-For": "10.0.0." + string(rune('0'+i%10))}})
	}
	e.send(req{method: "POST", path: "/api/v1/auth/login", body: map[string]any{"username": "u_last", "password": "x"},
		headers: map[string]string{"X-Forwarded-For": "10.9.9.9"}}).expect(429)
}

// Scenario 12: bootstrap admin.
func TestBootstrapAdmin(t *testing.T) {
	e := newEnv(t)
	ctx := context.Background()
	e.cfg.AdminBootstrapUsername, e.cfg.AdminBootstrapPassword = "platform_root", "short"
	if err := e.svc.Bootstrap.EnsureBootstrapAdmins(ctx); err == nil {
		t.Fatal("weak password accepted")
	}
	e.cfg.AdminBootstrapPassword = "a very long bootstrap passphrase"
	errs := make(chan error, 2)
	for i := 0; i < 2; i++ {
		go func() { errs <- e.svc.Bootstrap.EnsureBootstrapAdmins(ctx) }()
	}
	for i := 0; i < 2; i++ {
		if err := <-errs; err != nil {
			t.Fatal(err)
		}
	}
	var n int64
	e.db.Gorm.Raw(`SELECT count(*) FROM users u JOIN user_roles ur ON ur.user_id = u.id WHERE ur.role_id = 1`).Scan(&n)
	if n != 1 {
		t.Fatalf("%d admins created", n)
	}
	if err := e.svc.Bootstrap.EnsureBootstrapAdmins(ctx); err != nil {
		t.Fatal("bootstrap must be idempotent")
	}
	// A non-admin username cannot be promoted.
	c := e.register("client", nil)
	e.cfg.AdminBootstrapUsername = c.Username
	if err := e.svc.Bootstrap.EnsureBootstrapAdmins(ctx); err == nil {
		t.Fatal("existing non-admin promoted")
	}
	e.cfg.AdminBootstrapUsername = "platform_root"

	// Admin login only on the admin endpoint; forced password change.
	e.login("platform_root", "a very long bootstrap passphrase").expect(401)
	r := e.post("/api/v1/auth/admin/login", "", map[string]any{"username": "platform_root", "password": "a very long bootstrap passphrase"}).expect(200)
	tok := r.JSON()["access_token"].(string)
	if r.JSON()["user"].(map[string]any)["must_change_password"] != true {
		t.Fatal("must_change_password expected")
	}
	e.get("/api/v1/admin/stats", tok).expect(403)
	e.post("/api/v1/auth/admin/login", "", map[string]any{"username": c.Username, "password": pwd}).expect(401)
	e.put("/api/v1/auth/password", tok, map[string]any{"current_password": "a very long bootstrap passphrase", "new_password": "short pass 1"}).expect(422)
	e.put("/api/v1/auth/password", tok, map[string]any{"current_password": "a very long bootstrap passphrase", "new_password": "a sufficiently long admin pass"}).expect(204)
	e.get("/api/v1/admin/stats", tok).expect(200)

	// An admin cannot act on another admin.
	var adminID string
	e.db.Gorm.Raw(`SELECT id::text FROM users WHERE username = 'platform_root'`).Scan(&adminID)
	other := e.adminAccount()
	e.post("/api/v1/admin/users/"+adminID+"/suspend", other.Token, map[string]any{"reason": "x"}).expect(403)
	e.post("/api/v1/admin/users/"+adminID+"/password-reset", other.Token, map[string]any{"reason": "x"}).expect(403)
	e.del("/api/v1/admin/users/"+adminID, other.Token, map[string]any{"reason": "x"}).expect(403)

	// Admin access token TTL is 5 minutes.
	e.clock.Advance(6 * time.Minute)
	e.get("/api/v1/admin/stats", tok).expect(401)
	_ = models.RoleAdmin
}
