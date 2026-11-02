// Package jwt issues and verifies EdDSA (Ed25519) access tokens using a key
// ring indexed by "kid" (§12). Only the EdDSA algorithm is accepted.
package jwt

import (
	"crypto/ed25519"
	"crypto/x509"
	"encoding/pem"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	gojwt "github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
)

const (
	TokenType = "at+jwt"
	leeway    = 30 * time.Second
)

var ErrInvalidToken = errors.New("invalid token")

type Claims struct {
	Role string `json:"role"`
	SID  string `json:"sid"`
	gojwt.RegisteredClaims
}

type KeyRing struct {
	activeKID  string
	private    ed25519.PrivateKey
	publicKeys map[string]ed25519.PublicKey
}

// NewKeyRing builds a ring from an active private key and extra verification keys.
func NewKeyRing(activeKID string, active ed25519.PrivateKey, verify map[string]ed25519.PublicKey) *KeyRing {
	kr := &KeyRing{activeKID: activeKID, private: active, publicKeys: map[string]ed25519.PublicKey{}}
	for k, v := range verify {
		kr.publicKeys[k] = v
	}
	if active != nil {
		kr.publicKeys[activeKID] = active.Public().(ed25519.PublicKey)
	}
	return kr
}

func (k *KeyRing) ActiveKID() string { return k.activeKID }

func (k *KeyRing) KIDs() []string {
	out := make([]string, 0, len(k.publicKeys))
	for kid := range k.publicKeys {
		out = append(out, kid)
	}
	sort.Strings(out)
	return out
}

// LoadKeyRing reads "<kid>.pem" (PKCS#8 Ed25519 private keys) and
// "<kid>.pub.pem" (PKIX public keys, verification only) from dir.
func LoadKeyRing(dir, activeKID string) (*KeyRing, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, fmt.Errorf("read JWT keys dir: %w", err)
	}
	privs := map[string]ed25519.PrivateKey{}
	pubs := map[string]ed25519.PublicKey{}
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		name := e.Name()
		data, err := os.ReadFile(filepath.Join(dir, name))
		if err != nil {
			return nil, err
		}
		switch {
		case strings.HasSuffix(name, ".pub.pem"):
			pub, err := ParsePublicPEM(data)
			if err != nil {
				return nil, fmt.Errorf("%s: %w", name, err)
			}
			pubs[strings.TrimSuffix(name, ".pub.pem")] = pub
		case strings.HasSuffix(name, ".pem"):
			priv, err := ParsePrivatePEM(data)
			if err != nil {
				return nil, fmt.Errorf("%s: %w", name, err)
			}
			privs[strings.TrimSuffix(name, ".pem")] = priv
		}
	}
	active, ok := privs[activeKID]
	if !ok {
		return nil, fmt.Errorf("active JWT key %q not found in %s", activeKID, dir)
	}
	for kid, p := range privs {
		pubs[kid] = p.Public().(ed25519.PublicKey)
	}
	return NewKeyRing(activeKID, active, pubs), nil
}

func ParsePrivatePEM(data []byte) (ed25519.PrivateKey, error) {
	block, _ := pem.Decode(data)
	if block == nil {
		return nil, errors.New("no PEM block")
	}
	k, err := x509.ParsePKCS8PrivateKey(block.Bytes)
	if err != nil {
		return nil, err
	}
	priv, ok := k.(ed25519.PrivateKey)
	if !ok {
		return nil, errors.New("not an Ed25519 private key")
	}
	return priv, nil
}

func ParsePublicPEM(data []byte) (ed25519.PublicKey, error) {
	block, _ := pem.Decode(data)
	if block == nil {
		return nil, errors.New("no PEM block")
	}
	k, err := x509.ParsePKIXPublicKey(block.Bytes)
	if err != nil {
		return nil, err
	}
	pub, ok := k.(ed25519.PublicKey)
	if !ok {
		return nil, errors.New("not an Ed25519 public key")
	}
	return pub, nil
}

