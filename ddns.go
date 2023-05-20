package ddns

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/netip"
	"time"

	"github.com/cloudflare/cloudflare-go"
)

var defaultResolver = InterfaceResolver()

var discard = log.New(io.Discard, "", log.LstdFlags)

// DDNSClient is the interface for updating Dynamic DNS records.
//
// It is implemented by the client returned by ddns.New.
type DDNSClient interface {
	RunDDNS(ctx context.Context) error
}

// Resolver is the interface for looking up our external IP addresses.
//
// Results may be either IPv4 or IPv6,
// but should not include loopback interface addresses such as ::1.
//
// A non-nil error may be returned with partial results.
type Resolver interface {
	Resolve(context.Context) ([]netip.Addr, error)
}

// Provider is the interface for setting DNS records with a DNS provider.
//
// Records may be IPv4 and IPv6 combined,
// and implementations should expect both even if they only use one.
//
// The given records are the desired set for domain.
// It is up to implementations to track changes between calls.
type Provider interface {
	SetDNSRecords(ctx context.Context, domain string, records []netip.Addr) error
}

type cache interface {
	FilterNew([]netip.Addr) (add []netip.Addr, remove []netip.Addr)
}

// New creates a new DDNSClient for domain configured by options.
func New(domain string, options ...clientOption) (DDNSClient, error) {
	if domain == "" {
		return nil, fmt.Errorf("ddns.New: domain cannot be empty")
	}
	c := &client{
		Resolver: defaultResolver,
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
	setLog(c, c.logger)
	return c, nil
}

type clientOption func(*client) error

func UsingCloudflare(token string) clientOption {
	return func(c *client) (err error) {
		if c.Provider, err = newCloudflareProvider(token); err != nil {
			return fmt.Errorf("ddns.UsingCloudflare: error creating cloudflare DNS provider: %w", err)
		}
		return nil
	}
}
func UsingResolver(resolver Resolver) clientOption {
	return func(c *client) error {
		if resolver == nil {
			resolver = defaultResolver
		}
		c.Resolver = resolver
		return nil
	}
}

// WithLogger configures the client with a logger for verbose logging.
//
// The default logger discards verbose log messages.
func WithLogger(logger *log.Logger) clientOption {
	return func(c *client) error {
		c.logger = logger
		return nil
	}
}

// UsingHTTPClient configures the DDNSClient to use the given httpclient for requests made by the Provider and Resolver implementations supplied by this package,
// or for other types if they implement a SetHTTPClient method.
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
		case *cloudflareProvider:
			cloudflare.HTTPClient(httpclient)(p.api)
		case setHTTPClient:
			p.SetHTTPClient(httpclient)
		}
		return nil
	}
}

type client struct {
	Resolver
	Provider
	cache
	logger *log.Logger
	domain string
}

func (c *client) RunDDNS(ctx context.Context) error {
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

type logf interface {
	Printf(string, ...any)
}

// RunDaemon starts a goroutine to run ddnsClient every interval.
//
// Run errors are reported to logger.
// A nil logger indicates messages should be sent to the log package's default log.
//
// To stop the daemon,
// cancel the given context.
func RunDaemon(ddnsClient DDNSClient, ctx context.Context, interval time.Duration, logger logf) {
	if interval < 1*time.Minute {
		interval = 1 * time.Minute
	}
	if logger == nil {
		logger = log.Default()
	}
	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	for {
		err := ddnsClient.RunDDNS(ctx)
		if err != nil {
			logger.Printf("ddns.RunDaemon: %s", err)
		}
		var authentication interface {
			IsAuthenticationError() bool
		}
		if errors.As(err, &authentication) {
			if authentication.IsAuthenticationError() {
				logger.Printf("ddns.RunDaemon: bad credentials detected; stopping daemon")
				return
			}
		}
		var authorization interface {
			IsAuthorizationError() bool
		}
		if errors.As(err, &authorization) {
			if authorization.IsAuthorizationError() {
				logger.Printf("ddns.RunDaemon: credentials are not authorized to perform that action; stopping daemon")
				return
			}
		}
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
		}
	}
}

// Join constructs a resolver that combines the output of multiple resolvers into one.
//
// This is useful in some instances such as when you want records for both IPv4 and IPv6,
// but can only get one or the other from a single web service request.
func Join(resolver ...Resolver) Resolver {
	return &joinResolver{resolvers: resolver}
}

type joinResolver struct {
	resolvers []Resolver
}

func (r joinResolver) Resolve(ctx context.Context) (addrs []netip.Addr, err error) {
	var errs []error
	for _, rr := range r.resolvers {
		a, err := rr.Resolve(ctx)
		addrs = append(addrs, a...)
		errs = append(errs, err)
	}
	return addrs, errors.Join(errs...)
}

func setLog(c *client, logger *log.Logger) {
	if logger == nil {
		logger = discard
	}
	type setLogger interface {
		SetLogger(*log.Logger)
	}

	switch p := c.Provider.(type) {
	case *cloudflareProvider:
		p.logger = logger
	case setLogger:
		p.SetLogger(logger)
	}

	switch r := c.Resolver.(type) {
	case setLogger:
		r.SetLogger(logger)
	case *localResolver:
	case *webResolver:
	case *stringResolver:
	}
}
