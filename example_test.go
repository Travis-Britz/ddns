package ddns_test

import (
	"context"
	"io"
	"log"
	"net/http"
	"net/netip"
	"os"
	"time"

	"github.com/Travis-Britz/ddns"
)

func ExampleNew() {
	c, err := ddns.New(
		"dynamic-local-ip.example.com",
		ddns.NewCloudflare(os.Getenv("CLOUDFLARE_ZONE_TOKEN")),
		ddns.UsingResolver(ddns.InterfaceResolver("eth0")),
		ddns.WithLogger(log.New(io.Discard, "", 0)),
		ddns.UsingHTTPClient(http.DefaultClient),
	)
	if err != nil {
		log.Fatalf("error creating ddns client: %s", err)
	}
	// run once:
	err = c.RunDDNS(context.Background())
	if err != nil {
		log.Fatalf("ddns update failed: %s", err)
	}
}

func ExampleWebResolver() {
	// I'm not vouching for these services, but they do return the IP of the client connection.
	// If possible, run your own and provide the URL here instead.
	r := ddns.WebResolver(
		"https://checkip.amazonaws.com/",
		"https://icanhazip.com/", // operated by Cloudflare since ~2021
		"https://ipinfo.io/ip",
	)
	ddnsClient, err := ddns.New(
		"dynamic-ip.example.com",
		ddns.NewCloudflare(os.Getenv("CLOUDFLARE_ZONE_TOKEN")),
		ddns.UsingResolver(r),
	)
	if err != nil {
		log.Fatalf("error creating ddns client: %s", err)
	}
	// run once:
	err = ddnsClient.RunDDNS(context.Background())
	if err != nil {
		log.Fatalf("ddns update failed: %s", err)
	}
}

func ExampleRunDaemon() {
	ddnsClient, err := ddns.New("dynamic-local-ip.example.com",
		ddns.NewCloudflare(os.Getenv("CLOUDFLARE_ZONE_TOKEN")),
	)
	if err != nil {
		log.Fatalf("error creating ddns client: %s", err)
	}

	// run every 5 minutes and stop after an hour:
	ctx, cancel := context.WithTimeout(context.Background(), 1*time.Hour)
	defer cancel()
	ddns.RunDaemon(ddnsClient, ctx, 5*time.Minute, nil)
}
func ExampleInterfaceResolver() {
	resolver := ddns.InterfaceResolver("eth0", "wlan0")
	ddnsClient, err := ddns.New("dynamic-local-ip.example.com",
		ddns.NewCloudflare(os.Getenv("CLOUDFLARE_ZONE_TOKEN")),
		ddns.UsingResolver(resolver),
	)
	if err != nil {
		log.Fatalf("error creating ddns client: %s", err)
	}
	// run once:
	err = ddnsClient.RunDDNS(context.Background())
	if err != nil {
		log.Fatalf("ddns update failed: %s", err)
	}
}

func ExampleJoin() {
	r := ddns.Join(
		ddns.WebResolver("https://ipv4.icanhazip.com/"),
		ddns.WebResolver("https://ipv6.icanhazip.com/"),
	)
	ddnsClient, err := ddns.New("dynamic-ip.example.com",
		ddns.NewCloudflare(os.Getenv("CLOUDFLARE_ZONE_TOKEN")),
		ddns.UsingResolver(r),
	)
	if err != nil {
		log.Fatalf("error creating ddns client: %s", err)
	}
	// run once:
	err = ddnsClient.RunDDNS(context.Background())
	if err != nil {
		log.Fatalf("ddns update failed: %s", err)
	}
}
func ExampleResolverFunc() {
	fn := func(ctx context.Context) ([]netip.Addr, error) {
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-time.After(100 * time.Millisecond): // simulating some lookup method
			ip, err := netip.ParseAddr("10.0.0.10")
			return []netip.Addr{ip}, err
		}
	}
	ddnsClient, err := ddns.New("dynamic-ip.example.com",
		ddns.NewCloudflare(os.Getenv("CLOUDFLARE_ZONE_TOKEN")),
		ddns.UsingResolver(ddns.ResolverFunc(fn)),
	)
	if err != nil {
		log.Fatalf("error creating ddns client: %s", err)
	}
	// run once:
	err = ddnsClient.RunDDNS(context.Background())
	if err != nil {
		log.Fatalf("ddns update failed: %s", err)
	}
}
