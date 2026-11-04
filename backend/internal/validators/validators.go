// Package validators holds the domain-specific input checks (§5.1): username,
// password policy, E.164 phones, social network URLs, opening hours, slugs.
package validators

import (
	"encoding/json"
	"fmt"
	"net/url"
	"regexp"
	"strings"
	"unicode"
	"unicode/utf8"

	"golang.org/x/text/runes"
	"golang.org/x/text/transform"
	"golang.org/x/text/unicode/norm"
)

var (
	usernameRe = regexp.MustCompile(`^[a-zA-Z0-9_.-]{3,30}$`)
	e164Re     = regexp.MustCompile(`^\+[1-9][0-9]{6,14}$`)
	telegramRe = regexp.MustCompile(`^[A-Za-z][A-Za-z0-9_]{4,31}$`)
	hhmmRe     = regexp.MustCompile(`^([01][0-9]|2[0-3]):[0-5][0-9]$`)
	slugRe     = regexp.MustCompile(`^[a-z0-9]+(?:-[a-z0-9]+)*$`)
)

var reservedUsernames = map[string]bool{
	"admin": true, "administrator": true, "administrateur": true, "root": true, "system": true,
	"support": true, "moderator": true, "moderateur": true, "staff": true, "api": true, "null": true,
	"undefined": true, "opensex": true, "security": true, "billing": true, "manager": true,
	"gerant": true, "help": true, "contact": true, "info": true, "webmaster": true, "owner": true,
}

// Username checks RM-03 and returns an error code or "".
func Username(u string) string {
	if !usernameRe.MatchString(u) {
		return "invalid_username"
	}
	if reservedUsernames[strings.ToLower(u)] {
		return "reserved_username"
	}
	return ""
}

// PasswordLength checks RM-05 length rules (12–128, 16 for admins).
func PasswordLength(p string, admin bool) string {
	n := utf8.RuneCountInString(p)
	min := 12
	if admin {
		min = 16
	}
	if n < min {
		return "password_too_short"
	}
	if n > 128 {
		return "password_too_long"
	}
	return ""
}

// NormalizePhone strips spaces, dots, dashes and parentheses and converts a
// leading 00 into +. It returns "" if the result is not E.164.
func NormalizePhone(s string) string {
	var b strings.Builder
	for _, r := range strings.TrimSpace(s) {
		switch {
		case r >= '0' && r <= '9', r == '+':
			b.WriteRune(r)
		case r == ' ' || r == '.' || r == '-' || r == '(' || r == ')' || r == '/':
		default:
			return ""
		}
	}
	out := b.String()
	if strings.HasPrefix(out, "00") {
		out = "+" + out[2:]
	}
	if !e164Re.MatchString(out) {
		return ""
	}
	return out
}

var socialDomains = map[string][]string{
	"INSTAGRAM": {"instagram.com"},
	"FACEBOOK":  {"facebook.com", "fb.com"},
	"TELEGRAM":  {"t.me", "telegram.me"},
}

// NormalizeContact validates a contact value for its type and returns the
// normalised value or an error code.
func NormalizeContact(typ, value string) (string, string) {
	value = strings.TrimSpace(value)
	if value == "" || utf8.RuneCountInString(value) > 200 {
		return "", "invalid_contact_value"
	}
	switch typ {
	case "PHONE", "WHATSAPP":
		v := NormalizePhone(value)
		if v == "" {
			return "", "invalid_phone"
		}
		return v, ""
	case "TELEGRAM":
		h := strings.TrimPrefix(value, "@")
		if telegramRe.MatchString(h) {
			return "@" + h, ""
		}
		return httpsURL(value, socialDomains[typ])
	case "INSTAGRAM", "FACEBOOK":
		return httpsURL(value, socialDomains[typ])
	case "WEBSITE":
		return httpsURL(value, nil)
	case "OTHER":
		if hasControl(value) {
			return "", "invalid_contact_value"
		}
		return value, ""
	}
	return "", "invalid_contact_type"
}

func httpsURL(raw string, domains []string) (string, string) {
	u, err := url.Parse(raw)
	if err != nil || u.Scheme != "https" || u.Host == "" || u.User != nil {
		return "", "invalid_url"
	}
	host := strings.ToLower(u.Hostname())
	if len(domains) > 0 {
		ok := false
		for _, d := range domains {
			if host == d || strings.HasSuffix(host, "."+d) {
				ok = true
			}
		}
		if !ok {
			return "", "invalid_url_domain"
		}
	}
	u.Host = strings.ToLower(u.Host)
	u.Fragment = ""
	return u.String(), ""
}

var weekDays = map[string]bool{"mon": true, "tue": true, "wed": true, "thu": true, "fri": true, "sat": true, "sun": true}

