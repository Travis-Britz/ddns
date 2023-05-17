package main

import (
	"bufio"
	"context"
	"errors"
	"flag"
	"fmt"
	"io/fs"
	"log"
	"os"
	"path/filepath"
	"strings"
	"syscall"
	"time"

	"github.com/Travis-Britz/ddns"
	"github.com/cloudflare/cloudflare-go"
	"golang.org/x/term"
)

var config = struct {
	Domain string
	// Zone string
	KeyFile string
	// IP       string
	// Interval time.Duration
	Verbose bool
}{}

var (
	resolver ddns.Resolver
	provider ddns.Provider
	logger   *log.Logger
)

func init() {
	flag.StringVar(&config.Domain, "d", config.Domain, "DNS entry to update")
	// flag.stringVar(&config.Zone, "z", "", "The name of the Cloudflare Zone which is managing the domain name")
	// flag.StringVar(&config.IP, "ip", config.Domain, "IP address to set")
	flag.StringVar(&config.KeyFile, "k", filepath.Join(os.Getenv("HOME"), ".cloudflare"), "Path to cloudflare API credentials file")
	// flag.DurationVar(&config.Interval, "i", 1*time.Minute, "Duration to wait between IP checks")
	flag.BoolVar(&config.Verbose, "v", false, "Enable verbose logging")
	flag.Parse()

	if config.Verbose {
		logger = log.Default()
	}
	resolver = &ddns.LocalResolver{}

}

func main() {
	if err := run(); err != nil {
		log.Fatal(err)
	}
}

func run() error {

	if err := validate(); err != nil {
		return fmt.Errorf("run: %w", err)
	}
	logger.Printf("config is valid: %+v", config)

	key, err := readKey(config.KeyFile)
	if err != nil {
		return fmt.Errorf("error reading key: %w", err)
	}
	logger.Println("successfully read key from key file")

	client, err := ddns.New(config.Domain,
		ddns.UsingCloudflare(key),
		ddns.WithLogger(logger),
		ddns.UsingResolver(resolver),
	)
	if err != nil {
		return fmt.Errorf("error creating ddns.Client: %w", err)
	}
	if err := client.Run(context.TODO()); err != nil {
		return fmt.Errorf("run: %w", err)
	}

	return nil
}

func runSetup() error {
	logger.Println("running setup")
	time.Sleep(200 * time.Millisecond) // dirty timer hack to try to get stderr and stdout output lines to display in order
	fmt.Printf("Enter Cloudflare API Key: \n")
	bytekey, err := term.ReadPassword(int(syscall.Stdin))
	if err != nil {
		return fmt.Errorf("runSetup: error reading from stdin: %w", err)
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
		return errors.New("domain cannot be empty")
	}

	if !strings.Contains(config.Domain, ".") {
		return errors.New("domain must have at least one dot")
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

	perms := info.Mode().Perm()
	// Error messages will state that we want 0600,
	// but we'll also accept 0400 which is even more restricted.
	// The file might be provided by some secrets managing software as readonly.
	if perms != 0600 && perms != 0400 {
		return fmt.Errorf("invalid permissions for \"%s\": expected file permissions \"-rw-------\"; found \"%s\"", path, fs.FileMode(perms))
	}

	return nil
}
