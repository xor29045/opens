package services

import (
	"context"
	"time"

	"github.com/google/uuid"
	"github.com/redis/go-redis/v9"
)

// Denylist propagates revocations to every API instance through Redis
// (§12.1): deny:user:{id} cuts all access tokens of a user (suspension,
// deletion), deny:sid:{sid} cuts one session family. Keys live as long as an
// access token can.
type Denylist struct {
	rdb redis.UniversalClient
	ttl time.Duration
}

func (d *Denylist) DenyUser(ctx context.Context, userID uuid.UUID) error {
	return d.rdb.Set(ctx, "deny:user:"+userID.String(), "1", d.ttl).Err()
}

func (d *Denylist) AllowUser(ctx context.Context, userID uuid.UUID) error {
	return d.rdb.Del(ctx, "deny:user:"+userID.String()).Err()
}

func (d *Denylist) DenySessions(ctx context.Context, sids ...uuid.UUID) error {
	if len(sids) == 0 {
		return nil
	}
	pipe := d.rdb.Pipeline()
	for _, sid := range sids {
		pipe.Set(ctx, "deny:sid:"+sid.String(), "1", d.ttl)
	}
	_, err := pipe.Exec(ctx)
	return err
}

// IsDenied reports whether the user or the session is revoked.
func (d *Denylist) IsDenied(ctx context.Context, userID, sid uuid.UUID) (bool, error) {
	n, err := d.rdb.Exists(ctx, "deny:user:"+userID.String(), "deny:sid:"+sid.String()).Result()
	if err != nil {
		return false, err
	}
	return n > 0, nil
}
