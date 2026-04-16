package rime

import (
	"sync"
	"time"
)

const (
	defaultBertSentenceCacheTTL         = 2 * time.Minute
	defaultBertSentenceNegativeCacheTTL = 8 * time.Second
)

type bertSentenceCacheEntry struct {
	Sentences []string
	ExpiresAt time.Time
}

type bertSentenceCachePending struct {
	ready chan struct{}
}

type bertSentenceCandidateCache struct {
	mu      sync.Mutex
	ttl     time.Duration
	entries map[string]bertSentenceCacheEntry
	pending map[string]*bertSentenceCachePending
}

func newBertSentenceCandidateCache(ttl time.Duration) *bertSentenceCandidateCache {
	if ttl <= 0 {
		ttl = defaultBertSentenceCacheTTL
	}
	return &bertSentenceCandidateCache{
		ttl:     ttl,
		entries: make(map[string]bertSentenceCacheEntry),
		pending: make(map[string]*bertSentenceCachePending),
	}
}

func (c *bertSentenceCandidateCache) GetOrCompute(key string, compute func() ([]string, bool)) []string {
	if c == nil || key == "" {
		return nil
	}
	for {
		now := time.Now()
		c.mu.Lock()
		c.pruneLocked(now)
		if entry, ok := c.entries[key]; ok {
			c.mu.Unlock()
			return cloneStringSlice(entry.Sentences)
		}
		if pending, ok := c.pending[key]; ok {
			ready := pending.ready
			c.mu.Unlock()
			<-ready
			continue
		}
		pending := &bertSentenceCachePending{ready: make(chan struct{})}
		c.pending[key] = pending
		c.mu.Unlock()

		sentences, cacheable := compute()
		sentences = normalizeSentenceCacheValues(sentences)
		if !cacheable {
			c.mu.Lock()
			delete(c.pending, key)
			close(pending.ready)
			c.mu.Unlock()
			return nil
		}
		ttl := c.ttl
		if len(sentences) == 0 {
			ttl = defaultBertSentenceNegativeCacheTTL
		}

		c.mu.Lock()
		delete(c.pending, key)
		c.entries[key] = bertSentenceCacheEntry{
			Sentences: cloneStringSlice(sentences),
			ExpiresAt: time.Now().Add(ttl),
		}
		close(pending.ready)
		c.mu.Unlock()
		return sentences
	}
}

func (c *bertSentenceCandidateCache) pruneLocked(now time.Time) {
	for key, entry := range c.entries {
		if !entry.ExpiresAt.IsZero() && now.After(entry.ExpiresAt) {
			delete(c.entries, key)
		}
	}
}

func cloneStringSlice(items []string) []string {
	if len(items) == 0 {
		return nil
	}
	return append([]string(nil), items...)
}
