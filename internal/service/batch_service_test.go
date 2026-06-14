package service

import (
	"strings"
	"testing"

	"github.com/42ShortHub/shortlink/internal/model"
)

func TestComputeIdempotencyKey(t *testing.T) {
	links := []model.CreateLinkRequest{
		{URL: "https://example.com/b"},
		{URL: "https://example.com/a"},
	}
	key1 := computeIdempotencyKey(1, links)

	// Same URLs in different order should produce same key
	links2 := []model.CreateLinkRequest{
		{URL: "https://example.com/a"},
		{URL: "https://example.com/b"},
	}
	key2 := computeIdempotencyKey(1, links2)

	if key1 != key2 {
		t.Errorf("idempotency keys should match for same URLs regardless of order")
	}

	// Different user should produce different key
	key3 := computeIdempotencyKey(2, links)
	if key1 == key3 {
		t.Errorf("different users should produce different idempotency keys")
	}

	// Keys should be 64-char hex
	if len(key1) != 64 {
		t.Errorf("key length = %d, want 64", len(key1))
	}
}

func TestParseCSV(t *testing.T) {
	csv := `url,custom_code,title
https://example.com/1,abc,First
https://example.com/2,,Second
https://example.com/3,def,
`
	links, err := parseCSV(strings.NewReader(csv))
	if err != nil {
		t.Fatalf("parseCSV error: %v", err)
	}
	if len(links) != 3 {
		t.Fatalf("got %d links, want 3", len(links))
	}

	if links[0].URL != "https://example.com/1" {
		t.Errorf("links[0].URL = %q", links[0].URL)
	}
	if links[0].CustomCode != "abc" {
		t.Errorf("links[0].CustomCode = %q", links[0].CustomCode)
	}
	if links[1].CustomCode != "" {
		t.Errorf("links[1].CustomCode = %q, want empty", links[1].CustomCode)
	}
}

func TestParseCSV_MissingURLColumn(t *testing.T) {
	csv := `name,code
foo,bar
`
	_, err := parseCSV(strings.NewReader(csv))
	if err == nil {
		t.Error("expected error for missing 'url' column")
	}
}

func TestParseCSV_EmptyRows(t *testing.T) {
	csv := `url,title
https://example.com/1,First
,,
https://example.com/2,Second
`
	links, err := parseCSV(strings.NewReader(csv))
	if err != nil {
		t.Fatalf("parseCSV error: %v", err)
	}
	if len(links) != 2 {
		t.Errorf("got %d links, want 2 (empty row skipped)", len(links))
	}
}
