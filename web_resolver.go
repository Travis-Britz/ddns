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

// WebResolver constructs a resolver which uses external web services to look up a "public" IP address.
//
// Each serviceURL must speak HTTP and return status "200 OK",
// with a valid IPv4 or IPv6 address as the first line of the response body.
// All other responses are considered an error.
//
// If only one serviceURL is given,
// then the resolver will simply return the response.
// If multiple are given,
// then the resolver will request from up to three of them and only return successfully if the first two non-error responses agreed on the IP.
// No addresses will be returned if the web services did not agree on the IP address.
// This approach is taken due to the sensitive nature of public services having control over DNS records.
// It is recommended to run your own service over https instead when possible.
//
// For clients which have both IPv4 and IPv6 capability,
// it is possible for one service to return IPv4 and another to return IPv6,
// causing matching to fail.
// There are at least two ways to ensure both responses use the same protocol version:
// supply a custom *http.Client (using [ddns.WithHTTPClient]) with a custom http.Transport which is configured to use IPv4/6,
// or simply use a public IP service endpoint that prefers one or the other, e.g. https://ipv4.icanhazip.com.
//
// If you want both IPv4 and IPv6 DNS records set,
// then use one of the above approaches to ensure IPv4 and IPv6 respectively for each of two web resolvers and then use [ddns.Join] to combine their results.
//
// The http.Client used to make requests can be configured in ddns.New's clientOptions with [ddns.UsingHTTPClient].
func WebResolver(serviceURL ...string) Resolver {
	return &webResolver{serviceURLs: serviceURL}
}

type webResolver struct {
	httpClient  *http.Client
	serviceURLs []string
}

func (wr *webResolver) Resolve(ctx context.Context) ([]netip.Addr, error) {
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
	// todo: are there cases where one request is made over ipv4 and one over ipv6? one solution is to hit each resolver with both ipv4/6 and return both
	if wr.serviceURLs == nil {
		return nil, errors.New("no external IP lookup services were provided")
	}

	var URLs []*url.URL
	for _, u := range wr.serviceURLs {
		pu, err := url.Parse(u)
		if err != nil {
			return nil, fmt.Errorf("error parsing URL \"%s\": %w", u, err)
		}
		URLs = append(URLs, pu)
	}

	var useCount, waitFor int
	switch len(URLs) {
	case 1:
		useCount, waitFor = 1, 1
	case 2:
		useCount, waitFor = 2, 2
	default:
		useCount, waitFor = 3, 2
	}

	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	type result struct {
		addr netip.Addr
		err  error
	}

	results := make(chan result, useCount)

	resolvercount := len(URLs)
	var wg sync.WaitGroup
	wg.Add(useCount)
	for i := 0; i < useCount; i++ {
		u := URLs[i%resolvercount]
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
			if waitFor == 1 {
				return []netip.Addr{ip}, nil
			}
			continue
		}
		if ip == r.addr {
			return []netip.Addr{ip}, nil
		}
	}
	if resultCount < waitFor {
		return nil, fmt.Errorf("not enough resolvers responded without errors: %w", errors.Join(errs...))
	}

	return nil, errors.New("IP resolvers did not agree on our IP")

}

func (wr *webResolver) lookup(ctx context.Context, url *url.URL) (netip.Addr, error) {
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

	httpclient := wr.httpClient
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
