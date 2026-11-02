// Package crypto groups the keyed primitives used by the platform:
// HMAC (IP hashing, recovery codes), SHA-256 token hashing, random tokens and
// versioned AES-256-GCM field encryption.
package crypto

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"crypto/sha512"
	"crypto/subtle"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"fmt"
	"strconv"
	"strings"
)

// RandomBytes returns n cryptographically secure random bytes.
func RandomBytes(n int) []byte {
	b := make([]byte, n)
	if _, err := rand.Read(b); err != nil {
		panic(fmt.Sprintf("crypto/rand failure: %v", err))
	}
	return b
}

// RandomToken returns a URL-safe base64 token carrying n bytes of entropy.
func RandomToken(n int) string {
	return base64.RawURLEncoding.EncodeToString(RandomBytes(n))
}

const passwordAlphabet = "ABCDEFGHJKLMNPQRSTUVWXYZabcdefghijkmnopqrstuvwxyz23456789"

// RandomPassword returns a random password of the given length drawn from an
// unambiguous alphabet (used for temporary one-time passwords, RM-07).
func RandomPassword(length int) string {
	out := make([]byte, length)
	max := byte(256 - (256 % len(passwordAlphabet)))
	for i := 0; i < length; {
		b := RandomBytes(1)[0]
		if b >= max {
			continue
		}
		out[i] = passwordAlphabet[int(b)%len(passwordAlphabet)]
		i++
	}
	return string(out)
}

func SHA256(data []byte) []byte {
	s := sha256.Sum256(data)
	return s[:]
}

func HMACSHA256(key, data []byte) []byte {
	m := hmac.New(sha256.New, key)
	m.Write(data)
	return m.Sum(nil)
}

func HMACSHA256Hex(key []byte, data string) string {
	return hex.EncodeToString(HMACSHA256(key, []byte(data)))
}

func HMACSHA512Hex(key, data []byte) string {
	m := hmac.New(sha512.New, key)
	m.Write(data)
	return hex.EncodeToString(m.Sum(nil))
}

// ConstantTimeEqual compares two strings without leaking timing information.
func ConstantTimeEqual(a, b string) bool {
	return subtle.ConstantTimeCompare([]byte(a), []byte(b)) == 1
}

// DecodeKey accepts a base64 (std or url) or hex encoded key and checks its size.
func DecodeKey(s string, minLen int) ([]byte, error) {
	s = strings.TrimSpace(s)
	var (
		b   []byte
		err error
	)
	for _, dec := range []func(string) ([]byte, error){
		base64.StdEncoding.DecodeString,
		base64.RawStdEncoding.DecodeString,
		base64.URLEncoding.DecodeString,
		base64.RawURLEncoding.DecodeString,
		hex.DecodeString,
	} {
		b, err = dec(s)
		if err == nil && len(b) >= minLen {
			return b, nil
		}
	}
	return nil, fmt.Errorf("key must be base64 or hex encoded and at least %d bytes", minLen)
}

// FieldEncryptor encrypts small fields with AES-256-GCM. Ciphertexts are
// prefixed with the key version ("v1:<base64>") so keys can be rotated.
type FieldEncryptor struct {
	active string
	keys   map[string]cipher.AEAD
}

// NewFieldEncryptor parses "v1:<key>,v2:<key>" — the last entry is active.
func NewFieldEncryptor(spec string) (*FieldEncryptor, error) {
	fe := &FieldEncryptor{keys: map[string]cipher.AEAD{}}
	for _, part := range strings.Split(spec, ",") {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}
		version, raw, ok := strings.Cut(part, ":")
		if !ok || !strings.HasPrefix(version, "v") {
			return nil, errors.New("FIELD_ENCRYPTION_KEYS entries must look like v1:<base64 key>")
		}
		if _, err := strconv.Atoi(version[1:]); err != nil {
			return nil, errors.New("FIELD_ENCRYPTION_KEYS version must be numeric")
		}
		key, err := DecodeKey(raw, 32)
		if err != nil {
			return nil, err
		}
		block, err := aes.NewCipher(key[:32])
		if err != nil {
			return nil, err
		}
		aead, err := cipher.NewGCM(block)
		if err != nil {
			return nil, err
		}
		fe.keys[version] = aead
		fe.active = version
	}
	if fe.active == "" {
		return nil, errors.New("FIELD_ENCRYPTION_KEYS is empty")
	}
	return fe, nil
}

func (f *FieldEncryptor) Encrypt(plain string) string {
	aead := f.keys[f.active]
	nonce := RandomBytes(aead.NonceSize())
	ct := aead.Seal(nonce, nonce, []byte(plain), []byte(f.active))
	return f.active + ":" + base64.RawStdEncoding.EncodeToString(ct)
}

func (f *FieldEncryptor) Decrypt(value string) (string, error) {
	version, raw, ok := strings.Cut(value, ":")
	if !ok {
		return "", errors.New("malformed ciphertext")
	}
	aead, ok := f.keys[version]
	if !ok {
		return "", errors.New("unknown key version")
	}
	ct, err := base64.RawStdEncoding.DecodeString(raw)
	if err != nil || len(ct) < aead.NonceSize() {
		return "", errors.New("malformed ciphertext")
	}
	plain, err := aead.Open(nil, ct[:aead.NonceSize()], ct[aead.NonceSize():], []byte(version))
	if err != nil {
		return "", errors.New("decryption failed")
	}
	return string(plain), nil
}
