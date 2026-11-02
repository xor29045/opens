package password

import (
	"context"
	"testing"
)

var testParams = Params{MemoryKiB: 1024, Iterations: 1, Parallelism: 1, SaltLen: 16, KeyLen: 32}

func TestHashVerify(t *testing.T) {
	h := NewHasher(testParams, 2)
	ctx := context.Background()
	enc, err := h.Hash(ctx, "a very long passphrase")
	if err != nil {
		t.Fatal(err)
	}
	ok, rehash, err := h.Verify(ctx, "a very long passphrase", enc)
	if err != nil || !ok || rehash {
		t.Fatalf("verify failed ok=%v rehash=%v err=%v", ok, rehash, err)
	}
	ok, _, _ = h.Verify(ctx, "wrong", enc)
	if ok {
		t.Fatal("wrong password accepted")
	}
	h2 := NewHasher(Params{MemoryKiB: 2048, Iterations: 1, Parallelism: 1, SaltLen: 16, KeyLen: 32}, 1)
	if _, rehash, _ := h2.Verify(ctx, "a very long passphrase", enc); !rehash {
		t.Fatal("expected rehash with different params")
	}
	if _, _, err := h.Verify(ctx, "x", "$bcrypt$foo"); err == nil {
		t.Fatal("expected malformed error")
	}
}

func TestLocalChecker(t *testing.T) {
	if !(LocalChecker{}).IsBreached(context.Background(), "Password123") {
		t.Fatal("expected common password to be breached")
	}
	if (LocalChecker{}).IsBreached(context.Background(), "unlikely-passphrase-xyz-42") {
		t.Fatal("unexpected breach")
	}
}

// BenchmarkHashDefault documents the cost of the production parameters
// (target ≈ 250 ms per hash on the production host, §12.6 / T5.3).
func BenchmarkHashDefault(b *testing.B) {
	h := NewHasher(DefaultParams, 1)
	for i := 0; i < b.N; i++ {
		_, _ = h.Hash(context.Background(), "benchmark passphrase")
	}
}
