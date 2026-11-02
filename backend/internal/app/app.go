// Package app wires the configuration, infrastructure clients and services
// shared by the server, the worker and the ops CLI.
package app

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/redis/go-redis/v9"

	"opensex/backend/internal/config"
	"opensex/backend/internal/database"
	"opensex/backend/internal/repositories"
	"opensex/backend/internal/services"
	"opensex/backend/pkg/clock"
	"opensex/backend/pkg/crypto"
	"opensex/backend/pkg/jwt"
	"opensex/backend/pkg/password"
	"opensex/backend/pkg/payment"
	"opensex/backend/pkg/payment/fake"
	"opensex/backend/pkg/payment/nowpayments"
	"opensex/backend/pkg/ratelimit"
	"opensex/backend/pkg/storage"
)

type App struct {
	Cfg     *config.Config
	Logger  *slog.Logger
	DB      *database.DB
	Redis   redis.UniversalClient
	Limiter *ratelimit.Limiter
	Svc     *services.Services
}

// NewLogger builds the slog logger with sensitive attribute masking (§13.2).
func NewLogger(cfg *config.Config) *slog.Logger {
	var lvl slog.Level
	_ = lvl.UnmarshalText([]byte(cfg.LogLevel))
	opts := &slog.HandlerOptions{Level: lvl, ReplaceAttr: maskSensitive}
	var h slog.Handler
	if cfg.LogFormat == "text" {
		h = slog.NewTextHandler(os.Stdout, opts)
	} else {
		h = slog.NewJSONHandler(os.Stdout, opts)
	}
	return slog.New(h)
}

var sensitiveKeys = []string{"password", "token", "authorization", "cookie", "secret", "code", "recovery", "x-nowpayments-sig"}

func maskSensitive(_ []string, a slog.Attr) slog.Attr {
	k := strings.ToLower(a.Key)
	for _, s := range sensitiveKeys {
		if strings.Contains(k, s) && k != "status_code" {
			return slog.String(a.Key, "[REDACTED]")
		}
	}
	return a
}

// New connects to every dependency. The returned App must be closed.
func New(ctx context.Context, cfg *config.Config) (*App, error) {
	logger := NewLogger(cfg)
	slog.SetDefault(logger)

	db, err := database.Open(database.Options{URL: cfg.DatabaseURL, MaxConns: cfg.DBMaxConns, Debug: cfg.LogLevel == "debug"})
	if err != nil {
		return nil, err
	}
	ropts, err := redis.ParseURL(cfg.RedisURL)
	if err != nil {
		return nil, fmt.Errorf("REDIS_URL: %w", err)
	}
	rdb := redis.NewClient(ropts)
	pctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	if err := rdb.Ping(pctx).Err(); err != nil {
		return nil, fmt.Errorf("redis: %w", err)
	}
	keys, err := loadKeys(cfg, logger)
	if err != nil {
		return nil, err
	}
	enc, err := crypto.NewFieldEncryptor(cfg.FieldEncryptionKeys)
	if err != nil {
		return nil, err
	}
	var store storage.Storage
	if cfg.StorageDriver == "memory" {
		logger.Warn("in-memory photo storage: development only")
		store = storage.NewMemory()
	} else {
		store, err = storage.NewS3(storage.S3Config{Endpoint: cfg.S3Endpoint, Bucket: cfg.S3Bucket, AccessKey: cfg.S3AccessKey,
			SecretKey: cfg.S3SecretKey, Region: cfg.S3Region, PublicBaseURL: cfg.PhotosPublicBaseURL, PublicEndpoint: cfg.S3PublicEndpoint})
		if err != nil {
			return nil, err
		}
	}
	var pay payment.Provider
	if cfg.PaymentProvider == "fake" {
		logger.Warn("fake payment provider: development only")
		pay = fake.New(cfg.NOWPaymentsIPNSecret)
	} else {
		pay = nowpayments.New(cfg.NOWPaymentsAPIURL, cfg.NOWPaymentsAPIKey, cfg.NOWPaymentsIPNSecret)
	}
	var breach password.BreachChecker = password.LocalChecker{}
	if cfg.HIBPEnabled {
		breach = password.NewHIBPChecker()
	}
	limiter := ratelimit.New(rdb)
	svc := services.New(services.Deps{
		Cfg: cfg, DB: db, Repos: repositories.New(db), Clock: clock.System{},
		Hasher: password.NewHasher(password.Params{MemoryKiB: cfg.Argon2MemoryKiB, Iterations: cfg.Argon2Iterations,
			Parallelism: cfg.Argon2Parallelism, SaltLen: 16, KeyLen: 32}, cfg.Argon2MaxConcurrent),
		Breach: breach, JWT: &jwt.Issuer{Keys: keys, Issuer: cfg.JWTIssuer, Audience: cfg.JWTAudience},
		Redis: rdb, Limiter: limiter, Storage: store, Payment: pay, Encryptor: enc, Logger: logger,
	})
	return &App{Cfg: cfg, Logger: logger, DB: db, Redis: rdb, Limiter: limiter, Svc: svc}, nil
}

// loadKeys reads the JWT key ring. In development a missing active key is
// generated on the fly so that `docker compose up` works out of the box.
func loadKeys(cfg *config.Config, log *slog.Logger) (*jwt.KeyRing, error) {
	if cfg.IsDev() {
		if _, err := os.Stat(filepath.Join(cfg.JWTKeysDir, cfg.JWTActiveKID+".pem")); os.IsNotExist(err) {
			log.Warn("generating a development JWT key", "dir", cfg.JWTKeysDir, "kid", cfg.JWTActiveKID)
			// Another process (worker) may have generated it concurrently.
			if err := jwt.GenerateKeyFiles(cfg.JWTKeysDir, cfg.JWTActiveKID); err != nil && !strings.Contains(err.Error(), "already exists") {
				return nil, err
			}
		}
	}
	return jwt.LoadKeyRing(cfg.JWTKeysDir, cfg.JWTActiveKID)
}

func (a *App) Close() {
	_ = a.Redis.Close()
	_ = a.DB.Close()
}
