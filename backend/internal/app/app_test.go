package app

import (
	"bytes"
	"log/slog"
	"strings"
	"testing"
)

func TestLogMasking(t *testing.T) {
	var buf bytes.Buffer
	log := slog.New(slog.NewJSONHandler(&buf, &slog.HandlerOptions{ReplaceAttr: maskSensitive}))
	log.Info("x", "password", "hunter2", "recovery_code", "AAAA", "Authorization", "Bearer abc", "x-nowpayments-sig", "f00", "user", "bob")
	out := buf.String()
	for _, secret := range []string{"hunter2", "AAAA", "Bearer abc", "f00"} {
		if strings.Contains(out, secret) {
			t.Fatalf("secret %q logged: %s", secret, out)
		}
	}
	if !strings.Contains(out, "bob") {
		t.Fatal("non sensitive values must be kept")
	}
}
