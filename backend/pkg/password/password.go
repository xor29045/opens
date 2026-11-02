// Package password hashes passwords with Argon2id (PHC string format), limits
// concurrent hashing with a semaphore and checks passwords against breached
// password lists (HIBP k-anonymity with a local fallback).
package password

import (
	"context"
	"crypto/subtle"
	"encoding/base64"
	"errors"
	"fmt"
	"strings"

	"golang.org/x/crypto/argon2"

	"opensex/backend/pkg/crypto"
)

type Params struct {
	MemoryKiB   uint32
	Iterations  uint32
	Parallelism uint8
	SaltLen     int
	KeyLen      uint32
}

// DefaultParams follows the dossier (§12.6): m = 64 MiB, t = 3, p = 2.
var DefaultParams = Params{MemoryKiB: 64 * 1024, Iterations: 3, Parallelism: 2, SaltLen: 16, KeyLen: 32}

var ErrMalformedHash = errors.New("malformed password hash")

type Hasher struct {
	params Params
	sem    chan struct{}
	dummy  string
}

// NewHasher creates a hasher allowing at most maxConcurrent hash operations at
// once, which bounds memory usage (maxConcurrent × MemoryKiB).
func NewHasher(p Params, maxConcurrent int) *Hasher {
	if maxConcurrent < 1 {
		maxConcurrent = 1
	}
	h := &Hasher{params: p, sem: make(chan struct{}, maxConcurrent)}
	h.dummy = h.hashNoSem("dummy-password-for-unknown-users")
	return h
}

func (h *Hasher) acquire(ctx context.Context) error {
	select {
	case h.sem <- struct{}{}:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

func (h *Hasher) release() { <-h.sem }

func (h *Hasher) Hash(ctx context.Context, password string) (string, error) {
	if err := h.acquire(ctx); err != nil {
		return "", err
	}
	defer h.release()
	return h.hashNoSem(password), nil
}

func (h *Hasher) hashNoSem(password string) string {
	salt := crypto.RandomBytes(h.params.SaltLen)
	key := argon2.IDKey([]byte(password), salt, h.params.Iterations, h.params.MemoryKiB, h.params.Parallelism, h.params.KeyLen)
	return fmt.Sprintf("$argon2id$v=%d$m=%d,t=%d,p=%d$%s$%s",
		argon2.Version, h.params.MemoryKiB, h.params.Iterations, h.params.Parallelism,
		base64.RawStdEncoding.EncodeToString(salt), base64.RawStdEncoding.EncodeToString(key))
}

// Verify checks password against the PHC hash. needsRehash reports whether
// the hash was produced with different parameters than the current ones.
func (h *Hasher) Verify(ctx context.Context, password, encoded string) (ok bool, needsRehash bool, err error) {
	p, salt, key, err := decode(encoded)
	if err != nil {
		return false, false, err
	}
	if err := h.acquire(ctx); err != nil {
		return false, false, err
	}
	defer h.release()
	other := argon2.IDKey([]byte(password), salt, p.Iterations, p.MemoryKiB, p.Parallelism, uint32(len(key)))
	ok = subtle.ConstantTimeCompare(key, other) == 1
	needsRehash = p.MemoryKiB != h.params.MemoryKiB || p.Iterations != h.params.Iterations || p.Parallelism != h.params.Parallelism
	return ok, needsRehash, nil
}

// VerifyDummy burns the same CPU time as a real verification; it is used for
// unknown usernames so that response times do not reveal account existence.
func (h *Hasher) VerifyDummy(ctx context.Context, password string) {
	_, _, _ = h.Verify(ctx, password, h.dummy)
}

func decode(encoded string) (Params, []byte, []byte, error) {
	parts := strings.Split(encoded, "$")
	if len(parts) != 6 || parts[1] != "argon2id" {
		return Params{}, nil, nil, ErrMalformedHash
	}
	var version int
	if _, err := fmt.Sscanf(parts[2], "v=%d", &version); err != nil || version != argon2.Version {
		return Params{}, nil, nil, ErrMalformedHash
	}
	var p Params
	if _, err := fmt.Sscanf(parts[3], "m=%d,t=%d,p=%d", &p.MemoryKiB, &p.Iterations, &p.Parallelism); err != nil {
		return Params{}, nil, nil, ErrMalformedHash
	}
	salt, err := base64.RawStdEncoding.DecodeString(parts[4])
	if err != nil {
		return Params{}, nil, nil, ErrMalformedHash
	}
	key, err := base64.RawStdEncoding.DecodeString(parts[5])
	if err != nil || len(key) == 0 {
		return Params{}, nil, nil, ErrMalformedHash
	}
	return p, salt, key, nil
}
