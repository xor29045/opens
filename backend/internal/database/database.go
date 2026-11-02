// Package database opens PostgreSQL connections, runs versioned migrations
// (golang-migrate, ADR-004) and provides transaction and advisory-lock helpers.
package database

import (
	"context"
	"database/sql"
	"embed"
	"errors"
	"fmt"
	"hash/fnv"
	"log/slog"
	"strings"
	"time"

	"github.com/golang-migrate/migrate/v4"
	_ "github.com/golang-migrate/migrate/v4/database/pgx/v5" // migrate driver
	"github.com/golang-migrate/migrate/v4/source/iofs"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

//go:embed migrations/*.sql
var migrationsFS embed.FS

type DB struct {
	Gorm *gorm.DB
	SQL  *sql.DB
}

type Options struct {
	URL      string
	MaxConns int
	Debug    bool
}

func Open(opts Options) (*DB, error) {
	lvl := logger.Silent
	if opts.Debug {
		lvl = logger.Info
	}
	g, err := gorm.Open(postgres.Open(withUTC(opts.URL)), &gorm.Config{
		Logger:                 logger.Default.LogMode(lvl),
		SkipDefaultTransaction: true,
		TranslateError:         true,
		NowFunc:                func() time.Time { return time.Now().UTC() },
	})
	if err != nil {
		return nil, fmt.Errorf("open database: %w", err)
	}
	sqlDB, err := g.DB()
	if err != nil {
		return nil, err
	}
	if opts.MaxConns <= 0 {
		opts.MaxConns = 20
	}
	sqlDB.SetMaxOpenConns(opts.MaxConns)
	sqlDB.SetMaxIdleConns(opts.MaxConns / 2)
	sqlDB.SetConnMaxLifetime(30 * time.Minute)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := sqlDB.PingContext(ctx); err != nil {
		return nil, fmt.Errorf("ping database: %w", err)
	}
	return &DB{Gorm: g, SQL: sqlDB}, nil
}

func (d *DB) Close() error { return d.SQL.Close() }

func (d *DB) Ping(ctx context.Context) error { return d.SQL.PingContext(ctx) }

type txKey struct{}

// Conn returns the transaction bound to ctx, or the pool.
func (d *DB) Conn(ctx context.Context) *gorm.DB {
	if tx, ok := ctx.Value(txKey{}).(*gorm.DB); ok {
		return tx
	}
	return d.Gorm.WithContext(ctx)
}

// InTx reports whether ctx carries a transaction.
func InTx(ctx context.Context) bool {
	_, ok := ctx.Value(txKey{}).(*gorm.DB)
	return ok
}

// WithTx runs fn in a transaction (joining the current one if any).
func (d *DB) WithTx(ctx context.Context, fn func(ctx context.Context) error) error {
	if InTx(ctx) {
		return fn(ctx)
	}
	return d.Gorm.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		return fn(context.WithValue(ctx, txKey{}, tx))
	})
}

// LockKey derives a stable advisory-lock key from a name.
func LockKey(name string) int64 {
	h := fnv.New64a()
	_, _ = h.Write([]byte(name))
	return int64(h.Sum64())
}

// XactLock takes a transaction-scoped advisory lock (must be called in WithTx).
func (d *DB) XactLock(ctx context.Context, name string) error {
	return d.Conn(ctx).Exec("SELECT pg_advisory_xact_lock(?)", LockKey(name)).Error
}

// TryLock takes a session-level advisory lock on a dedicated connection.
// ok is false when another session holds the lock.
func (d *DB) TryLock(ctx context.Context, name string) (release func(), ok bool, err error) {
	conn, err := d.SQL.Conn(ctx)
	if err != nil {
		return nil, false, err
	}
	key := LockKey(name)
	if err := conn.QueryRowContext(ctx, "SELECT pg_try_advisory_lock($1)", key).Scan(&ok); err != nil {
		conn.Close()
		return nil, false, err
	}
	if !ok {
		conn.Close()
		return nil, false, nil
	}
	return func() {
		if _, err := conn.ExecContext(context.Background(), "SELECT pg_advisory_unlock($1)", key); err != nil {
			slog.Error("advisory unlock failed", "lock", name, "error", err)
		}
		conn.Close()
	}, true, nil
}

// Lock blocks until the session-level advisory lock is acquired.
func (d *DB) Lock(ctx context.Context, name string) (release func(), err error) {
	conn, err := d.SQL.Conn(ctx)
	if err != nil {
		return nil, err
	}
	key := LockKey(name)
	if _, err := conn.ExecContext(ctx, "SELECT pg_advisory_lock($1)", key); err != nil {
		conn.Close()
		return nil, err
	}
	return func() {
		_, _ = conn.ExecContext(context.Background(), "SELECT pg_advisory_unlock($1)", key)
		conn.Close()
	}, nil
}

func migrator(url string) (*migrate.Migrate, error) {
	src, err := iofs.New(migrationsFS, "migrations")
	if err != nil {
		return nil, err
	}
	u := url
	for _, p := range []string{"postgres://", "postgresql://"} {
		if strings.HasPrefix(u, p) {
			u = "pgx5://" + strings.TrimPrefix(u, p)
		}
	}
	return migrate.NewWithSourceInstance("iofs", src, u)
}

// MigrateUp applies all pending migrations.
func MigrateUp(url string) error {
	m, err := migrator(url)
	if err != nil {
		return err
	}
	defer m.Close()
	if err := m.Up(); err != nil && !errors.Is(err, migrate.ErrNoChange) {
		return err
	}
	return nil
}

// MigrateDown rolls back n migrations (all if n <= 0).
func MigrateDown(url string, n int) error {
	m, err := migrator(url)
	if err != nil {
		return err
	}
	defer m.Close()
	if n <= 0 {
		err = m.Down()
	} else {
		err = m.Steps(-n)
	}
	if err != nil && !errors.Is(err, migrate.ErrNoChange) {
		return err
	}
	return nil
}

// MigrateVersion returns the current schema version.
func MigrateVersion(url string) (uint, bool, error) {
	m, err := migrator(url)
	if err != nil {
		return 0, false, err
	}
	defer m.Close()
	v, dirty, err := m.Version()
	if errors.Is(err, migrate.ErrNilVersion) {
		return 0, false, nil
	}
	return v, dirty, err
}

// IsUniqueViolation reports whether err is a unique constraint violation.
func IsUniqueViolation(err error) bool {
	return errors.Is(err, gorm.ErrDuplicatedKey)
}

// IsNotFound reports whether err is gorm.ErrRecordNotFound.
func IsNotFound(err error) bool { return errors.Is(err, gorm.ErrRecordNotFound) }

// withUTC forces the session time zone to UTC so that timestamps are
// returned consistently whatever the server default.
func withUTC(url string) string {
	if strings.Contains(url, "timezone=") || !strings.Contains(url, "://") {
		return url
	}
	if strings.Contains(url, "?") {
		return url + "&timezone=UTC"
	}
	return url + "?timezone=UTC"
}
