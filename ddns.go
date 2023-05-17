package ddns

import (
	"context"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/netip"
	"net/url"
	"time"

	"github.com/cloudflare/cloudflare-go"
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

func New(domain string, options ...clientOption) (DDNSClient, error) {
	if domain == "" {
		return nil, fmt.Errorf("ddns.New: domain cannot be empty")
	}
	c := &client{
		Resolver: DefaultResolver,
		domain:   domain,
	}
	for i, opt := range options {
		if err := opt(c); err != nil {
			return nil, fmt.Errorf("ddns.New: option %d returned an error: %s", i, err)
		}
	}

	if c.Provider == nil {
		return nil, fmt.Errorf("ddns.New: no DNS provider was registered and there is no default option - use ddns.UsingCloudflare or similar")
	}

	// this lets us propagate the logger to dependencies that use one if WithLogger was called before all of the dependencies were registered
	withLogger(c.logger)(c)
	return c, nil
}

type clientOption func(*client) error

func UsingCloudflare(token string) clientOption {
	return func(c *client) (err error) {
		if c.Provider, err = NewCloudflareProvider(token); err != nil {
			return fmt.Errorf("ddns.UsingCloudflare: error creating cloudflare DNS provider: %w", err)
		}
		return nil
	}
}
func UsingResolver(resolver Resolver) clientOption {
	return func(c *client) error {
		if resolver == nil {
			resolver = DefaultResolver
		}
		c.Resolver = resolver
		return nil
	}
}

func UsingWebResolver(serviceURL ...string) clientOption {
	return func(c *client) error {
		var URLs []*url.URL
		for _, u := range serviceURL {
			pu, err := url.Parse(u)
			if err != nil {
				return fmt.Errorf("error parsing URL: %w", err)
			}
			URLs = append(URLs, pu)
		}
		c.Resolver = &webResolver{serviceURLs: URLs}
		return nil
	}
}
func withLogger(logger *log.Logger) clientOption {
	return func(c *client) error {
		if logger == nil {
			logger = discard
		}
		type setLogger interface {
			SetLogger(*log.Logger)
		}

		switch p := c.Provider.(type) {
		case *CloudflareProvider:
			p.logger = logger
		case setLogger:
			p.SetLogger(logger)
		}

		switch r := c.Resolver.(type) {
		case setLogger:
			r.SetLogger(logger)
		case *LocalResolver:
		case *webResolver:
		case *String:
		}

		return nil
	}
}
func WithLogger(logger *log.Logger) clientOption {
	return func(c *client) error {
		c.logger = logger
		return nil
	}
}

func UsingHTTPClient(httpclient *http.Client) clientOption {
	return func(c *client) error {
		if httpclient == nil {
			httpclient = http.DefaultClient
		}
		type setHTTPClient interface {
			SetHTTPClient(*http.Client)
		}
		switch hc := c.Resolver.(type) {
		case *webResolver:
			hc.httpClient = httpclient
		case setHTTPClient:
			hc.SetHTTPClient(httpclient)
		}
		switch p := c.Provider.(type) {
		case *CloudflareProvider:
			cloudflare.HTTPClient(httpclient)(p.api)
		case setHTTPClient:
			p.SetHTTPClient(httpclient)
		}
		return nil
	}
}

type DDNSClient interface {
	Run(ctx context.Context) error
	RunDaemon(ctx context.Context, interval time.Duration) error
}

type client struct {
	Resolver
	Provider
	Cache
	logger *log.Logger
	domain string
}

func (c *client) Run(ctx context.Context) error {
	newIPs, err := c.Resolve(ctx)
	if err != nil {
		return fmt.Errorf("error getting IPs: %w", err)
	}
	c.logger.Printf("got local IPs: %+v\n", newIPs)

	if err := c.SetDNSRecords(ctx, c.domain, newIPs); err != nil {
		return fmt.Errorf("error updating %s with new IPs: %w", c.domain, err)
	}
	return nil
}
func (c *client) RunDaemon(ctx context.Context, interval time.Duration) error {
	if interval < 1*time.Minute {
		interval = 1 * time.Minute
	}

	// This check may or may not turn out to be very useful,
	// but it guards against accidentally running the ddns client in daemon mode for a literal string IP that can never change dynamically.
	if _, ok := c.Resolver.(*String); ok {
		return fmt.Errorf("ddns.Client.RunDaemon: String resolver will never change IPs")
	}

	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return nil
		case <-ticker.C:
			err := c.Run(ctx)
			if err != nil {
				if ctx.Err() == context.Canceled || ctx.Err() == context.DeadlineExceeded {
					return nil
				}
				c.logger.Printf("ddns.Client.Run: %s", err)
			}
		}
	}
}
