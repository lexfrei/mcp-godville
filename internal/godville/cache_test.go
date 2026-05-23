package godville_test

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/lexfrei/mcp-godville/internal/godville"
)

func TestCache_HitWithinTTL(t *testing.T) {
	var calls atomic.Int32

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		calls.Add(1)
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"name": "Hero", "level": 7}`))
	}))
	defer server.Close()

	client := godville.NewClient(server.URL)
	cache := godville.NewCache(client, time.Minute)

	first, err := cache.GetHero(context.Background(), "TestGod", "")
	if err != nil {
		t.Fatalf("first GetHero failed: %v", err)
	}

	second, err := cache.GetHero(context.Background(), "TestGod", "")
	if err != nil {
		t.Fatalf("second GetHero failed: %v", err)
	}

	if calls.Load() != 1 {
		t.Errorf("expected 1 upstream call, got %d", calls.Load())
	}

	if first != second {
		t.Error("expected cache to return the same pointer on hit")
	}
}

func TestCache_MissOnExpiry(t *testing.T) {
	var calls atomic.Int32

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		calls.Add(1)
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"name": "Hero"}`))
	}))
	defer server.Close()

	client := godville.NewClient(server.URL)
	cache := godville.NewCache(client, time.Millisecond)

	_, err := cache.GetHero(context.Background(), "TestGod", "")
	if err != nil {
		t.Fatalf("first GetHero failed: %v", err)
	}

	time.Sleep(5 * time.Millisecond)

	_, err = cache.GetHero(context.Background(), "TestGod", "")
	if err != nil {
		t.Fatalf("second GetHero failed: %v", err)
	}

	if calls.Load() != 2 {
		t.Errorf("expected 2 upstream calls after expiry, got %d", calls.Load())
	}
}

