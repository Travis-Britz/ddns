# ddns

ddns is a small dynamic DNS Go library and command line tool for updating Cloudflare DNS records.

This project was written specifically to run on the Pi Zero W and update records with the _local_ IP addresses assigned to the Pi. It likely works on any platform supported by Go, however.

## Installation

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

## Usage

ddnscf -h:

    Usage of ddnscf:
    -d string
            The domain name to update
    -k string
            Path to cloudflare API credentials file (default "~/.cloudflare")
    -v    Enable verbose logging

## Examples

Update a domain with the _local_ IPs assigned to the Pi:

```sh
ddnscf -v -d pi1.example.com
```

## Tips

Configuring devices or the router on the network where this ddns client resides to use Cloudflare's `1.1.1.1` DNS resolver service will further reduce DNS propagation time in the event of IP changes.
