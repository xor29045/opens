package services

import (
	"context"
	"time"

	"github.com/google/uuid"

	"opensex/backend/internal/apperr"
	"opensex/backend/internal/rbac"
)

// StatsService keeps aggregated, anonymous counters per profile and day in
// Redis (profile views and views of CLIENTS contacts, RM-42). No client is
// identified and no table is required.
type StatsService struct{ *Services }

const statsRetention = 90 * 24 * time.Hour

func statsKey(kind string, profileID uuid.UUID, day time.Time) string {
	return "stats:" + kind + ":" + profileID.String() + ":" + day.Format("20060102")
}

func (s *StatsService) incr(ctx context.Context, kind string, profileID uuid.UUID) {
	key := statsKey(kind, profileID, s.now())
	pipe := s.Redis.Pipeline()
	pipe.Incr(ctx, key)
	pipe.Expire(ctx, key, statsRetention)
	if _, err := pipe.Exec(ctx); err != nil {
		s.Logger.DebugContext(ctx, "stats increment failed", "error", err)
	}
}

func (s *StatsService) RecordView(ctx context.Context, profileID uuid.UUID) {
	s.incr(ctx, "views", profileID)
}

func (s *StatsService) RecordContactView(ctx context.Context, profileID uuid.UUID) {
	s.incr(ctx, "contacts", profileID)
}

type ProfileStats struct {
	ProfileID    uuid.UUID `json:"profile_id"`
	DisplayName  string    `json:"display_name"`
	Views        int64     `json:"views"`
	ContactViews int64     `json:"contact_views"`
}

type EstablishmentStats struct {
	Days     int            `json:"days"`
	Views    int64          `json:"views"`
	Contacts int64          `json:"contact_views"`
	Profiles []ProfileStats `json:"profiles"`
}

// ForEstablishment returns the last `days` days of counters (subscription required).
func (s *StatsService) ForEstablishment(ctx context.Context, actor *rbac.Actor, days int) (*EstablishmentStats, error) {
	if days < 1 || days > 90 {
		days = 30
	}
	entitled, _, err := s.Repos.Profiles.HasEntitlement(ctx, actor.EstablishmentID, s.now())
	if err != nil {
		return nil, apperr.Internal(err)
	}
	if !entitled {
		return nil, apperr.Forbidden("subscription_required", "statistics require an active subscription")
	}
	profiles, err := s.Repos.Profiles.AllOfEstablishment(ctx, actor.EstablishmentID)
	if err != nil {
		return nil, apperr.Internal(err)
	}
	out := &EstablishmentStats{Days: days, Profiles: []ProfileStats{}}
	now := s.now()
	for _, p := range profiles {
		var keysV, keysC []string
		for d := 0; d < days; d++ {
			day := now.AddDate(0, 0, -d)
			keysV = append(keysV, statsKey("views", p.ID, day))
			keysC = append(keysC, statsKey("contacts", p.ID, day))
		}
		ps := ProfileStats{ProfileID: p.ID, DisplayName: p.DisplayName, Views: s.sum(ctx, keysV), ContactViews: s.sum(ctx, keysC)}
		out.Views += ps.Views
		out.Contacts += ps.ContactViews
		out.Profiles = append(out.Profiles, ps)
	}
	return out, nil
}

func (s *StatsService) sum(ctx context.Context, keys []string) int64 {
	vals, err := s.Redis.MGet(ctx, keys...).Result()
	if err != nil {
		return 0
	}
	var total int64
	for _, v := range vals {
		if str, ok := v.(string); ok {
			var n int64
			for _, ch := range str {
				n = n*10 + int64(ch-'0')
			}
			total += n
		}
	}
	return total
}
