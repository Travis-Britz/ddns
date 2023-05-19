package ddns

import (
	"context"
	"errors"
	"fmt"
	"net"
	"net/netip"
)

// InterfaceResolver constructs a resolver that returns the IP addresses reported by the given interfaces.
// If no interfaces are provided then all interfaces will be used, but loopback addresses will be skipped.
func InterfaceResolver(iface ...string) Resolver {
	if len(iface) == 0 {
		return localResolver{}
	}
	return interfaceResolver{ifaces: iface}
}

type interfaceResolver struct {
	ifaces []string
}

func (r interfaceResolver) Resolve(ctx context.Context) (addrs []netip.Addr, err error) {
	var errs []error
	for _, ifs := range r.ifaces {
		iface, err := net.InterfaceByName(ifs)
		if err != nil {
			errs = append(errs, fmt.Errorf("error getting interface %s by name: %w", ifs, err))
		}
		a, err := iface.Addrs()
		if err != nil {
			errs = append(errs, fmt.Errorf("error looking up addresses for interface %s: %w", ifs, err))
			continue
		}
		for _, addr := range a {
			ip, err := netip.ParsePrefix(addr.String())
			if err != nil {
				errs = append(errs, fmt.Errorf("error parsing local ip %s for interface %s: %s", addr.String(), ifs, err))
				continue
			}
			if ip.Addr().IsLoopback() {
				continue
			}
			addrs = append(addrs, ip.Addr())
		}
	}
	return addrs, errors.Join(errs...)
}

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
