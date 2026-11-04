package repositories

import (
	"context"
	"time"

	"github.com/google/uuid"
)

// publicationSQL computes the RM-22 projection for the profiles of the given
// establishments (or all when the scope is empty) in one statement:
//  1. manager ACTIVE, linked professional account (if any) ACTIVE;
//  2. establishment not hidden, not deleted;
//  3. entitlement: ACTIVE/TRIALING/PAST_DUE and now < period end + grace,
//     except for a cancellation that took effect at period end;
//  4. profile ACTIVE; 5. not hidden; 6. primary photo;
//  7. max_active_profiles quota (oldest publications keep their slot).
const publicationSQL = `
WITH entitled AS (
    SELECT s.establishment_id, pl.max_active_profiles
    FROM subscriptions s JOIN subscription_plans pl ON pl.id = s.plan_id
    WHERE s.status IN ('ACTIVE','TRIALING','PAST_DUE')
      AND s.current_period_end IS NOT NULL
      AND (
            (s.status IN ('ACTIVE','TRIALING') AND @now < s.current_period_end)
         OR (NOT s.cancel_at_period_end AND @now < COALESCE(s.grace_until, s.current_period_end + make_interval(days => pl.grace_days)))
      )
), elig AS (
    SELECT p.id, p.establishment_id, p.published_at, p.created_at, en.max_active_profiles,
        (    m.status = 'ACTIVE' AND m.deleted_at IS NULL
         AND (p.user_id IS NULL OR EXISTS (SELECT 1 FROM users u WHERE u.id = p.user_id AND u.status = 'ACTIVE' AND u.deleted_at IS NULL))
         AND NOT e.is_hidden AND e.deleted_at IS NULL
         AND en.establishment_id IS NOT NULL
         AND p.status = 'ACTIVE'
         AND NOT p.is_hidden AND p.deleted_at IS NULL
         AND EXISTS (SELECT 1 FROM profile_photos ph WHERE ph.profile_id = p.id AND ph.is_primary)
        ) AS ok
    FROM professional_profiles p
    JOIN establishments e ON e.id = p.establishment_id
    JOIN users m ON m.id = e.manager_user_id
    LEFT JOIN entitled en ON en.establishment_id = p.establishment_id
    WHERE (@all OR p.establishment_id IN @scope)
), ranked AS (
    SELECT id, ok, max_active_profiles, published_at,
        row_number() OVER (PARTITION BY establishment_id, ok ORDER BY published_at NULLS LAST, created_at, id) AS rn
    FROM elig
), final AS (
    SELECT id, published_at, (ok AND (max_active_profiles IS NULL OR rn <= max_active_profiles)) AS pub FROM ranked
)
UPDATE professional_profiles p
SET is_published = f.pub,
    published_at = CASE WHEN f.pub AND NOT p.is_published THEN @now WHEN f.pub THEN p.published_at ELSE NULL END,
    updated_at = CASE WHEN f.pub <> p.is_published THEN @now ELSE p.updated_at END
FROM final f
WHERE p.id = f.id AND (p.is_published <> f.pub OR (f.pub AND p.published_at IS NULL))`

// RecomputePublication refreshes is_published for the given establishments
// (all establishments when ids is empty). It returns the number of changes.
func (r *ProfileRepo) RecomputePublication(ctx context.Context, now time.Time, ids ...uuid.UUID) (int64, error) {
	scope := ids
	if len(scope) == 0 {
		scope = []uuid.UUID{uuid.Nil}
	}
	res := r.db(ctx).Exec(publicationSQL, map[string]any{"now": now, "all": len(ids) == 0, "scope": scope})
	return res.RowsAffected, res.Error
}

// HasEntitlement is the SQL twin of the entitlement CTE above for one establishment.
func (r *ProfileRepo) HasEntitlement(ctx context.Context, establishmentID uuid.UUID, now time.Time) (bool, *int, error) {
	type row struct {
		MaxActiveProfiles *int
	}
	var rows []row
	err := r.db(ctx).Raw(`SELECT pl.max_active_profiles FROM subscriptions s JOIN subscription_plans pl ON pl.id = s.plan_id
		WHERE s.establishment_id = @id AND s.status IN ('ACTIVE','TRIALING','PAST_DUE') AND s.current_period_end IS NOT NULL
		AND ((s.status IN ('ACTIVE','TRIALING') AND @now < s.current_period_end)
		  OR (NOT s.cancel_at_period_end AND @now < COALESCE(s.grace_until, s.current_period_end + make_interval(days => pl.grace_days))))`,
		map[string]any{"id": establishmentID, "now": now}).Scan(&rows).Error
	if err != nil || len(rows) == 0 {
		return false, nil, err
	}
	return true, rows[0].MaxActiveProfiles, nil
}
