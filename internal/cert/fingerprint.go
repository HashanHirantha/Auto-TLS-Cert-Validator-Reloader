// Package cert provides X.509 certificate parsing, validation, and fingerprinting.
package cert

import (
	"crypto/sha256"
	"encoding/hex"
	"sync"
)

// FingerprintCache provides a thread-safe in-memory cache for certificate fingerprints.
// It is used to detect fingerprint changes without re-parsing the full certificate
// when the Secret data hasn't changed.
type FingerprintCache struct {
	store sync.Map // key: "namespace/secretName" → value: string (sha256 hex)
}

// NewFingerprintCache creates a new FingerprintCache.
func NewFingerprintCache() *FingerprintCache {
	return &FingerprintCache{}
}

// HasChanged checks whether the fingerprint for the given key has changed.
// Returns true on first observation or when the fingerprint differs from the cached value.
func (c *FingerprintCache) HasChanged(key, newFingerprint string) bool {
	old, loaded := c.store.LoadOrStore(key, newFingerprint)
	if !loaded {
		return true // first time seeing this key
	}
	if old.(string) != newFingerprint {
		c.store.Store(key, newFingerprint)
		return true
	}
	return false
}

// Get returns the cached fingerprint for the given key.
func (c *FingerprintCache) Get(key string) (string, bool) {
	val, ok := c.store.Load(key)
	if !ok {
		return "", false
	}
	return val.(string), true
}

// Set stores a fingerprint for the given key.
func (c *FingerprintCache) Set(key, fingerprint string) {
	c.store.Store(key, fingerprint)
}

// Delete removes a fingerprint from the cache.
func (c *FingerprintCache) Delete(key string) {
	c.store.Delete(key)
}

// ComputeFingerprint computes a SHA-256 fingerprint from raw DER certificate bytes.
func ComputeFingerprint(derBytes []byte) string {
	hash := sha256.Sum256(derBytes)
	return hex.EncodeToString(hash[:])
}
