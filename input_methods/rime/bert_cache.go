package rime

import (
	"sync"
	"time"
)

const (
	defaultBertCacheTTL        = 2 * time.Minute
	defaultBertFailureCacheTTL = 3 * time.Second
)

type bertCachedResult struct {
	Result    bertRerankResult
	ExpiresAt time.Time
}

type bertRerankCache struct {
	mu      sync.Mutex
	ttl     time.Duration
	entries map[string]bertCachedResult
}

func newBertRerankCache(ttl time.Duration) *bertRerankCache {
	if ttl <= 0 {
		ttl = defaultBertCacheTTL
	}
	return &bertRerankCache{
		ttl:     ttl,
		entries: make(map[string]bertCachedResult),
	}
}

func (c *bertRerankCache) Get(key string) (bertRerankResult, bool) {
	if c == nil || key == "" {
		return bertRerankResult{}, false
	}
	now := time.Now()
	c.mu.Lock()
	defer c.mu.Unlock()
	entry, ok := c.entries[key]
	if !ok {
		return bertRerankResult{}, false
	}
	if !entry.ExpiresAt.IsZero() && now.After(entry.ExpiresAt) {
		delete(c.entries, key)
		return bertRerankResult{}, false
	}
	return cloneBertRerankResult(entry.Result), true
}

func (c *bertRerankCache) Put(key string, result bertRerankResult) {
	c.PutWithTTL(key, result, c.ttl)
}

func (c *bertRerankCache) PutWithTTL(key string, result bertRerankResult, ttl time.Duration) {
	if c == nil || key == "" {
		return
	}
	if ttl <= 0 {
		ttl = c.ttl
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	c.pruneLocked(time.Now())
	c.entries[key] = bertCachedResult{
		Result:    cloneBertRerankResult(result),
		ExpiresAt: time.Now().Add(ttl),
	}
}

func (c *bertRerankCache) pruneLocked(now time.Time) {
	for key, entry := range c.entries {
		if !entry.ExpiresAt.IsZero() && now.After(entry.ExpiresAt) {
			delete(c.entries, key)
		}
	}
}
