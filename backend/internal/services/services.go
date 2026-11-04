// Package services implements the business rules (RM-xx), ownership checks,
// transactions and alerts. Services receive context.Context (carrying the
// Actor and request metadata) and never depend on Gin.
package services

import (
	"context"
	"log/slog"
	"time"

	"github.com/redis/go-redis/v9"

	"opensex/backend/internal/config"
	"opensex/backend/internal/database"
	"opensex/backend/internal/repositories"
	"opensex/backend/pkg/clock"
	"opensex/backend/pkg/jwt"
	"opensex/backend/pkg/password"
	"opensex/backend/pkg/payment"
	"opensex/backend/pkg/ratelimit"
	"opensex/backend/pkg/storage"
)

type Deps struct {
	Cfg     *config.Config
	DB      *database.DB
	Repos   *repositories.Repos
	Clock   clock.Clock
	Hasher  *password.Hasher
	Breach  password.BreachChecker
	JWT     *jwt.Issuer
	Redis   redis.UniversalClient
	Limiter *ratelimit.Limiter
	Storage storage.Storage
	Payment payment.Provider
	Logger  *slog.Logger
}

// Services is the container of every business service. Each service embeds
// a pointer to it to reach the dependencies and its sibling services.
type Services struct {
	Deps
	Alert          *AlertService
	Denylist       *Denylist
	Auth           *AuthService
	Bootstrap      *BootstrapService
	Establishments *EstablishmentService
	Staff          *StaffService
	Profiles       *ProfileService
	Publication    *PublicationService
	Catalog        *CatalogService
	Clients        *ClientService
	Billing        *BillingService
	Admin          *AdminService
	Stats          *StatsService
}

func New(d Deps) *Services {
	s := &Services{Deps: d}
	if s.Clock == nil {
		s.Clock = clock.System{}
	}
	if s.Logger == nil {
		s.Logger = slog.Default()
	}
	s.Alert = &AlertService{s}
	s.Denylist = &Denylist{rdb: d.Redis, ttl: maxDur(d.Cfg.AccessTTL, d.Cfg.AdminAccessTTL) + time.Minute}
	s.Auth = &AuthService{s}
	s.Bootstrap = &BootstrapService{s}
	s.Establishments = &EstablishmentService{s}
	s.Staff = &StaffService{s}
	s.Profiles = &ProfileService{s}
	s.Publication = &PublicationService{s}
	s.Catalog = &CatalogService{s}
	s.Clients = &ClientService{s}
	s.Billing = &BillingService{s}
	s.Admin = &AdminService{s}
	s.Stats = &StatsService{s}
	return s
}

func (s *Services) now() time.Time { return s.Clock.Now() }

// tx runs fn in a database transaction.
func (s *Services) tx(ctx context.Context, fn func(ctx context.Context) error) error {
	return s.DB.WithTx(ctx, fn)
}

func maxDur(a, b time.Duration) time.Duration {
	if a > b {
		return a
	}
	return b
}

func ptr[T any](v T) *T { return &v }

func deref[T any](p *T) T {
	var zero T
	if p == nil {
		return zero
	}
	return *p
}
