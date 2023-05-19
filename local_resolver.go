package ddns

import (
	"context"
	"errors"
	"fmt"
	"net"
	"net/netip"
)

type localResolver struct{}

func (r localResolver) Resolve(ctx context.Context) (addrs []netip.Addr, err error) {
	adds, err := net.InterfaceAddrs()
	if err != nil {
		return nil, fmt.Errorf("error getting addresses for interface: %w", err)
	}
	// addr: ip+net:192.168.86.253/24
	// addr: ip+net:fd64:9f44:fc30:0:b951:8b16:2812:a227/64
	// addr: ip+net:fe80::2cc9:801b:3551:9a43/64
	var parseErrors []error
	for _, addr := range adds {
		ip, err := netip.ParsePrefix(addr.String())
		if err != nil {
			parseErrors = append(parseErrors, fmt.Errorf("error parsing local ip %s: %s", addr.String(), err))
			continue
		}
		if ip.Addr().IsLoopback() {
			continue
		}
		addrs = append(addrs, ip.Addr())
	}
	return addrs, errors.Join(parseErrors...)
}
