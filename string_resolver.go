package ddns

import (
	"context"
	"fmt"
	"net/netip"
)

type String string

func (s *String) Resolve(context.Context) ([]netip.Addr, error) {
	addr, err := netip.ParseAddr(string(*s))
	if err != nil {
		return nil, fmt.Errorf("unable to parse IP: %w", err)
	}
	return []netip.Addr{addr}, nil
}
