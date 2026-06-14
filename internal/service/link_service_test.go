package service

import (
	"testing"

	"github.com/42ShortHub/shortlink/internal/config"
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

func TestValidateURL(t *testing.T) {
	cfg := &config.Config{}
	svc := &LinkService{cfg: cfg}

	tests := []struct {
		url   string
		valid bool
	}{
		{"https://example.com", true},
		{"http://example.com/path?q=1", true},
		{"ftp://example.com", false},
		{"not-a-url", false},
		{"http://localhost/admin", false},
		{"http://127.0.0.1:8080", false},
		{"https://0.0.0.0", false},
		{"", false},
	}

	for _, tt := range tests {
		err := svc.validateURL(tt.url)
		if tt.valid && err != nil {
			t.Errorf("expected valid for %q, got error: %v", tt.url, err)
		}
		if !tt.valid && err == nil {
			t.Errorf("expected invalid for %q, got nil", tt.url)
		}
	}
}
