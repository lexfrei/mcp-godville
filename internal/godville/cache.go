package godville

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"sync"
	"time"

	"github.com/cockroachdb/errors"
	"golang.org/x/sync/singleflight"
)

// fetchTimeout bounds the upstream fetch initiated by a singleflight winner.
// The fetch is detached from caller contexts, so without an explicit bound a
// stuck upstream would hold the singleflight slot forever.
const fetchTimeout = 30 * time.Second

// HeroFetcher is the subset of Client used by Cache. Tests can substitute
// their own implementation; production code passes a *Client.
type HeroFetcher interface {
	GetHero(ctx context.Context, godname, userkey string) (*Hero, error)
}

// Cache wraps a HeroFetcher with an in-memory cache keyed by the exact
// (godname, userkey) pair. Godville's data only updates once per minute
// upstream, and the API enforces a rate limit of 30 requests / 10 minutes
// per (god+ip), so aggressive caching is both safe and required. Concurrent
// callers requesting the same (godname, userkey) on a cold key share a
// single upstream call via singleflight to keep within budget.
type Cache struct {
	fetcher HeroFetcher
	ttl     time.Duration

	mu      sync.RWMutex
	entries map[string]*cacheEntry

	group singleflight.Group
}

type cacheEntry struct {
	hero      *Hero
	expiresAt time.Time
}

// NewCache returns a Cache wrapping fetcher with the given TTL.
func NewCache(fetcher HeroFetcher, ttl time.Duration) *Cache {
	return &Cache{
		fetcher: fetcher,
		ttl:     ttl,
		entries: make(map[string]*cacheEntry),
	}
}

// GetHero returns the cached Hero if present and fresh, otherwise fetches a
// new one and caches it. Errors are not cached.
//
// Concurrent callers on a cold key collapse to a single upstream request via
// singleflight. The in-flight fetch is detached from any individual caller's
// context via context.WithoutCancel, then bounded by fetchTimeout — so the
// first caller cancelling (e.g. an HTTP client disconnect) does not poison
// the fetch result for every other waiter. Each caller still gets to bail
// out of the wait on their own ctx via the outer select.
func (c *Cache) GetHero(ctx context.Context, godname, userkey string) (*Hero, error) {
	key := cacheKey(godname, userkey)

	hero, ok := c.lookup(key)
	if ok {
		return hero, nil
	}

	resultCh := c.group.DoChan(key, func() (any, error) {
		if cached, hit := c.lookup(key); hit {
			return cached, nil
		}

		fetchCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), fetchTimeout)
		defer cancel()

		fetched, fetchErr := c.fetcher.GetHero(fetchCtx, godname, userkey)
		if fetchErr != nil {
			return nil, errors.Wrap(fetchErr, "fetch hero")
		}

		if fetched == nil {
			// HeroFetcher is exported; a custom implementation that
			// returned (nil, nil) would poison the cache and panic every
			// downstream consumer. Treat it as a fetch error and refuse
			// to store.
			return nil, errors.New("fetcher returned nil hero with no error")
		}

		c.store(key, fetched)

		return fetched, nil
	})

	select {
	case res := <-resultCh:
		if res.Err != nil {
			return nil, res.Err
		}

		hero, ok = res.Val.(*Hero)
		if !ok {
			return nil, errors.New("unexpected cache value type")
		}

		return hero, nil
	case <-ctx.Done():
		return nil, errors.Wrap(ctx.Err(), "fetch cancelled")
	}
}

// Invalidate clears all cached entries.
func (c *Cache) Invalidate() {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.entries = make(map[string]*cacheEntry)
}

// lookup returns the cached hero if present and fresh. Expired entries are
// evicted on encounter so the map does not grow unboundedly under HTTP
// transport with per-client elicited credentials.
func (c *Cache) lookup(key string) (*Hero, bool) {
	c.mu.RLock()
	entry, ok := c.entries[key]
	c.mu.RUnlock()

	if !ok {
		return nil, false
	}

	if time.Now().After(entry.expiresAt) {
		c.mu.Lock()

		current, stillThere := c.entries[key]
		if stillThere && current == entry {
			delete(c.entries, key)
		}

		c.mu.Unlock()

		return nil, false
	}

	return entry.hero, true
}

func (c *Cache) store(key string, hero *Hero) {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.entries[key] = &cacheEntry{
		hero:      hero,
		expiresAt: time.Now().Add(c.ttl),
	}
}

// cacheKey derives a stable key from (godname, userkey). The userkey is
// hashed so secrets do not appear as raw map keys; godname is kept literal
// for debuggability. Distinct non-empty userkeys MUST produce distinct keys
// to prevent cross-tenant data leakage when the server fronts multiple
// callers (e.g. via HTTP transport with per-client elicited credentials).
// The full sha256 digest is used (not a truncated prefix) — there is no
// upside to shortening it.
func cacheKey(godname, userkey string) string {
	if userkey == "" {
		return "public:" + godname
	}

	sum := sha256.Sum256([]byte(userkey))

	return "private:" + godname + ":" + hex.EncodeToString(sum[:])
}
