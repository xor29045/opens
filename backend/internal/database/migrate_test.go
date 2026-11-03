package database

import (
	"context"
	"net/url"
	"os"
	"testing"
)

// Requires TEST_DATABASE_URL (see Makefile: make test-integration). The test
// uses its own database so that it can run alongside the integration tests.
func TestMigrationsUpDownUp(t *testing.T) {
	raw := os.Getenv("TEST_DATABASE_URL")
	if raw == "" {
		t.Skip("TEST_DATABASE_URL not set")
	}
	admin, err := Open(Options{URL: raw, MaxConns: 1})
	if err != nil {
		t.Fatal(err)
	}
	defer admin.Close()
	ctx := context.Background()
	_, _ = admin.SQL.ExecContext(ctx, `DROP DATABASE IF EXISTS opensex_migrate_test`)
	if _, err := admin.SQL.ExecContext(ctx, `CREATE DATABASE opensex_migrate_test`); err != nil {
		t.Fatal(err)
	}
	u, _ := url.Parse(raw)
	u.Path = "/opensex_migrate_test"
	dsn := u.String()
	for i, step := range []func() error{
		func() error { return MigrateUp(dsn) },
		func() error { return MigrateDown(dsn, 0) },
		func() error { return MigrateUp(dsn) },
	} {
		if err := step(); err != nil {
			t.Fatalf("step %d: %v", i, err)
		}
	}
	v, dirty, err := MigrateVersion(dsn)
	if err != nil || dirty || v != 4 {
		t.Fatalf("version %d dirty=%v err=%v", v, dirty, err)
	}
}
