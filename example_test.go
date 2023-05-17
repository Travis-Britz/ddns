package ddns_test

import (
	"context"
	"log"
	"os"
	"time"

	"github.com/Travis-Britz/ddns"
)

func Example() {
	cfkey, _ := os.LookupEnv("CLOUDFLARE_ZONE_TOKEN")
	ddnsClient, err := ddns.New("dynamic-local-ip.example.com", ddns.UsingCloudflare(cfkey))
	if err != nil {
		log.Fatalf("error creating ddns client: %s", err)
	}
	// run once:
	err = ddnsClient.Run(context.Background())
	if err != nil {
		log.Fatalf("ddns update failed: %s", err)
	}

	// run every 5 minutes and stop after an hour:
	ctx, cancel := context.WithTimeout(context.Background(), 1*time.Hour)
	defer cancel()
	err = ddnsClient.RunDaemon(ctx, 5*time.Minute)
	if err != nil {
		log.Fatalf("ddns daemon error: %s", err)
	}
}
