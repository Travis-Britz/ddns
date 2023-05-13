# ddns

ddns is a small dynamic DNS command line tool and Go library for updating Cloudflare DNS records.

This project is written specifically to run on the Pi Zero W and update records with the _local_ IP addresses assigned to the Pi. It likely works on any platform supported by Go, however.

## Installation

Place the build binary in your preferred location (with execute permissions) and then run it.

The program will prompt for a Cloudflare [API token](https://dash.cloudflare.com/profile/api-tokens) and then store it to a file. The token must have `Zone.DNS:Edit` permissions.

To skip the prompt you may create the key file in advance with the proper file permissions:

```bash
echo "MyVerySecretDNSToken" > ~/.cloudflare && chmod 600 ~/.cloudflare
```

## Usage

ddns -h:

    Usage of ddns:
    -d string
            The domain name to update
    -k string
            Path to cloudflare API credentials file (default "~/.cloudflare")
    -v    Enable verbose logging

## Examples

Update a domain with the _local_ IPs assigned to the Pi:

```sh
ddns -v -d pi1.example.com
```

## Tips

Configuring devices or the router on the network where this ddns client resides to use Cloudflare's `1.1.1.1` DNS resolver service will further reduce DNS propagation time in the event of IP changes.
