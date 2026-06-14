package service

import (
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/42ShortHub/shortlink/internal/config"
	"github.com/42ShortHub/shortlink/internal/model"
)

func TestRandomBase62(t *testing.T) {
	cfg := &config.Config{App: config.AppConfig{ShortCodeLen: 6}}
	svc := &LinkService{cfg: cfg}

	seen := make(map[string]bool)
	for i := 0; i < 1000; i++ {
		code := svc.randomBase62(6)
		if len(code) != 6 {
			t.Fatalf("expected length 6, got %d", len(code))
		}
		for _, c := range code {
			if !((c >= '0' && c <= '9') || (c >= 'A' && c <= 'Z') || (c >= 'a' && c <= 'z')) {
				t.Fatalf("invalid character: %c", c)
			}
		}
		if seen[code] {
			t.Logf("collision at iteration %d (expected for small space)", i)
		}
		seen[code] = true
	}
}

func TestValidateURL_Basic(t *testing.T) {
	cfg := &config.Config{}
	svc := &LinkService{cfg: cfg}

	tests := []struct {
		url   string
		valid bool
		desc  string
	}{
		{"https://example.com", true, "normal https"},
		{"http://example.com/path?q=1", true, "http with path and query"},
		{"ftp://example.com", false, "ftp scheme blocked"},
		{"not-a-url", false, "not a URL"},
		{"", false, "empty string"},
		{"https://", false, "scheme only no host"},
	}

	for _, tt := range tests {
		err := svc.validateURL(tt.url)
		if tt.valid && err != nil {
			t.Errorf("[%s] expected valid for %q, got error: %v", tt.desc, tt.url, err)
		}
		if !tt.valid && err == nil {
			t.Errorf("[%s] expected invalid for %q, got nil", tt.desc, tt.url)
		}
	}
}

func TestValidateURL_PrivateIPs(t *testing.T) {
	cfg := &config.Config{}
	svc := &LinkService{cfg: cfg}

	blocked := []struct {
		url  string
		desc string
	}{
		{"http://localhost/admin", "localhost"},
		{"http://127.0.0.1:8080", "loopback 127.0.0.1"},
		{"http://127.0.0.2/path", "loopback 127.x"},
		{"https://10.0.0.1", "RFC1918 10.x"},
		{"https://10.255.255.255", "RFC1918 10.x max"},
		{"http://172.16.0.1", "RFC1918 172.16.x"},
		{"http://172.31.255.255", "RFC1918 172.31.x"},
		{"http://192.168.1.1", "RFC1918 192.168.x"},
		{"http://192.168.0.100:3000", "RFC1918 with port"},
		{"http://169.254.1.1", "link-local"},
		{"http://0.0.0.0", "unspecified"},
		{"http://224.0.0.1", "multicast"},
		{"http://240.0.0.1", "reserved class E"},
		{"http://100.64.0.1", "CGNAT shared space"},
		{"http://192.0.2.1", "TEST-NET-1"},
		{"http://198.51.100.1", "TEST-NET-2"},
		{"http://203.0.113.1", "TEST-NET-3"},
		{"http://[::1]/path", "IPv6 loopback"},
		{"http://[fc00::1]", "IPv6 unique local"},
		{"http://[fe80::1]", "IPv6 link-local"},
		{"http://[ff02::1]", "IPv6 multicast"},
	}

	for _, tt := range blocked {
		err := svc.validateURL(tt.url)
		if err == nil {
			t.Errorf("[%s] %q should be blocked but was allowed", tt.desc, tt.url)
		}
	}
}

func TestValidateURL_PublicIPs(t *testing.T) {
	cfg := &config.Config{}
	svc := &LinkService{cfg: cfg}

	allowed := []struct {
		url  string
		desc string
	}{
		{"https://8.8.8.8", "Google DNS"},
		{"https://1.1.1.1", "Cloudflare DNS"},
		{"http://93.184.216.34", "example.com IP"},
	}

	for _, tt := range allowed {
		err := svc.validateURL(tt.url)
		if err != nil {
			t.Errorf("[%s] %q should be allowed but got error: %v", tt.desc, tt.url, err)
		}
	}
}

func TestRecordAccess_NonBlocking(t *testing.T) {
	cfg := &config.Config{App: config.AppConfig{ShortCodeLen: 6}}
	svc := &LinkService{
		cfg:     cfg,
		logChan: make(chan model.AccessLog, 2),
		stopCh:  make(chan struct{}),
	}

	// Fill the channel
	svc.RecordAccess(1, "1.1.1.1", "ua", "ref")
	svc.RecordAccess(1, "1.1.1.1", "ua", "ref")

	// This should NOT block even though channel is full
	done := make(chan struct{})
	go func() {
		svc.RecordAccess(1, "1.1.1.1", "ua", "ref")
		close(done)
	}()

	select {
	case <-done:
		// good, did not block
	case <-time.After(100 * time.Millisecond):
		t.Fatal("RecordAccess blocked when channel was full")
	}

	if svc.Dropped() != 1 {
		t.Errorf("expected 1 dropped, got %d", svc.Dropped())
	}
}

func TestRecordAccess_ConcurrentDropCount(t *testing.T) {
	cfg := &config.Config{App: config.AppConfig{ShortCodeLen: 6}}
	svc := &LinkService{
		cfg:     cfg,
		logChan: make(chan model.AccessLog, 0), // zero capacity: every send drops
		stopCh:  make(chan struct{}),
	}

	const goroutines = 50
	const perGoroutine = 200

	var wg sync.WaitGroup
	wg.Add(goroutines)
	for i := 0; i < goroutines; i++ {
		go func() {
			defer wg.Done()
			for j := 0; j < perGoroutine; j++ {
				svc.RecordAccess(1, "1.1.1.1", "ua", "ref")
			}
		}()
	}
	wg.Wait()

	expected := int64(goroutines * perGoroutine)
	got := atomic.LoadInt64(&svc.dropped)
	if got != expected {
		t.Errorf("expected dropped=%d, got %d (race condition in counter?)", expected, got)
	}
}
