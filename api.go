package ddns

import (
	"context"
	"net/netip"
)

type Resolver interface {
	Resolve(context.Context) ([]netip.Addr, error)
}

type Provider interface {
	SetDNSRecords(ctx context.Context, domain string, records []netip.Addr) error
}
