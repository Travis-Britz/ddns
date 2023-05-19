package ddns

import (
	"context"
	"fmt"
	"net/netip"
)

func FromString(addr string) (Resolver, error) {
	return stringResolver(addr), nil
}

type stringResolver string

func (s stringResolver) Resolve(context.Context) ([]netip.Addr, error) {
	addr, err := netip.ParseAddr(string(s))
	if err != nil {
		return nil, fmt.Errorf("unable to parse IP: %w", err)
	}
	return []netip.Addr{addr}, nil
}
