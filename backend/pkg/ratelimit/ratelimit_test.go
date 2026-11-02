package ratelimit

import (
	"context"
	"testing"

	"github.com/alicebob/miniredis/v2"
	"github.com/redis/go-redis/v9"
)

func TestAllow(t *testing.T) {
	mr := miniredis.RunT(t)
	l := New(redis.NewClient(&redis.Options{Addr: mr.Addr()}))
	ctx := context.Background()
	for i := 0; i < 5; i++ {
		r, err := l.Allow(ctx, "login:alice", PerMinute(5), PerHour(20))
		if err != nil || !r.Allowed {
			t.Fatalf("attempt %d denied (%v)", i+1, err)
		}
	}
	r, _ := l.Allow(ctx, "login:alice", PerMinute(5), PerHour(20))
	if r.Allowed || r.RetryAfter <= 0 {
		t.Fatal("6th attempt should be denied with Retry-After")
	}
	if r, _ := l.Allow(ctx, "login:bob", PerMinute(5)); !r.Allowed {
		t.Fatal("other key must not be affected")
	}
}

func TestIPKey(t *testing.T) {
	if IPKey("2001:db8:1:2:3:4:5:6") != IPKey("2001:db8:1:2:ffff::1") {
		t.Fatal("same /64 must share a key")
	}
	if IPKey("10.0.0.1") != "10.0.0.1" {
		t.Fatal("ipv4 unchanged")
	}
}