type timeRange struct {
	Open  string `json:"open"`
	Close string `json:"close"`
}

// OpeningHours validates {"mon":[{"open":"10:00","close":"02:00"}], ...}
// and returns its canonical JSON.
func OpeningHours(raw json.RawMessage) (json.RawMessage, string) {
	if len(raw) == 0 || string(raw) == "null" {
		return json.RawMessage("{}"), ""
	}
	var m map[string][]timeRange
	dec := json.NewDecoder(strings.NewReader(string(raw)))
	dec.DisallowUnknownFields()
	if err := dec.Decode(&m); err != nil {
		return nil, "invalid_opening_hours"
	}
	for day, ranges := range m {
		if !weekDays[day] || len(ranges) > 3 {
			return nil, "invalid_opening_hours"
		}
		for _, r := range ranges {
			if !hhmmRe.MatchString(r.Open) || !hhmmRe.MatchString(r.Close) || r.Open == r.Close {
				return nil, "invalid_opening_hours"
			}
		}
	}
	out, _ := json.Marshal(m)
	return out, ""
}

// PlainText rejects control characters (other than newlines/tabs) and
// enforces a maximum length in runes. Text is stored as is and escaped on
// output by the frontend.
func PlainText(s string, max int) (string, string) {
	s = strings.TrimSpace(s)
	if utf8.RuneCountInString(s) > max {
		return "", "too_long"
	}
	if !utf8.ValidString(s) || hasControl(strings.NewReplacer("\n", "", "\r", "", "\t", "").Replace(s)) {
		return "", "invalid_characters"
	}
	return s, ""
}

func hasControl(s string) bool {
	for _, r := range s {
		if unicode.IsControl(r) {
			return true
		}
	}
	return false
}

// Slugify turns a display name into a URL slug ("Chez Lola" → "chez-lola").
func Slugify(s string) string {
	t := transform.Chain(norm.NFD, runes.Remove(runes.In(unicode.Mn)), norm.NFC)
	ascii, _, _ := transform.String(t, s)
	var b strings.Builder
	dash := false
	for _, r := range strings.ToLower(ascii) {
		if (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') {
			b.WriteRune(r)
			dash = false
		} else if !dash && b.Len() > 0 {
			b.WriteByte('-')
			dash = true
		}
	}
	out := strings.Trim(b.String(), "-")
	if len(out) > 60 {
		out = strings.Trim(out[:60], "-")
	}
	return out
}

func IsSlug(s string) bool { return len(s) <= 100 && slugRe.MatchString(s) }

// ISO 639-1 codes accepted for profile languages.
var Languages = []string{"fr", "nl", "de", "en", "es", "it", "pt", "ro", "bg", "pl", "ru", "uk", "ar", "tr", "hu", "cs", "sk", "el", "sq", "zh", "th", "vi", "ja", "ko", "hi"}

var languageSet = func() map[string]bool {
	m := map[string]bool{}
	for _, l := range Languages {
		m[l] = true
	}
	return m
}()

func Language(code string) bool { return languageSet[code] }

// Physical attribute enumerations (CHECK constraints in SQL).
var (
	HairColors = []string{"BLONDE", "BROWN", "BLACK", "RED", "GREY", "OTHER"}
	EyeColors  = []string{"BLUE", "GREEN", "BROWN", "HAZEL", "GREY", "BLACK", "OTHER"}
	BodyTypes  = []string{"SLIM", "ATHLETIC", "AVERAGE", "CURVY", "PLUS_SIZE", "MUSCULAR"}
)

// ExtraAttributes is the closed list of keys of professional_profiles.extra_attributes.
var ExtraAttributes = map[string][]string{
	"gender":    {"FEMALE", "MALE", "TRANS_WOMAN", "TRANS_MAN", "NON_BINARY"},
	"skin_tone": {"FAIR", "LIGHT", "MEDIUM", "OLIVE", "TAN", "BROWN", "DARK"},
	"tattoos":   {"true", "false"},
	"piercings": {"true", "false"},
	"smoker":    {"true", "false"},
}

func OneOf(v string, allowed []string) bool {
	for _, a := range allowed {
		if v == a {
			return true
		}
	}
	return false
}

// ValidateExtraAttributes checks keys and values and returns canonical JSON.
func ValidateExtraAttributes(m map[string]string) (json.RawMessage, string) {
	for k, v := range m {
		allowed, ok := ExtraAttributes[k]
		if !ok || !OneOf(v, allowed) {
			return nil, fmt.Sprintf("invalid_extra_attribute:%s", k)
		}
	}
	if m == nil {
		m = map[string]string{}
	}
	out, _ := json.Marshal(m)
	return out, ""
}
