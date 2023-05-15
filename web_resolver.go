package ddns

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/netip"
	"net/url"
	"strings"
	"sync"
	"time"
)

// WebResolver implements ddns.Resolver to look up our public IP address.
//
// urls must speak http and return status "200 OK" with a valid IPv4 or IPv6 address as the first line of the response body.
// All other responses are considered an error.
type WebResolver struct {
	Client *http.Client
	URLs   []*url.URL
}

// Resolve implements ddns.Resolver.
func (wr *WebResolver) Resolve(ctx context.Context) ([]netip.Addr, error) {
	// IP lookup calls out to three of the public IP resolver urls.
	// It only returns a nil error if the first two non-error responses had matching IPs.
	// This approach has a number of benefits:
	// - faster responses
	// - less likely to be affected by service downtime
	// - safer from wrong results in the event of accidental caching
	// - safer from a single compromised service returning malicious results (assuming all supplied resolvers are https)
	//
	// todo: round-robin or randomize resolver selection. right now it's just using the first three.
	// todo: having less than three services configured will increase traffic to one
	// todo: are there cases where one request is made over ipv4 and one over ipv6?
	if wr.URLs == nil {
		return nil, errors.New("no external IP lookup services were provided")
	}

	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	type result struct {
		addr netip.Addr
		err  error
	}

	results := make(chan result, 2)
	const useCount = 3

	resolvercount := len(wr.URLs)
	var wg sync.WaitGroup
	wg.Add(useCount)
	for i := 0; i < useCount; i++ {
		u := wr.URLs[i%resolvercount]
		go func() {
			defer wg.Done()
			r := result{}
			r.addr, r.err = wr.lookup(ctx, u)

			select {
			case results <- r:
			default:
			}
		}()
	}
	go func() { wg.Wait(); close(results) }()

	resultCount := 0
	var errs []error
	var ip netip.Addr
	for r := range results {
		if r.err != nil {
			errs = append(errs, r.err)
			continue
		}
		resultCount++ // don't increase the result count for errors
		if (ip == netip.Addr{}) {
			ip = r.addr
			continue
		}
		if ip == r.addr {
			return []netip.Addr{ip}, nil
		}
	}
	if resultCount < 2 {
		return nil, fmt.Errorf("not enough resolvers responded without errors: %w", errors.Join(errs...))
	}

	return nil, errors.New("IP resolvers did not agree on our IP")

}

func (wr *WebResolver) lookup(ctx context.Context, url *url.URL) (netip.Addr, error) {
	// 15 seconds is an eternity for the size of the request we're making,
	// but this ensures that all calls to resolve will eventually complete even if the user supplied context.TODO or context.Background
	// using http.DefaultClient (with no timeout).
	ctx, cancel := context.WithTimeout(ctx, 15*time.Second)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url.String(), nil)
	if err != nil {
		return netip.Addr{}, fmt.Errorf("error creating request: %w", err)
	}
	req.Header.Set("Cache-Control", "no-cache")

	httpclient := wr.Client
	if httpclient == nil {
		httpclient = http.DefaultClient
	}

	resp, err := httpclient.Do(req)
	if err != nil {
		return netip.Addr{}, fmt.Errorf("http request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		return netip.Addr{}, fmt.Errorf("http request returned %s", resp.Status)
	}

	scanner := bufio.NewReader(resp.Body)
	ipstring, _ := scanner.ReadString('\n')
	ip, err := netip.ParseAddr(strings.TrimSpace(ipstring))
	if err != nil {
		return netip.Addr{}, fmt.Errorf("error parsing IP address from response body: %w", err)
	}
	return ip, nil
}
