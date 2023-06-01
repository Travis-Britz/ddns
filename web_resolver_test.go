package ddns_test

import (
	"io"
	"net/http"
	"net/http/httptest"
	"net/netip"
	"sync"
	"testing"
	"time"

	"github.com/Travis-Britz/ddns"
	"golang.org/x/net/context"
)

func TestLookup(t *testing.T) {
	hits := 0
	var mu sync.Mutex
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		hits++
		mu.Unlock()
		io.WriteString(w, "192.168.2.1")
	}))
	defer srv.Close()
	wr := ddns.WebResolver(srv.URL)
	res, err := wr.Resolve(context.Background())
	if err != nil {
		t.Fatalf("Request failed: %s", err)
	}
	if hits != 1 {
		t.Errorf("Expected 1 hit; got %d", hits)
	}
	if expected, got := netip.MustParseAddr("192.168.2.1"), res[0]; expected != got {
		t.Fatalf("Expected %q; got %q", expected, got)
	}
}

func TestMismatch(t *testing.T) {

	ips := []string{"192.168.2.1", "10.0.0.10", "127.0.0.1"}
	var srvs []string
	for _, ip := range ips {
		ip := ip
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			io.WriteString(w, ip)
		}))
		defer srv.Close()
		srvs = append(srvs, srv.URL)
	}
	wr := ddns.WebResolver(srvs...)
	res, err := wr.Resolve(context.Background())
	if err == nil {
		t.Fatalf("Expected error response; got err == nil")
	}
	if res != nil {
		t.Fatalf("Expected empty slice; got %+v", res)
	}
}

func TestOneFailure(t *testing.T) {
	ips := []string{"192.168.2.1", "a", "192.168.2.1"}
	var srvs []string
	for _, ip := range ips {
		ip := ip
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			io.WriteString(w, ip)
		}))
		defer srv.Close()
		srvs = append(srvs, srv.URL)
	}
	wr := ddns.WebResolver(srvs...)
	res, err := wr.Resolve(context.Background())
	if err != nil {
		t.Fatalf("Resolve failed: %s", err)
	}
	if expected, got := netip.MustParseAddr("192.168.2.1"), res[0]; expected != got {
		t.Fatalf("Expected %q; got %q", expected, got)
	}
}

func TestTwoFailures(t *testing.T) {
	ips := []string{"192.168.2.1", "a", "a"}
	var srvs []string
	for _, ip := range ips {
		ip := ip
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			io.WriteString(w, ip)
		}))
		defer srv.Close()
		srvs = append(srvs, srv.URL)
	}
	wr := ddns.WebResolver(srvs...)
	res, err := wr.Resolve(context.Background())
	if err == nil {
		t.Fatalf("Expected error response; got err == nil")
	}
	if res != nil {
		t.Fatalf("Expected empty slice; got %+v", res)
	}
}

func TestConcurrency(t *testing.T) {
	ips := []string{"192.168.2.1", "192.168.2.1", "192.168.2.1"}
	var srvs []string
	for _, ip := range ips {
		ip := ip
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			time.Sleep(50 * time.Millisecond)
			io.WriteString(w, ip)
		}))
		defer srv.Close()
		srvs = append(srvs, srv.URL)
	}
	wr := ddns.WebResolver(srvs...)
	ctx, cancel := context.WithTimeout(context.Background(), 75*time.Millisecond)
	defer cancel()
	res, err := wr.Resolve(ctx)
	if err != nil {
		t.Fatalf("Resolve failed: %s", err)
	}
	if expected, got := netip.MustParseAddr("192.168.2.1"), res[0]; expected != got {
		t.Fatalf("Expected %q; got %q", expected, got)
	}
}
