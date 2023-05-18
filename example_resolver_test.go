package ddns_test

import (
	"context"
	"log"
	"os"

	"github.com/Travis-Britz/ddns"
)

func Example_publicIPResolver() {
	cfkey, _ := os.LookupEnv("CLOUDFLARE_ZONE_TOKEN")
	// I'm not vouching for these services, but they do return the IP of the client connection.
	// If possible, run your own and provide the URL here instead.
	services := []string{
		"https://checkip.amazonaws.com/",
		"https://icanhazip.com/", // operated by Cloudflare since ~2021
		"https://ipinfo.io/ip",
	}
	ddnsClient, err := ddns.New("dynamic-ip.example.com",
		ddns.UsingCloudflare(cfkey),
		ddns.UsingWebResolver(services...),
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
