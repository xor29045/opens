package password

import (
	"bufio"
	"context"
	"crypto/sha1" //nolint:gosec // SHA-1 is mandated by the HIBP range API, not used for security.
	_ "embed"
	"encoding/hex"
	"log/slog"
	"net/http"
	"strings"
	"time"
)

//go:embed common_passwords.txt
var commonPasswordsRaw string

var commonPasswords = func() map[string]struct{} {
	m := map[string]struct{}{}
	for _, line := range strings.Split(commonPasswordsRaw, "\n") {
		line = strings.TrimSpace(strings.ToLower(line))
		if line != "" && !strings.HasPrefix(line, "#") {
			m[line] = struct{}{}
		}
	}
	return m
}()

// BreachChecker reports whether a password is known to be compromised.
type BreachChecker interface {
	IsBreached(ctx context.Context, password string) bool
}

// LocalChecker only uses the embedded list of very common passwords.
type LocalChecker struct{}

func (LocalChecker) IsBreached(_ context.Context, password string) bool {
	_, found := commonPasswords[strings.ToLower(password)]
	return found
}

// HIBPChecker queries the Have I Been Pwned range API (k-anonymity: only the
// first 5 hex chars of the SHA-1 leave the server) and falls back to the local
// list when the API is unreachable.
type HIBPChecker struct {
	Client  *http.Client
	BaseURL string
}

func NewHIBPChecker() *HIBPChecker {
	return &HIBPChecker{Client: &http.Client{Timeout: 3 * time.Second}, BaseURL: "https://api.pwnedpasswords.com/range/"}
}

func (h *HIBPChecker) IsBreached(ctx context.Context, password string) bool {
	if (LocalChecker{}).IsBreached(ctx, password) {
		return true
	}
	sum := sha1.Sum([]byte(password)) //nolint:gosec
	hexSum := strings.ToUpper(hex.EncodeToString(sum[:]))
	prefix, suffix := hexSum[:5], hexSum[5:]
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, h.BaseURL+prefix, nil)
	if err != nil {
		return false
	}
	req.Header.Set("Add-Padding", "true")
	req.Header.Set("User-Agent", "opensex-backend")
	resp, err := h.Client.Do(req)
	if err != nil {
		slog.WarnContext(ctx, "hibp unavailable, local list used", "error", err)
		return false
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		slog.WarnContext(ctx, "hibp unexpected status, local list used", "status", resp.StatusCode)
		return false
	}
	sc := bufio.NewScanner(resp.Body)
	for sc.Scan() {
		line := sc.Text()
		hash, count, ok := strings.Cut(line, ":")
		if ok && hash == suffix && strings.TrimSpace(count) != "0" {
			return true
		}
	}
	return false
}