// GenerateKeyFiles writes a new key pair "<kid>.pem" and "<kid>.pub.pem" into dir.
func GenerateKeyFiles(dir, kid string) error {
	pub, priv, err := ed25519.GenerateKey(nil)
	if err != nil {
		return err
	}
	privDER, err := x509.MarshalPKCS8PrivateKey(priv)
	if err != nil {
		return err
	}
	pubDER, err := x509.MarshalPKIXPublicKey(pub)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return err
	}
	privPath := filepath.Join(dir, kid+".pem")
	if _, err := os.Stat(privPath); err == nil {
		return fmt.Errorf("key %s already exists", privPath)
	}
	if err := os.WriteFile(privPath, pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: privDER}), 0o600); err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(dir, kid+".pub.pem"), pem.EncodeToMemory(&pem.Block{Type: "PUBLIC KEY", Bytes: pubDER}), 0o644)
}

type Issuer struct {
	Keys     *KeyRing
	Issuer   string
	Audience string
	Now      func() time.Time
}

type IssueParams struct {
	Subject string
	Role    string
	SID     string
	TTL     time.Duration
}

// Issue signs a new access token. It returns the token and its expiry.
func (i *Issuer) Issue(p IssueParams) (string, time.Time, error) {
	if i.Keys.private == nil {
		return "", time.Time{}, errors.New("no active signing key")
	}
	now := i.now().Truncate(time.Second)
	exp := now.Add(p.TTL)
	claims := Claims{
		Role: p.Role,
		SID:  p.SID,
		RegisteredClaims: gojwt.RegisteredClaims{
			Issuer:    i.Issuer,
			Subject:   p.Subject,
			Audience:  gojwt.ClaimStrings{i.Audience},
			ExpiresAt: gojwt.NewNumericDate(exp),
			NotBefore: gojwt.NewNumericDate(now),
			IssuedAt:  gojwt.NewNumericDate(now),
			ID:        uuid.NewString(),
		},
	}
	tok := gojwt.NewWithClaims(gojwt.SigningMethodEdDSA, claims)
	tok.Header["kid"] = i.Keys.activeKID
	tok.Header["typ"] = TokenType
	s, err := tok.SignedString(i.Keys.private)
	return s, exp, err
}

func (i *Issuer) now() time.Time {
	if i.Now != nil {
		return i.Now()
	}
	return time.Now()
}

// Verify validates a token following §12.3; any failure returns ErrInvalidToken
// wrapped with the internal reason (to be logged, never returned to clients).
func (i *Issuer) Verify(token string, allowedRoles map[string]bool) (*Claims, error) {
	claims := &Claims{}
	parser := gojwt.NewParser(
		gojwt.WithValidMethods([]string{gojwt.SigningMethodEdDSA.Alg()}),
		gojwt.WithIssuer(i.Issuer),
		gojwt.WithAudience(i.Audience),
		gojwt.WithExpirationRequired(),
		gojwt.WithIssuedAt(),
		gojwt.WithLeeway(leeway),
		gojwt.WithTimeFunc(i.now),
	)
	_, err := parser.ParseWithClaims(token, claims, func(t *gojwt.Token) (any, error) {
		if typ, _ := t.Header["typ"].(string); typ != TokenType {
			return nil, errors.New("bad typ")
		}
		kid, _ := t.Header["kid"].(string)
		key, ok := i.Keys.publicKeys[kid]
		if !ok {
			return nil, errors.New("unknown kid")
		}
		return key, nil
	})
	if err != nil {
		return nil, fmt.Errorf("%w: %v", ErrInvalidToken, err)
	}
	if claims.IssuedAt == nil {
		return nil, fmt.Errorf("%w: missing iat", ErrInvalidToken)
	}
	if _, err := uuid.Parse(claims.Subject); err != nil {
		return nil, fmt.Errorf("%w: bad sub", ErrInvalidToken)
	}
	if _, err := uuid.Parse(claims.SID); err != nil {
		return nil, fmt.Errorf("%w: bad sid", ErrInvalidToken)
	}
	if !allowedRoles[claims.Role] {
		return nil, fmt.Errorf("%w: bad role", ErrInvalidToken)
	}
	return claims, nil
}
