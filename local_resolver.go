package ddns

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log"
	"net"
	"net/netip"
)

type LocalResolver struct {
	Logger *log.Logger
}

func (r *LocalResolver) Resolve(ctx context.Context) (addrs []netip.Addr, err error) {
	if r.Logger == nil {
		r.Logger = log.New(io.Discard, "", log.LstdFlags)
	}
	adds, err := net.InterfaceAddrs()
	if err != nil {
		return nil, fmt.Errorf("error getting addresses for interface: %w", err)
	}
	// addr: ip+net:192.168.86.253/24
	// addr: ip+net:fd64:9f44:fc30:0:b951:8b16:2812:a227/64
	// addr: ip+net:fe80::2cc9:801b:3551:9a43/64
	var hasErrors bool
	for _, addr := range adds {
		ip, err := netip.ParsePrefix(addr.String())
		if err != nil {
			hasErrors = true
			r.Logger.Printf("error parsing local ip %s: %s", addr.String(), err)
			continue
		}
		if ip.Addr().IsLoopback() {
			continue
		}
		addrs = append(addrs, ip.Addr())
	}

	if hasErrors {
		return addrs, errors.New("errors were encountered while retrieving local IP addresses - see logger for details")
	}

	return addrs, nil
}
