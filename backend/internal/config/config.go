// Package config loads the configuration from environment variables and
// secret files and refuses to start on missing or unsafe values (fail fast).
package config

import (
	"errors"
	"fmt"
	"net"
	"net/url"
	"os"
	"strconv"
	"strings"
	"time"

	"opensex/backend/pkg/crypto"
)

const (
	EnvDevelopment = "development"
	EnvTest        = "test"
	EnvStaging     = "staging"
	EnvProduction  = "production"
)

type Config struct {
	Env          string
	BaseURL      string // public URL of the site, e.g. https://example.be
	APIPort      int
	LogLevel     string
	LogFormat    string
	SwaggerOn    bool
	TermsVersion string

	DatabaseURL string
	DBMaxConns  int
	RedisURL    string

	JWTIssuer          string
	JWTAudience        string
	JWTKeysDir         string
	JWTActiveKID       string
	AccessTTL          time.Duration
	AdminAccessTTL     time.Duration
	RefreshTTL         time.Duration
	RefreshAbsTTL      time.Duration
	AdminRefreshTTL    time.Duration
	AdminRefreshAbsTTL time.Duration
	CookieSecure       bool

	Argon2MemoryKiB     uint32
	Argon2Iterations    uint32
	Argon2Parallelism   uint8
	Argon2MaxConcurrent int
	HIBPEnabled         bool

	IPHashKey       []byte
	RecoveryCodeKey []byte

	CORSAllowedOrigins []string
	TrustedProxies     []string

	PaymentProvider           string // nowpayments | fake
	NOWPaymentsAPIURL         string
	NOWPaymentsAPIKey         string
	NOWPaymentsIPNSecret      string
	NOWPaymentsIPNCallbackURL string
	UnderpayToleranceBPS      int64

	StorageDriver       string // s3 | memory
	S3Endpoint          string
	S3Bucket            string
	S3AccessKey         string
	S3SecretKey         string
	S3Region            string
	S3PublicEndpoint    string
	PhotosPublicBaseURL string

	PlatformLegalName     string
	PlatformCompanyNumber string
	PlatformAddress       string

	AdminBootstrapUsername  string
	AdminBootstrapPassword  string // resolved from file or (dev) env
	AdminBootstrapUsersFile string
	AdminAllowedCIDRs       []*net.IPNet
	AdminAlertWebhookURL    string
}

func (c *Config) IsProd() bool { return c.Env == EnvProduction }
func (c *Config) IsDev() bool  { return c.Env == EnvDevelopment || c.Env == EnvTest }

type loader struct {
	errs []string
}

func (l *loader) str(key, def string) string {
	if v, ok := os.LookupEnv(key); ok && strings.TrimSpace(v) != "" {
		return strings.TrimSpace(v)
	}
	return def
}

// secret reads KEY, or the content of the file named by KEY_FILE.
func (l *loader) secret(key string) string {
	if f := l.str(key+"_FILE", ""); f != "" {
		b, err := os.ReadFile(f)
		if err != nil {
			l.errs = append(l.errs, fmt.Sprintf("%s_FILE: %v", key, err))
			return ""
		}
		return strings.TrimSpace(string(b))
	}
	return l.str(key, "")
}

func (l *loader) required(key, v string) string {
	if v == "" {
		l.errs = append(l.errs, key+" is required")
	}
	return v
}

func (l *loader) int(key string, def int) int {
	v := l.str(key, "")
	if v == "" {
		return def
	}
	n, err := strconv.Atoi(v)
	if err != nil {
		l.errs = append(l.errs, key+" must be an integer")
	}
	return n
}

func (l *loader) bool(key string, def bool) bool {
	v := l.str(key, "")
	if v == "" {
		return def
	}
	b, err := strconv.ParseBool(v)
	if err != nil {
		l.errs = append(l.errs, key+" must be a boolean")
	}
	return b
}

func (l *loader) dur(key string, def time.Duration) time.Duration {
	v := l.str(key, "")
	if v == "" {
		return def
	}
	d, err := time.ParseDuration(v)
	if err != nil || d <= 0 {
		l.errs = append(l.errs, key+" must be a positive duration (e.g. 10m, 168h)")
	}
	return d
}

func (l *loader) key(name string, minLen int) []byte {
	v := l.secret(name)
	if v == "" {
		l.errs = append(l.errs, name+" is required")
		return nil
	}
	k, err := crypto.DecodeKey(v, minLen)
	if err != nil {
		l.errs = append(l.errs, name+": "+err.Error())
	}
	return k
}

func list(v string) []string {
	var out []string
	for _, p := range strings.Split(v, ",") {
		if p = strings.TrimSpace(p); p != "" {
			out = append(out, p)
		}
	}
	return out
}

