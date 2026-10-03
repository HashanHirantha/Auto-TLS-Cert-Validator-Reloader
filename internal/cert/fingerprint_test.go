package cert

import (
	"testing"
)

func TestFingerprintCache_HasChanged(t *testing.T) {
	cache := NewFingerprintCache()

	// First observation — always returns true
	if !cache.HasChanged("ns/secret1", "abc123") {
		t.Error("expected HasChanged=true for first observation")
	}

	// Same fingerprint — returns false
	if cache.HasChanged("ns/secret1", "abc123") {
		t.Error("expected HasChanged=false for same fingerprint")
	}

	// Different fingerprint — returns true
	if !cache.HasChanged("ns/secret1", "def456") {
		t.Error("expected HasChanged=true for changed fingerprint")
	}

	// Verify stored value is updated
	got, ok := cache.Get("ns/secret1")
	if !ok {
		t.Error("expected key to exist in cache")
	}
	if got != "def456" {
		t.Errorf("expected cached fingerprint 'def456', got '%s'", got)
	}
}

func TestFingerprintCache_Delete(t *testing.T) {
	cache := NewFingerprintCache()
	cache.Set("ns/secret1", "abc123")

	cache.Delete("ns/secret1")

	_, ok := cache.Get("ns/secret1")
	if ok {
		t.Error("expected key to be deleted from cache")
	}
}

func TestComputeFingerprint(t *testing.T) {
	data := []byte("test certificate data")
	fp := ComputeFingerprint(data)

	if len(fp) != 64 {
		t.Errorf("expected 64-char hex fingerprint, got %d chars", len(fp))
	}

	// Same input should produce the same fingerprint
	fp2 := ComputeFingerprint(data)
	if fp != fp2 {
		t.Error("expected same fingerprint for same input")
	}

	// Different input should produce a different fingerprint
	fp3 := ComputeFingerprint([]byte("different data"))
	if fp == fp3 {
		t.Error("expected different fingerprint for different input")
	}
}