func TestCache_DistinctKeyPerUserkey(t *testing.T) {
	var calls atomic.Int32

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		calls.Add(1)
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{}`))
	}))
	defer server.Close()

	client := godville.NewClient(server.URL)
	cache := godville.NewCache(client, time.Minute)

	_, _ = cache.GetHero(context.Background(), "TestGod", "")
	_, _ = cache.GetHero(context.Background(), "TestGod", "secret-key")
	_, _ = cache.GetHero(context.Background(), "TestGod", "")
	_, _ = cache.GetHero(context.Background(), "TestGod", "secret-key")

	if calls.Load() != 2 {
		t.Errorf("expected 2 distinct cache entries (public + private), got %d", calls.Load())
	}
}

// Regression: two distinct non-empty userkeys for the same godname MUST not
// share a cache entry — that would be a cross-tenant data leak under HTTP
// transport with per-client elicited credentials.
func TestCache_DistinctKeyPerDistinctUserkey(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")

		if r.URL.Path == "/gods/api/TestGod/alice-key.json" {
			_, _ = w.Write([]byte(`{"motto": "alice payload"}`))

			return
		}

		_, _ = w.Write([]byte(`{"motto": "bob payload"}`))
	}))
	defer server.Close()

	client := godville.NewClient(server.URL)
	cache := godville.NewCache(client, time.Minute)

	aliceHero, err := cache.GetHero(context.Background(), "TestGod", "alice-key")
	if err != nil {
		t.Fatalf("alice fetch failed: %v", err)
	}

	bobHero, err := cache.GetHero(context.Background(), "TestGod", "bob-key")
	if err != nil {
		t.Fatalf("bob fetch failed: %v", err)
	}

	if aliceHero == bobHero {
		t.Fatal("alice and bob got the same cached hero — cross-tenant leak")
	}

	if bobHero.Motto != "bob payload" {
		t.Errorf("bob got alice's payload: motto=%q", bobHero.Motto)
	}
}

// Regression: HeroFetcher is an exported interface; a misbehaving custom
// implementation returning (nil, nil) must NOT poison the cache. The
// downstream tools dereference hero.Field and would panic if nil leaked.
type nilFetcher struct{}

func (nilFetcher) GetHero(_ context.Context, _, _ string) (*godville.Hero, error) {
	return nil, nil //nolint:nilnil // deliberately exercising the bad-fetcher contract
}

func TestCache_RefusesNilHero(t *testing.T) {
	cache := godville.NewCache(nilFetcher{}, time.Minute)

	hero, err := cache.GetHero(context.Background(), "TestGod", "")
	if err == nil {
		t.Fatal("expected error when fetcher returns (nil, nil)")
	}

	if hero != nil {
		t.Errorf("expected nil hero on rejected fetch, got %+v", hero)
	}

	// Next call must NOT be served from cache (no poisoning).
	hero, err = cache.GetHero(context.Background(), "TestGod", "")
	if err == nil || hero != nil {
		t.Errorf("expected error to repeat on next call (not cached), got hero=%v err=%v", hero, err)
	}
}

// Regression: when the singleflight WINNER cancels mid-fetch, other waiters
// must still receive the fetched result — not get poisoned with
// context.Canceled. Earlier the callback passed the winner's ctx directly to
// the fetcher, so any caller could kill the result for everyone else.
func TestCache_WinnerCancelDoesNotPoisonWaiters(t *testing.T) {
	release := make(chan struct{})

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		<-release
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"name": "TheHero"}`))
	}))
	// Defer order matters: server.Close drains active handlers, so close
	// release first via the LIFO defer ordering.
	defer server.Close()

	client := godville.NewClient(server.URL)
	cache := godville.NewCache(client, time.Minute)

	// Winner: cancel after 30ms.
	winnerCtx, winnerCancel := context.WithCancel(context.Background())

	winnerDone := make(chan error, 1)

	go func() {
		_, err := cache.GetHero(winnerCtx, "TestGod", "")
		winnerDone <- err
	}()

	// Make sure the winner has installed the singleflight slot.
	time.Sleep(30 * time.Millisecond)

	// Waiter: fresh ctx, expects a real hero.
	waiterDone := make(chan struct {
		hero *godville.Hero
		err  error
	}, 1)

	go func() {
		hero, err := cache.GetHero(context.Background(), "TestGod", "")
		waiterDone <- struct {
			hero *godville.Hero
			err  error
		}{hero, err}
	}()

	// Give the waiter time to also enter singleflight.
	time.Sleep(30 * time.Millisecond)

	// Kill the winner.
	winnerCancel()

	// Now release the server so the fetch completes.
	close(release)

	winnerErr := <-winnerDone
	if winnerErr == nil {
		t.Error("expected winner to receive a cancel error")
	}

	result := <-waiterDone

	if result.err != nil {
		t.Errorf("waiter must not be poisoned by winner cancel, got: %v", result.err)
	}

	if result.hero == nil || result.hero.Name != "TheHero" {
		t.Errorf("waiter must get the real hero, got: %+v", result.hero)
	}
}

// Regression: a caller whose context cancels while another goroutine is
// fetching must return quickly with ctx.Err(), not block until the winning
// fetch resolves. The blocking-singleflight footgun would otherwise leak
// goroutines on HTTP transport client disconnects.
func TestCache_CallerContextCancelDoesNotBlock(t *testing.T) {
	release := make(chan struct{})

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		<-release // hold the in-flight fetch until the test releases it
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{}`))
	}))
	// Defer order matters: server.Close waits for active handlers to drain,
	// so we MUST close `release` first to unblock the handler before
	// server.Close runs. Defers fire LIFO, so declare server.Close FIRST.
	defer server.Close()
	defer close(release)

	client := godville.NewClient(server.URL)
	cache := godville.NewCache(client, time.Minute)

	// First caller: keep their context alive so they own the in-flight fetch.
	winnerDone := make(chan struct{})

	go func() {
		defer close(winnerDone)
		_, _ = cache.GetHero(context.Background(), "TestGod", "")
	}()

	// Give the winner a moment to start the fetch.
	time.Sleep(20 * time.Millisecond)

	// Second caller: cancel right away. Must NOT wait for the winner.
	cancelCtx, cancel := context.WithCancel(context.Background())
	cancel()

	start := time.Now()

	_, err := cache.GetHero(cancelCtx, "TestGod", "")

	elapsed := time.Since(start)

	if err == nil {
		t.Fatal("expected context-cancelled error")
	}

	if elapsed > 100*time.Millisecond {
		t.Errorf("expected immediate return on ctx cancel, took %s", elapsed)
	}
}

// Regression: concurrent fan-out on a cold key must coalesce to a single
// upstream call. Nine tools called in parallel must not burn 9/30 of the
// 10-minute rate-limit budget.
func TestCache_ColdMissCoalesces(t *testing.T) {
	var calls atomic.Int32

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		calls.Add(1)

		// Hold the connection briefly so concurrent callers pile up.
		time.Sleep(50 * time.Millisecond)

		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{}`))
	}))
	defer server.Close()

	client := godville.NewClient(server.URL)
	cache := godville.NewCache(client, time.Minute)

	const goroutines = 9

	var wg sync.WaitGroup
	wg.Add(goroutines)

	for range goroutines {
		go func() {
			defer wg.Done()

			_, _ = cache.GetHero(context.Background(), "TestGod", "")
		}()
	}

	wg.Wait()

	if calls.Load() != 1 {
		t.Errorf("expected cold-cache fan-out to coalesce to 1 upstream call, got %d", calls.Load())
	}
}