// Load reads and validates the configuration.
func Load() (*Config, error) {
	l := &loader{}
	c := &Config{}
	c.Env = l.str("APP_ENV", EnvDevelopment)
	switch c.Env {
	case EnvDevelopment, EnvTest, EnvStaging, EnvProduction:
	default:
		l.errs = append(l.errs, "APP_ENV must be development, test, staging or production")
	}
	c.BaseURL = strings.TrimRight(l.required("APP_BASE_URL", l.str("APP_BASE_URL", "")), "/")
	c.APIPort = l.int("API_PORT", 8080)
	c.LogLevel = l.str("LOG_LEVEL", "info")
	c.LogFormat = l.str("LOG_FORMAT", map[bool]string{true: "text", false: "json"}[c.IsDev()])
	c.SwaggerOn = l.bool("SWAGGER_ENABLED", c.IsDev())
	c.TermsVersion = l.str("TERMS_VERSION", "2026-11-02")

	c.DatabaseURL = l.required("DATABASE_URL", l.secret("DATABASE_URL"))
	c.DBMaxConns = l.int("DB_MAX_CONNS", 20)
	c.RedisURL = l.required("REDIS_URL", l.secret("REDIS_URL"))

	c.JWTIssuer = l.str("JWT_ISSUER", c.BaseURL+"/api")
	c.JWTAudience = l.str("JWT_AUDIENCE", hostOf(c.BaseURL)+"-api")
	c.JWTKeysDir = l.required("JWT_KEYS_DIR", l.str("JWT_KEYS_DIR", ""))
	c.JWTActiveKID = l.required("JWT_ACTIVE_KID", l.str("JWT_ACTIVE_KID", ""))
	c.AccessTTL = l.dur("ACCESS_TOKEN_TTL", 10*time.Minute)
	c.AdminAccessTTL = l.dur("ADMIN_ACCESS_TOKEN_TTL", 5*time.Minute)
	c.RefreshTTL = l.dur("REFRESH_TOKEN_TTL", 7*24*time.Hour)
	c.RefreshAbsTTL = l.dur("REFRESH_TOKEN_ABSOLUTE_TTL", 30*24*time.Hour)
	c.AdminRefreshTTL = l.dur("ADMIN_REFRESH_TOKEN_TTL", 8*time.Hour)
	c.AdminRefreshAbsTTL = l.dur("ADMIN_REFRESH_TOKEN_ABSOLUTE_TTL", 12*time.Hour)
	c.CookieSecure = l.bool("COOKIE_SECURE", true)

	c.Argon2MemoryKiB = uint32(l.int("ARGON2_MEMORY_KIB", 64*1024))
	c.Argon2Iterations = uint32(l.int("ARGON2_ITERATIONS", 3))
	c.Argon2Parallelism = uint8(l.int("ARGON2_PARALLELISM", 2))
	c.Argon2MaxConcurrent = l.int("ARGON2_MAX_CONCURRENT", 8)
	c.HIBPEnabled = l.bool("HIBP_ENABLED", !c.IsDev())

	c.IPHashKey = l.key("IP_HASH_KEY", 32)
	c.RecoveryCodeKey = l.key("RECOVERY_CODE_HMAC_KEY", 32)

	c.CORSAllowedOrigins = list(l.str("CORS_ALLOWED_ORIGINS", c.BaseURL))
	c.TrustedProxies = list(l.str("TRUSTED_PROXIES", ""))

	c.PaymentProvider = l.str("PAYMENT_PROVIDER", "nowpayments")
	c.NOWPaymentsAPIURL = strings.TrimRight(l.str("NOWPAYMENTS_API_URL", "https://api-sandbox.nowpayments.io/v1"), "/")
	c.NOWPaymentsAPIKey = l.secret("NOWPAYMENTS_API_KEY")
	c.NOWPaymentsIPNSecret = l.secret("NOWPAYMENTS_IPN_SECRET")
	c.NOWPaymentsIPNCallbackURL = l.str("NOWPAYMENTS_IPN_CALLBACK_URL", c.BaseURL+"/api/v1/webhooks/nowpayments")
	c.UnderpayToleranceBPS = int64(l.int("PAYMENT_UNDERPAY_TOLERANCE_BPS", 100))
	switch c.PaymentProvider {
	case "nowpayments":
		l.required("NOWPAYMENTS_API_KEY", c.NOWPaymentsAPIKey)
		l.required("NOWPAYMENTS_IPN_SECRET", c.NOWPaymentsIPNSecret)
	case "fake":
		if c.NOWPaymentsIPNSecret == "" {
			c.NOWPaymentsIPNSecret = "fake-ipn-secret"
		}
	default:
		l.errs = append(l.errs, "PAYMENT_PROVIDER must be nowpayments or fake")
	}

	c.StorageDriver = l.str("STORAGE_DRIVER", "s3")
	c.S3Endpoint = l.str("S3_ENDPOINT", "")
	c.S3Bucket = l.str("S3_BUCKET", "")
	c.S3AccessKey = l.secret("S3_ACCESS_KEY")
	c.S3SecretKey = l.secret("S3_SECRET_KEY")
	c.S3Region = l.str("S3_REGION", "eu-west-1")
	c.S3PublicEndpoint = l.str("S3_PUBLIC_ENDPOINT", c.S3Endpoint)
	c.PhotosPublicBaseURL = l.str("PHOTOS_PUBLIC_BASE_URL", "")
	switch c.StorageDriver {
	case "s3":
		l.required("S3_ENDPOINT", c.S3Endpoint)
		l.required("S3_BUCKET", c.S3Bucket)
		l.required("S3_ACCESS_KEY", c.S3AccessKey)
		l.required("S3_SECRET_KEY", c.S3SecretKey)
	case "memory":
	default:
		l.errs = append(l.errs, "STORAGE_DRIVER must be s3 or memory")
	}

	c.AdminBootstrapUsername = l.str("ADMIN_BOOTSTRAP_USERNAME", "")
	c.AdminBootstrapUsersFile = l.str("ADMIN_BOOTSTRAP_USERS_FILE", "")
	if f := l.str("ADMIN_BOOTSTRAP_PASSWORD_FILE", ""); f != "" {
		b, err := os.ReadFile(f)
		if err != nil {
			l.errs = append(l.errs, "ADMIN_BOOTSTRAP_PASSWORD_FILE: "+err.Error())
		}
		c.AdminBootstrapPassword = strings.TrimSpace(string(b))
	} else if v := l.str("ADMIN_BOOTSTRAP_PASSWORD", ""); v != "" {
		if !c.IsDev() {
			l.errs = append(l.errs, "ADMIN_BOOTSTRAP_PASSWORD is only allowed in development; use ADMIN_BOOTSTRAP_PASSWORD_FILE")
		}
		c.AdminBootstrapPassword = v
	}
	for _, cidr := range list(l.str("ADMIN_ALLOWED_CIDRS", "")) {
		_, n, err := net.ParseCIDR(cidr)
		if err != nil {
			l.errs = append(l.errs, "ADMIN_ALLOWED_CIDRS: invalid CIDR "+cidr)
			continue
		}
		c.AdminAllowedCIDRs = append(c.AdminAllowedCIDRs, n)
	}
	c.AdminAlertWebhookURL = l.str("ADMIN_ALERT_WEBHOOK_URL", "")

	c.validateSafety(l)
	if len(l.errs) > 0 {
		return nil, errors.New("invalid configuration:\n  - " + strings.Join(l.errs, "\n  - "))
	}
	return c, nil
}

