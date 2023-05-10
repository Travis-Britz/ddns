package main

import (
	"bufio"
	"context"
	"errors"
	"flag"
	"fmt"
	"io/fs"
	"log"
	"net"
	"net/netip"
	"os"
	"syscall"
	"time"

	"github.com/cloudflare/cloudflare-go"
	"golang.org/x/term"
)

var config = struct {
	Domain  string
	KeyFile string
	IP      string
}{
	KeyFile: "~/.cloudflare",
}

func init() {
	flag.StringVar(&config.Domain, "d", config.Domain, "DNS entry to update")
	flag.StringVar(&config.IP, "ip", config.Domain, "IP address to set")
	flag.StringVar(&config.KeyFile, "k", config.KeyFile, "Path to cloudflare API credentials file")
	flag.Parse()
}

func main() {
	if err := run(); err != nil {
		log.Fatal(err)
	}
}

func run() error {

	if err := validate(); err != nil {
		return err
	}

	key, err := readKey(config.KeyFile)
	if err != nil {
		return fmt.Errorf("error reading key: %w", err)
	}

	newIPs, err := onlyIPv4(getLocalIPAddresses())
	if err != nil {
		return fmt.Errorf("error getting local IPs: %w", err)
	}

	api, err := cloudflare.NewWithAPIToken(key)
	if err != nil {
		return fmt.Errorf("error making api: %w", err)
	}

	if err := setIPs(api, config.Domain, newIPs); err != nil {
		return fmt.Errorf("error updating %s with new IPs: %w", config.Domain, err)
	}

	return nil
}

func setIPs(api *cloudflare.API, domain string, addrs []netip.Addr) error {
	zid, err := api.ZoneIDByName(domain)
	if err != nil {
		return fmt.Errorf("could not find zone \"%s\" by name: %w", domain, err)
	}
	api.UpdateDNSRecord(context.TODO(), cloudflare.ZoneIdentifier(zid), cloudflare.UpdateDNSRecordParams{
		Type:     "",
		Name:     "",
		Content:  "",
		Data:     nil,
		ID:       "",
		Priority: new(uint16),
		TTL:      0,
		Proxied:  new(bool),
		Comment:  "",
		Tags:     []string{},
	})

	api.CreateDNSRecord(context.TODO(), cloudflare.ZoneIdentifier(zid), cloudflare.CreateDNSRecordParams{
		CreatedOn:  time.Time{},
		ModifiedOn: time.Time{},
		Type:       "",
		Name:       "",
		Content:    "",
		Meta:       nil,
		Data:       nil,
		ID:         "",
		ZoneID:     "",
		ZoneName:   "",
		Priority:   new(uint16),
		TTL:        0,
		Proxied:    new(bool),
		Proxiable:  false,
		Locked:     false,
		Comment:    "",
		Tags:       []string{},
	})
}

func runSetup() error {

	fmt.Print("Enter Cloudflare API Key: ")
	bytekey, err := term.ReadPassword(int(syscall.Stdin))
	if err != nil {
		return fmt.Errorf("error reading from stdin: %w", err)
	}
	key := string(bytekey)

	api, err := cloudflare.NewWithAPIToken(key)
	if err != nil {
		return fmt.Errorf("error creating api client: %w", err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	result, err := api.VerifyAPIToken(ctx)
	if err != nil {
		return fmt.Errorf("unable to verify api token: %w", err)
	}
	if result.Status != "active" {
		return fmt.Errorf("expected api token status to be \"active\"; got \"%s\"", result.Status)
	}

	f, err := os.OpenFile(config.KeyFile, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0600)
	if err != nil {
		return fmt.Errorf("unable to create \"%s\": %w", config.KeyFile, err)
	}
	defer f.Close()
	fmt.Fprintln(f, key)

	return nil
}

func env(envvar string, defaultvalue string) string {
	e, found := os.LookupEnv(envvar)
	if found {
		return e
	}
	return defaultvalue
}

func readKey(path string) (key string, err error) {
	f, err := os.Open(path)
	if err != nil {
		return "", fmt.Errorf("error reading key: %w", err)
	}
	defer f.Close()

	r := bufio.NewReader(f)
	keyb, _, err := r.ReadLine()
	if err != nil {
		return "", fmt.Errorf("error reading line: %w", err)
	}
	return string(keyb), nil
}

func validate() error {

	if config.Domain == "" {
		return errors.New("error: domain cannot be empty")
	}

	_, err := os.Stat(config.KeyFile)
	if os.IsNotExist(err) {
		if err := runSetup(); err != nil {
			return fmt.Errorf("setup: %w", err)
		}
	}
	if err := verifyPermissions(config.KeyFile); err != nil {
		return err
	}

	return nil
}

func verifyPermissions(path string) error {

	info, err := os.Stat(path)
	if err != nil {
		return fmt.Errorf("error checking keyfile permissions: %w", err)
	}

	if perms := info.Mode().Perm(); perms != 0600 {
		return fmt.Errorf("invalid permissions for \"%s\": %w", path, permissionError(perms))
	}

	return nil
}

type permissionError fs.FileMode

func (pe permissionError) Error() string {
	return fmt.Sprintf("expected file permissions \"-rw-------\"; found \"%s\"", fs.FileMode(pe))
}

func getLocalIPAddresses() (addrs []netip.Addr, err error) {
	adds, err := net.InterfaceAddrs()
	if err != nil {
		return nil, fmt.Errorf("error getting addresses for interface: %w", err)
	}
	// addr: ip+net:192.168.86.253/24
	// addr: ip+net:fd64:9f44:fc30:0:b951:8b16:2812:a227/64
	// addr: ip+net:fe80::2cc9:801b:3551:9a43/64
	var hasErrors bool
	for _, addr := range adds {
		ip, err := netip.ParsePrefix(addr.String())
		if err != nil {
			hasErrors = true
			log.Printf("error parsing local ip %s: %s", addr.String(), err)
			continue
		}
		if ip.Addr().IsLoopback() {
			continue
		}
		addrs = append(addrs, ip.Addr())
	}

	if hasErrors {
		return addrs, errors.New("errors were encountered while retrieving local IP addresses - see log for details")
	}

	return addrs, nil
}

func onlyIPv4(addrs []netip.Addr, e error) (filtered []netip.Addr, err error) {
	if e != nil {
		return nil, e
	}

	for _, a := range addrs {
		if a.Is4() {
			filtered = append(filtered, a)
		}
	}
	return filtered, nil
}
