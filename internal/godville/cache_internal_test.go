package godville

import (
	"context"
	"sync/atomic"
	"testing"
	"time"
)

type recordingFetcher struct{ calls atomic.Int32 }

func (rec *recordingFetcher) GetHero(_ context.Context, _, _ string) (*Hero, error) {
	rec.calls.Add(1)

	return &Hero{}, nil
}

// Regression: expired entries must not linger in the map. Under HTTP
// transport with per-client userkeys, an ever-growing map of stale entries
// is an unbounded-memory path. Lives in the godville package itself so it
// can probe internal map state without exporting accessors.
func TestCache_EvictsExpiredEntries_Internal(t *testing.T) {
	fetcher := &recordingFetcher{}
	cache := NewCache(fetcher, time.Millisecond)

	_, _ = cache.GetHero(context.Background(), "TestGod", "")

	cache.mu.RLock()
	_, present := cache.entries["public:TestGod"]
	cache.mu.RUnlock()

	if !present {
		t.Fatal("expected entry present immediately after fetch")
	}

	time.Sleep(5 * time.Millisecond)

	cache.mu.RLock()
	firstEntry := cache.entries["public:TestGod"]
	cache.mu.RUnlock()

	// Touch the same key — lookup must observe expiry, evict the old entry,
	// then refetch installs a fresh one with a later expiry.
	_, _ = cache.GetHero(context.Background(), "TestGod", "")

	cache.mu.RLock()
	secondEntry := cache.entries["public:TestGod"]
	cache.mu.RUnlock()

	if firstEntry == secondEntry {
		t.Error("expected refetch to install a NEW entry; the expired entry was not evicted")
	}

	if !secondEntry.expiresAt.After(firstEntry.expiresAt) {
		t.Error("expected fresh entry's expiry to be later than the evicted one")
	}
}
