package validators

import (
	"encoding/json"
	"testing"
)

func TestCompanyNumber(t *testing.T) {
	// 0403.170.701 is a well-known valid BCE number (checksum 97 − 4031707 mod 97).
	valid := []string{"0403.170.701", "0403170701", "BE 0403 170 701", "403170701"}
	for _, v := range valid {
		if CompanyNumber(v) != "0403.170.701" {
			t.Errorf("%q should be valid, got %q", v, CompanyNumber(v))
		}
	}
	for _, v := range []string{"0403.170.702", "2403170701", "abc", "", "04031707011"} {
		if CompanyNumber(v) != "" {
			t.Errorf("%q should be invalid", v)
		}
	}
}

func TestUsername(t *testing.T) {
	if Username("lola_92") != "" || Username("ab") == "" || Username("Admin") != "reserved_username" || Username("a b c") == "" {
		t.Fatal("username rules")
	}
}

func TestContacts(t *testing.T) {
	cases := []struct{ typ, in, want, code string }{
		{"PHONE", "+32 470 12 34 56", "+32470123456", ""},
		{"PHONE", "0032 470/12.34.56", "+32470123456", ""},
		{"PHONE", "0470123456", "", "invalid_phone"},
		{"INSTAGRAM", "https://www.instagram.com/lola", "https://www.instagram.com/lola", ""},
		{"INSTAGRAM", "https://evil.com/instagram.com", "", "invalid_url_domain"},
		{"INSTAGRAM", "http://instagram.com/lola", "", "invalid_url"},
		{"WEBSITE", "javascript:alert(1)", "", "invalid_url"},
		{"TELEGRAM", "@lola_bxl", "@lola_bxl", ""},
		{"FACEBOOK", "https://user@facebook.com/x", "", "invalid_url"},
	}
	for _, c := range cases {
		got, code := NormalizeContact(c.typ, c.in)
		if got != c.want || code != c.code {
			t.Errorf("%s %q → %q %q, want %q %q", c.typ, c.in, got, code, c.want, c.code)
		}
	}
}

func TestOpeningHours(t *testing.T) {
	if _, code := OpeningHours(json.RawMessage(`{"mon":[{"open":"10:00","close":"02:00"}]}`)); code != "" {
		t.Fatal(code)
	}
	for _, bad := range []string{`{"monday":[]}`, `{"mon":[{"open":"25:00","close":"02:00"}]}`, `{"mon":[{"open":"10:00","close":"11:00","x":1}]}`} {
		if _, code := OpeningHours(json.RawMessage(bad)); code == "" {
			t.Errorf("%s accepted", bad)
		}
	}
}

func TestSlugify(t *testing.T) {
	if got := Slugify("  Chez Lola — Ixelles! "); got != "chez-lola-ixelles" {
		t.Fatal(got)
	}
	if got := Slugify("Élodie"); got != "elodie" {
		t.Fatal(got)
	}
}
