package ddns_test

import (
	"context"
	"io"
	"log"
	"net/http"
	"os"
	"time"

	"github.com/Travis-Britz/ddns"
)

func ExampleNew() {
	cfkey, _ := os.LookupEnv("CLOUDFLARE_ZONE_TOKEN")
	c, err := ddns.New("dynamic-local-ip.example.com",
		ddns.UsingCloudflare(cfkey),
		ddns.UsingResolver(ddns.InterfaceResolver("eth0")),
		ddns.WithLogger(log.New(io.Discard, "", log.LstdFlags)),
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
	cfkey, _ := os.LookupEnv("CLOUDFLARE_ZONE_TOKEN")
	// I'm not vouching for these services, but they do return the IP of the client connection.
	// If possible, run your own and provide the URL here instead.
	r, _ := ddns.WebResolver(
		"https://checkip.amazonaws.com/",
		"https://icanhazip.com/", // operated by Cloudflare since ~2021
		"https://ipinfo.io/ip",
	)
	ddnsClient, err := ddns.New("dynamic-ip.example.com",
		ddns.UsingCloudflare(cfkey),
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
	cfkey, _ := os.LookupEnv("CLOUDFLARE_ZONE_TOKEN")
	ddnsClient, err := ddns.New("dynamic-local-ip.example.com",
		ddns.UsingCloudflare(cfkey),
		ddns.WithLogger(log.Default()),
	)
	if err != nil {
		log.Fatalf("error creating ddns client: %s", err)
	}

	// run every 5 minutes and stop after an hour:
	ctx, cancel := context.WithTimeout(context.Background(), 1*time.Hour)
	defer cancel()
	ddns.RunDaemon(ddnsClient, ctx, 5*time.Minute, nil)
	<-ctx.Done()
}
func ExampleInterfaceResolver() {
	cfkey, _ := os.LookupEnv("CLOUDFLARE_ZONE_TOKEN")
	resolver := ddns.InterfaceResolver("eth0", "wlan0")
	ddnsClient, err := ddns.New("dynamic-local-ip.example.com",
		ddns.UsingCloudflare(cfkey),
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
	cfkey, _ := os.LookupEnv("CLOUDFLARE_ZONE_TOKEN")
	// I'm not vouching for these services, but they do return the IP of the client connection.
	r1, _ := ddns.WebResolver("https://ipv4.icanhazip.com/")
	r2, _ := ddns.WebResolver("https://ipv6.icanhazip.com/")

	ddnsClient, err := ddns.New("dynamic-ip.example.com",
		ddns.UsingCloudflare(cfkey),
		ddns.UsingResolver(ddns.Join(r1, r2)),
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
