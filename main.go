package main

import (
	"bufio"
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"io/fs"
	"log"
	"net"
	"net/netip"
	"os"
	"path/filepath"
	"strings"
	"syscall"
	"time"

	"github.com/cloudflare/cloudflare-go"
	"golang.org/x/term"
)

var config = struct {
	Domain   string
	KeyFile  string
	IP       string
	Interval time.Duration
	Verbose  bool
}{}

func init() {
	flag.StringVar(&config.Domain, "d", config.Domain, "DNS entry to update")
	flag.StringVar(&config.IP, "ip", config.Domain, "IP address to set")
	flag.StringVar(&config.KeyFile, "k", filepath.Join(os.Getenv("HOME"), ".cloudflare"), "Path to cloudflare API credentials file")
	flag.DurationVar(&config.Interval, "i", 1*time.Minute, "Duration to wait between IP checks")
	flag.BoolVar(&config.Verbose, "v", false, "Enable verbose logging")
	flag.Parse()

	if config.Verbose {
		logger = log.Default()
	}
}

var logger *log.Logger = log.New(io.Discard, "", log.LstdFlags)

func main() {
	if err := run(); err != nil {
		log.Fatal(err)
	}
}

func run() error {

	if err := validate(); err != nil {
		return err
	}
	logger.Printf("config is valid: %+v", config)

	key, err := readKey(config.KeyFile)
	if err != nil {
		return fmt.Errorf("error reading key: %w", err)
	}
	logger.Println("successfully read key from key file")

	newIPs, err := getLocalIPAddresses()
	if err != nil {
		return fmt.Errorf("error getting local IPs: %w", err)
	}
	logger.Printf("got local IPs: %+v\n", newIPs)

	api, err := cloudflare.NewWithAPIToken(key)
	if err != nil {
		return fmt.Errorf("error making api: %w", err)
	}

	if err := setIPs(api, config.Domain, newIPs); err != nil {
		return fmt.Errorf("error updating %s with new IPs: %w", config.Domain, err)
	}

	return nil
}
func recordType(a netip.Addr) string {
	if a.Is4() {
		return "A"
	}
	if a.Is6() {
		return "AAAA"
	}
	panic("unknown ip configuration")
}
func setIPs(api *cloudflare.API, domain string, addrs []netip.Addr) error {

	sl := strings.Split(domain, ".")
	zone := strings.Join(sl[len(sl)-2:], ".")
	logger.Printf("looking up zone ID for %s...\n", zone)
	zid, err := api.ZoneIDByName(zone)
	if err != nil {
		return fmt.Errorf("unable to get zone ID for %s: %w", zone, err)
	}
	logger.Printf("got zone ID: %s\n", zid)
	logger.Printf("looking up A,AAAA records for zone %s...\n", zid)

	records, _, err := api.ListDNSRecords(context.Background(), cloudflare.ZoneIdentifier(zid), cloudflare.ListDNSRecordsParams{
		Type:    "A,AAAA",
		Name:    domain,
		Content: "",
		Comment: "",
	})
	logger.Printf("found %d existing records: %+v\n", len(records), records)
	existing := map[netip.Addr]bool{}
	newAddrs := map[netip.Addr]bool{}

	for _, a := range addrs {
		newAddrs[a] = true
	}
	for _, r := range records {
		a, err := netip.ParseAddr(r.Content)
		if err != nil {
			return fmt.Errorf("error parsing IP from content: %w", err)
		}
		existing[a] = true

		if _, found := newAddrs[a]; found {
			logger.Printf("existing record %s is in the set of new addrs\n", a)
			continue
		}

		logger.Printf("deleting DNS record for %s...\n", a)
		err = api.DeleteDNSRecord(context.TODO(), cloudflare.ZoneIdentifier(zid), r.ID)
		if err != nil {
			return fmt.Errorf("unable to delete DNS record %s: %w", r.ID, err)
		}
		logger.Printf("successfully deleted record for %s\n", a)
	}

	for _, a := range addrs {
		if _, found := existing[a]; found {
			logger.Printf("record already exists for %s\n", a)
			continue
		}
		logger.Printf("creating record for %s...", a)
		record, err := api.CreateDNSRecord(context.TODO(), cloudflare.ZoneIdentifier(zid), cloudflare.CreateDNSRecordParams{
			Type:    recordType(a),
			Name:    domain,
			Content: a.String(),
			ZoneID:  zid,
			TTL:     60,
			Comment: "managed by ddns",
		})
		if err != nil {
			return fmt.Errorf("error creating DNS record: %w", err)
		}
		logger.Printf("successfully added record: %+v\n", record)
	}

	return nil
}

func runSetup() error {
	logger.Println("running setup")
	time.Sleep(200 * time.Millisecond)
	fmt.Printf("Enter Cloudflare API Key: \n")
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
	logger.Println("verifying token...")
	result, err := api.VerifyAPIToken(ctx)
	if err != nil {
		return fmt.Errorf("unable to verify api token: %w", err)
	}
	if result.Status != "active" {
		return fmt.Errorf("expected api token status to be \"active\"; got \"%s\"", result.Status)
	}
	logger.Println("token verified successfully")

	logger.Printf("creating key file at \"%s\"\n", config.KeyFile)
	f, err := os.OpenFile(config.KeyFile, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0600)
	if err != nil {
		return fmt.Errorf("unable to create \"%s\": %w", config.KeyFile, err)
	}
	defer f.Close()
	fmt.Fprintln(f, key)
	logger.Printf("token written to \"%s\"\n", config.KeyFile)
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

	if !strings.Contains(config.Domain, ".") {
		return errors.New("error: domain must have at least one dot")
	}

	_, err := os.Stat(config.KeyFile)
	if os.IsNotExist(err) {
		logger.Printf("key file \"%s\" does not exist\n", config.KeyFile)
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