// validateSafety refuses unsafe settings outside development (§15.2).
func (c *Config) validateSafety(l *loader) {
	if c.IsDev() {
		return
	}
	for _, o := range c.CORSAllowedOrigins {
		if o == "*" {
			l.errs = append(l.errs, "CORS_ALLOWED_ORIGINS must not contain * outside development")
		}
	}
	if !c.CookieSecure {
		l.errs = append(l.errs, "COOKIE_SECURE must be true outside development")
	}
	if !strings.HasPrefix(c.BaseURL, "https://") {
		l.errs = append(l.errs, "APP_BASE_URL must use https outside development")
	}
	if c.StorageDriver == "memory" {
		l.errs = append(l.errs, "STORAGE_DRIVER=memory is only allowed in development")
	}
	if c.PaymentProvider == "fake" {
		l.errs = append(l.errs, "PAYMENT_PROVIDER=fake is only allowed in development")
	}
	if c.IsProd() {
		if strings.Contains(c.NOWPaymentsAPIURL, "sandbox") {
			l.errs = append(l.errs, "NOWPAYMENTS_API_URL must not be the sandbox in production")
		}
		if c.SwaggerOn {
			l.errs = append(l.errs, "SWAGGER_ENABLED must be false in production")
		}
		if strings.HasPrefix(c.JWTActiveKID, "dev") {
			l.errs = append(l.errs, "JWT_ACTIVE_KID looks like a development key")
		}
	}
}

func hostOf(base string) string {
	u, err := url.Parse(base)
	if err != nil || u.Host == "" {
		return "opensex"
	}
	return u.Hostname()
}
