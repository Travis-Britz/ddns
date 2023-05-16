package ddns

import (
	"context"
	"fmt"
	"io"
	"log"
	"net/netip"
	"time"
)

var DefaultResolver = &LocalResolver{}

var discard = log.New(io.Discard, "", log.LstdFlags)

type Resolver interface {
	Resolve(context.Context) ([]netip.Addr, error)
}

type Provider interface {
	SetDNSRecords(ctx context.Context, domain string, records []netip.Addr) error
}

type Cache interface {
	FilterNew([]netip.Addr) (add []netip.Addr, remove []netip.Addr)
}

func New(options ...clientOption) (*Client, error) {
	c := &Client{}
	for i, opt := range options {
		if err := opt(c); err != nil {
			return nil, fmt.Errorf("ddns.New: option %d returned an error: %s", i, err)
		}
	}
	return c, nil
}

type clientOption func(*Client) error

func UsingCloudflare(token string) clientOption {
	return func(c *Client) (err error) {
		if c.Provider, err = NewCloudflareProvider(token); err != nil {
			return fmt.Errorf("error creating cloudflare dns provider: %w", err)
		}
		return nil
	}
}
func UsingResolver(resolver Resolver) clientOption {
	return func(c *Client) error {
		if resolver == nil {
			resolver = DefaultResolver
		}
		c.Resolver = resolver
		return nil
	}
}
func WithLogger(logger *log.Logger) clientOption {
	return func(c *Client) error {
		if logger == nil {
			logger = discard
		}
		c.Log = logger

		switch p := c.Provider.(type) {
		case *CloudflareProvider:
			p.logger = logger
		}

		switch r := c.Resolver.(type) {
		case *LocalResolver:
			r.Logger = logger
		case *WebResolver:
		case *String:

		}
		return nil
	}
}

type Client struct {
	Resolver
	Provider
	Cache
	Log *log.Logger
}

func (c *Client) Run(ctx context.Context, domain string) error {
	newIPs, err := c.Resolve(ctx)
	if err != nil {
		return fmt.Errorf("error getting IPs: %w", err)
	}
	c.log().Printf("got local IPs: %+v\n", newIPs)

	if err := c.SetDNSRecords(ctx, domain, newIPs); err != nil {
		return fmt.Errorf("error updating %s with new IPs: %w", domain, err)
	}
	return nil
}
func (c *Client) RunDaemon(ctx context.Context, domain string, interval time.Duration) error {
	if interval < 1*time.Minute {
		interval = 1 * time.Minute
	}

	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return nil
		case <-ticker.C:
			err := c.Run(ctx, domain)
			if err != nil {
				c.log().Printf("ddns.Client.Run: %s", err)
			}
		}
	}
}

func (c *Client) log() *log.Logger {
	if c.Log == nil {
		c.Log = discard
	}
	return c.Log
}
