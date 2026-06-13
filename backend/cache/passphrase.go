package cache

import (
	"context"
	"sync"
	"time"
)

type entry struct {
	passphrase string
	expiresAt  time.Time
}

// PassphraseCache is an in-memory TTL cache for PGP key passphrases.
type PassphraseCache struct {
	mu      sync.Mutex
	entries map[string]*entry
	ttl     time.Duration
}

// NewPassphraseCache creates a new cache with the given TTL.
// The background eviction goroutine is controlled by ctx.
func NewPassphraseCache(ctx context.Context, ttl time.Duration) *PassphraseCache {
	c := &PassphraseCache{
		entries: make(map[string]*entry),
		ttl:     ttl,
	}
	go c.evictLoop(ctx)
	return c
}

// Set stores a passphrase for the given fingerprint.
func (c *PassphraseCache) Set(fingerprint, passphrase string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.entries[fingerprint] = &entry{
		passphrase: passphrase,
		expiresAt:  time.Now().Add(c.ttl),
	}
}

// Get retrieves a passphrase for the given fingerprint, or empty string if not found.
func (c *PassphraseCache) Get(fingerprint string) string {
	c.mu.Lock()
	defer c.mu.Unlock()
	e, ok := c.entries[fingerprint]
	if !ok {
		return ""
	}
	if time.Now().After(e.expiresAt) {
		delete(c.entries, fingerprint)
		return ""
	}
	return e.passphrase
}

// Has returns true if a non-expired entry exists for the given fingerprint.
func (c *PassphraseCache) Has(fingerprint string) bool {
	c.mu.Lock()
	defer c.mu.Unlock()
	e, ok := c.entries[fingerprint]
	if !ok {
		return false
	}
	if time.Now().After(e.expiresAt) {
		delete(c.entries, fingerprint)
		return false
	}
	return true
}

// Clear removes all cached entries.
func (c *PassphraseCache) Clear() {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.entries = make(map[string]*entry)
}

// evictLoop periodically removes expired entries.
// Runs until ctx is cancelled.
func (c *PassphraseCache) evictLoop(ctx context.Context) {
	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			c.mu.Lock()
			now := time.Now()
			for fp, e := range c.entries {
				if now.After(e.expiresAt) {
					delete(c.entries, fp)
				}
			}
			c.mu.Unlock()
		}
	}
}
