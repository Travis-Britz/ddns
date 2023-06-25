This project was written specifically to run on the Raspberry Pi Zero W and update records with the _local_ IP addresses assigned to the Pi. It likely works on any platform supported by Go, however.

# ddns

[![Go Reference](https://pkg.go.dev/badge/github.com/Travis-Britz/ddns.svg)](https://pkg.go.dev/github.com/Travis-Britz/ddns)
[![Go](https://github.com/Travis-Britz/ddns/actions/workflows/go.yml/badge.svg)](https://github.com/Travis-Britz/ddns/actions/workflows/go.yml)

ddns is a small Go library for dynamically updating DNS records.

Currently the only DNS provider included is Cloudflare,
but the [ddns.Provider](https://pkg.go.dev/github.com/Travis-Britz/ddns#Provider) interface is a single method if you would like to wrap your own provider's API.

```go
package main

import (
	"context"
	"log"
	"os"

	"github.com/Travis-Britz/ddns"
)

func main() {
	c, err := ddns.New(
		"dynamic-local-ip.example.com",
		ddns.NewCloudflare(os.Getenv("CLOUDFLARE_ZONE_TOKEN")),
		ddns.UsingResolver(ddns.InterfaceResolver("eth0")),
	)
	if err != nil {
		log.Fatalf("error creating ddns client: %s", err)
	}
	ctx := context.Background()
	err = c.RunDDNS(ctx)
	if err != nil {
		log.Fatalf("dns update failed: %s", err)
	}
}

```

More examples: https://pkg.go.dev/github.com/Travis-Britz/ddns#pkg-examples

## ddnscf

ddnscf is a small command line tool for dynamically updating Cloudflare DNS records.

### Installation

Using go install:

```bash
go install github.com/Travis-Britz/ddns/cmd/ddnscf@latest
```

Other:

build ddns/cmd/ddnscf and then move the build binary to your preferred location (with execute permissions).

Once the program is in place, run it. It will prompt for a Cloudflare [API token](https://dash.cloudflare.com/profile/api-tokens) and then store it to a file. The token must have `Zone.DNS:Edit` permissions.

To skip the prompt you may create the key file in advance with the proper file permissions:

```bash
echo "MyVerySecretDNSToken" > ~/.cloudflare && chmod 600 ~/.cloudflare
```

### Usage

ddnscf -h:

    Usage of ddnscf:
    -d string
            The domain name to update
    -k string
            Path to cloudflare API credentials file (default "~/.cloudflare")
    -ip string
            Set a specific IP address
    -url string
            Use a public IP lookup URL
    -if string
            Use a specific network interface
    -i string
            Interval duration between runs (default 5m0s)
    -once
            Run once and exit
    -v
            Enable verbose logging

### Examples

Update a domain with all of the _local_ IPs assigned to the Pi:

```sh
ddnscf -v -d pi1.example.com
```

Update a domain with the IPs for a specific network interface:

```sh
ddnscf -v -d pi1.example.com -if wlan0
```

Update a domain with our public IP (using a lookup service):

```sh
ddnscf -v -d pi1.example.com -url https://ipv4.icanhazip.com
```

`url` must speak HTTP and respond "200 OK" with an IP address as the first line of the response body.

Update a domain once with a specific IP:

```sh
ddnscf -v -d pi1.example.com -ip 192.168.0.2 -once
```

Update a domain every minute:

```sh
ddnscf -v -d pi1.example.com -i 1m
```

## Systemd Service

Create the service file:

```sh
cd /lib/systemd/system
sudo touch ddnscf.service
```

Add the contents:

```ini
[Unit]
Description=Keep DNS records updated
After=network-online.target
Wants=network-online.target

[Service]
ExecStart=/usr/bin/ddnscf -d pi1.example.com -k /home/pi/.cloudflare
User=pi

[Install]
WantedBy=multi-user.target
```

```sh
sudo systemctl daemon-reload
sudo systemctl enable ddnscf.service
```

## Tips

Configuring devices or the router on the network where this ddns client resides to use Cloudflare's `1.1.1.1` DNS resolver service will further reduce DNS propagation time in the event of IP changes.
