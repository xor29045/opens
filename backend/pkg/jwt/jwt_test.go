package jwt

import (
	"crypto/ed25519"
	"encoding/base64"
	"encoding/json"
	"strings"
	"testing"
	"time"

	gojwt "github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
)

var roles = map[string]bool{"ADMIN": true, "MANAGER": true, "PROFESSIONAL": true, "CLIENT": true}

func newIssuer(t *testing.T, now time.Time) (*Issuer, ed25519.PrivateKey) {
	t.Helper()
	_, priv, _ := ed25519.GenerateKey(nil)
	return &Issuer{Keys: NewKeyRing("k1", priv, nil), Issuer: "https://example.test/api", Audience: "example-api", Now: func() time.Time { return now }}, priv
}

func issue(t *testing.T, i *Issuer) string {
	t.Helper()
	tok, _, err := i.Issue(IssueParams{Subject: uuid.NewString(), Role: "CLIENT", SID: uuid.NewString(), TTL: 10 * time.Minute})
	if err != nil {
		t.Fatal(err)
	}
	return tok
}

func signRaw(t *testing.T, method gojwt.SigningMethod, key any, header map[string]any, claims gojwt.Claims) string {
	t.Helper()
	tok := gojwt.NewWithClaims(method, claims)
	for k, v := range header {
		tok.Header[k] = v
	}
	s, err := tok.SignedString(key)
	if err != nil {
		t.Fatal(err)
	}
	return s
}

func baseClaims(now time.Time) *Claims {
	return &Claims{Role: "CLIENT", SID: uuid.NewString(), RegisteredClaims: gojwt.RegisteredClaims{
		Issuer: "https://example.test/api", Subject: uuid.NewString(), Audience: gojwt.ClaimStrings{"example-api"},
		IssuedAt: gojwt.NewNumericDate(now), NotBefore: gojwt.NewNumericDate(now), ExpiresAt: gojwt.NewNumericDate(now.Add(10 * time.Minute)),
	}}
}

func TestValidToken(t *testing.T) {
	now := time.Now()
	i, _ := newIssuer(t, now)
	c, err := i.Verify(issue(t, i), roles)
	if err != nil || c.Role != "CLIENT" {
		t.Fatalf("valid token rejected: %v", err)
	}
}

// Scenario 2 of §16.
func TestRejectedTokens(t *testing.T) {
	now := time.Now().Truncate(time.Second)
	i, priv := newIssuer(t, now)
	hdr := map[string]any{"kid": "k1", "typ": TokenType}
	_, foreign, _ := ed25519.GenerateKey(nil)

	cases := map[string]func() string{
		"expired": func() string {
			c := baseClaims(now.Add(-time.Hour))
			return signRaw(t, gojwt.SigningMethodEdDSA, priv, hdr, c)
		},
		"missing exp": func() string {
			c := baseClaims(now)
			c.ExpiresAt = nil
			return signRaw(t, gojwt.SigningMethodEdDSA, priv, hdr, c)
		},
		"future iat": func() string {
			c := baseClaims(now.Add(5 * time.Minute))
			return signRaw(t, gojwt.SigningMethodEdDSA, priv, hdr, c)
		},
		"future nbf": func() string {
			c := baseClaims(now)
			c.NotBefore = gojwt.NewNumericDate(now.Add(5 * time.Minute))
			return signRaw(t, gojwt.SigningMethodEdDSA, priv, hdr, c)
		},
		"bad iss": func() string {
			c := baseClaims(now)
			c.Issuer = "https://evil.test"
			return signRaw(t, gojwt.SigningMethodEdDSA, priv, hdr, c)
		},
		"bad aud": func() string {
			c := baseClaims(now)
			c.Audience = gojwt.ClaimStrings{"other"}
			return signRaw(t, gojwt.SigningMethodEdDSA, priv, hdr, c)
		},
		"bad typ": func() string {
			return signRaw(t, gojwt.SigningMethodEdDSA, priv, map[string]any{"kid": "k1", "typ": "JWT"}, baseClaims(now))
		},
		"unknown kid": func() string {
			return signRaw(t, gojwt.SigningMethodEdDSA, priv, map[string]any{"kid": "nope", "typ": TokenType}, baseClaims(now))
		},
		"foreign key": func() string {
			return signRaw(t, gojwt.SigningMethodEdDSA, foreign, hdr, baseClaims(now))
		},
		"bad role": func() string {
			c := baseClaims(now)
			c.Role = "ROOT"
			return signRaw(t, gojwt.SigningMethodEdDSA, priv, hdr, c)
		},
		"tampered signature": func() string {
			tok := issue(t, i)
			parts := strings.Split(tok, ".")
			payload, _ := base64.RawURLEncoding.DecodeString(parts[1])
			var m map[string]any
			_ = json.Unmarshal(payload, &m)
			m["role"] = "ADMIN"
			b, _ := json.Marshal(m)
			parts[1] = base64.RawURLEncoding.EncodeToString(b)
			return strings.Join(parts, ".")
		},
		"alg none": func() string {
			return signRaw(t, gojwt.SigningMethodNone, gojwt.UnsafeAllowNoneSignatureType, hdr, baseClaims(now))
		},
		"HS256 with public key": func() string {
			pub := priv.Public().(ed25519.PublicKey)
			return signRaw(t, gojwt.SigningMethodHS256, []byte(pub), hdr, baseClaims(now))
		},
		"garbage": func() string { return "not.a.jwt" },
	}
	for name, mk := range cases {
		t.Run(name, func(t *testing.T) {
			if _, err := i.Verify(mk(), roles); err == nil {
				t.Fatal("token accepted")
			}
		})
	}
}

func TestKeyFiles(t *testing.T) {
	dir := t.TempDir()
	if err := GenerateKeyFiles(dir, "2026-11"); err != nil {
		t.Fatal(err)
	}
	if err := GenerateKeyFiles(dir, "2026-11"); err == nil {
		t.Fatal("overwrote existing key")
	}
	kr, err := LoadKeyRing(dir, "2026-11")
	if err != nil || kr.ActiveKID() != "2026-11" {
		t.Fatalf("load: %v", err)
	}
}
