package service

import (
	"testing"
)

func TestHashKey_Deterministic(t *testing.T) {
	key := "sk_abc123def456"
	h1 := hashKey(key)
	h2 := hashKey(key)
	if h1 != h2 {
		t.Error("hashKey not deterministic")
	}
	if len(h1) != 64 {
		t.Errorf("hashKey length = %d, want 64", len(h1))
	}
}

func TestHashKey_DifferentInputs(t *testing.T) {
	h1 := hashKey("sk_key1")
	h2 := hashKey("sk_key2")
	if h1 == h2 {
		t.Error("different keys should produce different hashes")
	}
}

func TestGenerateRawKey_Format(t *testing.T) {
	key := generateRawKey()
	if len(key) < 10 {
		t.Errorf("key too short: %q", key)
	}
	if key[:3] != "sk_" {
		t.Errorf("key should start with 'sk_', got %q", key[:3])
	}
}

func TestGenerateRawKey_Unique(t *testing.T) {
	seen := make(map[string]bool)
	for i := 0; i < 100; i++ {
		key := generateRawKey()
		if seen[key] {
			t.Fatalf("duplicate key generated: %q", key)
		}
		seen[key] = true
	}
}
