// Package jobs runs the scheduled tasks of §14.5. Each run takes a
// PostgreSQL advisory lock so that several workers never run the same task
// concurrently.
package jobs

import (
	"context"
	"log/slog"
	"time"

	"opensex/backend/internal/services"
)

type Job struct {
	Name     string
	Interval time.Duration
	Run      func(ctx context.Context) (int, error)
}

func Jobs(s *services.Services) []Job {
	return []Job{
		{Name: "renewal_invoices", Interval: time.Hour, Run: s.Billing.CreateRenewalInvoices},
		{Name: "expire_periods", Interval: 15 * time.Minute, Run: s.Billing.ExpirePeriods},
		{Name: "reconcile_payments", Interval: 15 * time.Minute, Run: s.Billing.ReconcileStalePayments},
		{Name: "recompute_publication", Interval: 24 * time.Hour, Run: func(ctx context.Context) (int, error) {
			n, err := s.Repos.Profiles.RecomputePublication(ctx, s.Clock.Now())
			return int(n), err
		}},
		{Name: "purge", Interval: 24 * time.Hour, Run: func(ctx context.Context) (int, error) { return Purge(ctx, s) }},
	}
}

// RunOnce executes a job under its advisory lock; skipped is true when
// another worker holds the lock.
func RunOnce(ctx context.Context, s *services.Services, j Job) (n int, skipped bool, err error) {
	release, ok, err := s.DB.TryLock(ctx, "job:"+j.Name)
	if err != nil {
		return 0, false, err
	}
	if !ok {
		return 0, true, nil
	}
	defer release()
	n, err = j.Run(ctx)
	return n, false, err
}

// Scheduler runs every job at start then at its interval until ctx is done.
func Scheduler(ctx context.Context, s *services.Services, log *slog.Logger) {
	for _, j := range Jobs(s) {
		go func(j Job) {
			t := time.NewTicker(j.Interval)
			defer t.Stop()
			for {
				start := time.Now()
				n, skipped, err := RunOnce(ctx, s, j)
				switch {
				case err != nil:
					log.Error("job failed", "job", j.Name, "error", err)
				case skipped:
					log.Debug("job skipped (locked elsewhere)", "job", j.Name)
				default:
					log.Info("job done", "job", j.Name, "changes", n, "duration_ms", time.Since(start).Milliseconds())
				}
				select {
				case <-ctx.Done():
					return
				case <-t.C:
				}
			}
		}(j)
	}
	<-ctx.Done()
}

const (
	tokenRetention   = 7 * 24 * time.Hour
	deletedRetention = 30 * 24 * time.Hour
)

// Purge removes expired tokens (J+7) and deleted accounts/profiles (J+30).
func Purge(ctx context.Context, s *services.Services) (int, error) {
	now := s.Clock.Now()
	n1, err := s.Repos.Tokens.PurgeExpired(ctx, now.Add(-tokenRetention))
	if err != nil {
		return 0, err
	}
	ids, err := s.Repos.Profiles.PurgeDeleted(ctx, now.Add(-deletedRetention))
	if err != nil {
		return int(n1), err
	}
	photos, err := s.Repos.Profiles.PhotosUnscoped(ctx, ids)
	if err != nil {
		return int(n1), err
	}
	for _, id := range ids {
		if err := s.Repos.Profiles.HardDelete(ctx, id); err != nil {
			return int(n1), err
		}
	}
	for _, ph := range photos {
		if err := s.Storage.Delete(ctx, ph.StorageKey); err != nil {
			s.Logger.Warn("photo purge failed", "key", ph.StorageKey, "error", err)
		}
	}
	n3, err := s.Repos.Users.PurgeDeleted(ctx, now.Add(-deletedRetention))
	return int(n1) + len(ids) + int(n3), err
}
