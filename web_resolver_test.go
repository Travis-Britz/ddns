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
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		io.WriteString(w, "192.168.2.1")
	}))
	defer srv.Close()
	wr := ddns.WebResolver(srv.URL)
	res, err := wr.Resolve(context.Background())
	if err != nil {
		t.Fatalf("Request failed: %s", err)
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
	ips := []string{"192.168.2.1", "invalid ip", "192.168.2.1"}
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

func TestHitCount(t *testing.T) {
	// This test should align the behavior of the WebResolver closer to its comment.
	var mu sync.Mutex
	var hits int
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		hits++
		// forcing every request to fail should prevent early returns with in-flight requests
		io.WriteString(w, "invalid ip")
		mu.Unlock()
	}))
	defer srv.Close()

	wrs := []ddns.Resolver{
		ddns.WebResolver(srv.URL),
		ddns.WebResolver(srv.URL, srv.URL),
		ddns.WebResolver(srv.URL, srv.URL, srv.URL),
		ddns.WebResolver(srv.URL, srv.URL, srv.URL, srv.URL),
		ddns.WebResolver(srv.URL, srv.URL, srv.URL, srv.URL, srv.URL),
	}
	for i := 0; i < 5; i++ {
		mu.Lock()
		hits = 0
		mu.Unlock()
		wr := wrs[i]
		_, err := wr.Resolve(context.Background())
		if err == nil {
			t.Fatalf("Expected an error; got err == nil")
		}
		mu.Lock()
		h := hits
		mu.Unlock()
		if i < 3 && h != i+1 {
			t.Fatalf("Expected %d hits; got %d", i+1, h)
		} else if h != 3 {
			t.Fatalf("Expected 3 hits; got %d", h)
		}
	}
}
