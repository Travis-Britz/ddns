package ddns_test

import (
	"context"
	"net/netip"
	"testing"
	"time"

	"github.com/Travis-Britz/ddns"
)

func TestConcurrentJoin(t *testing.T) {
	f := ddns.ResolverFunc(func(ctx context.Context) ([]netip.Addr, error) {
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-time.After(25 * time.Millisecond):
			return nil, nil
		}
	})

	r := ddns.Join(f, f, f)
	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()

	_, err := r.Resolve(ctx)
	if err != nil {
		t.Fatalf("Expected concurrent resolvers to finish before context timeout; got %q", err)
	}
}