func TestCache_DistinctKeyPerGodname(t *testing.T) {
	var calls atomic.Int32

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		calls.Add(1)
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{}`))
	}))
	defer server.Close()

	client := godville.NewClient(server.URL)
	cache := godville.NewCache(client, time.Minute)

	_, _ = cache.GetHero(context.Background(), "A", "")
	_, _ = cache.GetHero(context.Background(), "B", "")
	_, _ = cache.GetHero(context.Background(), "A", "")
	_, _ = cache.GetHero(context.Background(), "B", "")

	if calls.Load() != 2 {
		t.Errorf("expected 2 cache entries (A and B), got %d", calls.Load())
	}
}

func TestCache_DoesNotCacheErrors(t *testing.T) {
	var calls atomic.Int32

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		calls.Add(1)
		w.WriteHeader(http.StatusForbidden)
		_, _ = w.Write([]byte(`{"error": "rate limit"}`))
	}))
	defer server.Close()

	client := godville.NewClient(server.URL)
	cache := godville.NewCache(client, time.Minute)

	_, err1 := cache.GetHero(context.Background(), "TestGod", "")
	if err1 == nil {
		t.Fatal("expected first call to error")
	}

	_, err2 := cache.GetHero(context.Background(), "TestGod", "")
	if err2 == nil {
		t.Fatal("expected second call to also error")
	}

	if calls.Load() != 2 {
		t.Errorf("expected errors NOT to be cached: got %d upstream calls, want 2", calls.Load())
	}

	if !errors.Is(err2, godville.ErrAPI) {
		t.Errorf("expected ErrAPI on second call, got: %v", err2)
	}
}

func TestCache_Invalidate(t *testing.T) {
	var calls atomic.Int32

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		calls.Add(1)
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{}`))
	}))
	defer server.Close()

	client := godville.NewClient(server.URL)
	cache := godville.NewCache(client, time.Minute)

	_, _ = cache.GetHero(context.Background(), "TestGod", "")
	cache.Invalidate()
	_, _ = cache.GetHero(context.Background(), "TestGod", "")

	if calls.Load() != 2 {
		t.Errorf("expected Invalidate to clear cache, got %d calls", calls.Load())
	}
}

func TestCache_ConcurrentAccess(t *testing.T) {
	var calls atomic.Int32

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		calls.Add(1)
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{}`))
	}))
	defer server.Close()

	client := godville.NewClient(server.URL)
	cache := godville.NewCache(client, time.Minute)

	const goroutines = 20

	var wg sync.WaitGroup
	wg.Add(goroutines)

	for range goroutines {
		go func() {
			defer wg.Done()

			_, _ = cache.GetHero(context.Background(), "TestGod", "")
		}()
	}

	wg.Wait()

	if calls.Load() == 0 {
		t.Error("expected at least one upstream call")
	}
}
