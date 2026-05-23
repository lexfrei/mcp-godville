package auth_test

import (
	"context"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/cockroachdb/errors"

	"github.com/lexfrei/mcp-godville/internal/auth"
)

type errElicitor struct{ err error }

func (e *errElicitor) Elicit(_ context.Context, _, _ string) (string, bool, error) {
	return "", false, e.err
}

type delayedElicitor struct {
	value string
	delay time.Duration
	calls atomic.Int32
}

func (d *delayedElicitor) Elicit(_ context.Context, _, _ string) (string, bool, error) {
	d.calls.Add(1)
	time.Sleep(d.delay)

	return d.value, true, nil
}

func TestAuthenticator_GodnameFromEnv(t *testing.T) {
	aut := auth.NewAuthenticator("MyGod", "secret")

	godname, err := aut.Godname(context.Background())
	if err != nil {
		t.Fatalf("Godname failed: %v", err)
	}

	if godname != "MyGod" {
		t.Errorf("expected MyGod, got %q", godname)
	}
}

func TestAuthenticator_UserkeyFromEnv(t *testing.T) {
	aut := auth.NewAuthenticator("MyGod", "secret")

	userkey, err := aut.Userkey(context.Background())
	if err != nil {
		t.Fatalf("Userkey failed: %v", err)
	}

	if userkey != "secret" {
		t.Errorf("expected secret, got %q", userkey)
	}
}

func TestAuthenticator_EmptyUserkeyIsPublicMode(t *testing.T) {
	aut := auth.NewAuthenticator("MyGod", "")

	userkey, err := aut.Userkey(context.Background())
	if err != nil {
		t.Fatalf("Userkey must not error in public mode: %v", err)
	}

	if userkey != "" {
		t.Errorf("expected empty userkey, got %q", userkey)
	}
}

func TestAuthenticator_GodnameMissingNoElicitor(t *testing.T) {
	aut := auth.NewAuthenticator("", "")

	_, err := aut.Godname(context.Background())
	if err == nil {
		t.Fatal("expected error when godname is unset and no elicitor configured")
	}

	if !errors.Is(err, auth.ErrGodnameRequired) {
		t.Errorf("expected ErrGodnameRequired, got: %v", err)
	}
}

type stubElicitor struct {
	responses    map[string]string
	calls        int
	declined     bool
	lastMessages []string
}

func (s *stubElicitor) Elicit(_ context.Context, field, message string) (string, bool, error) {
	s.calls++
	s.lastMessages = append(s.lastMessages, message)

	if s.declined {
		return "", false, nil
	}

	val, ok := s.responses[field]
	if !ok {
		return "", false, nil
	}

	return val, true, nil
}

func TestAuthenticator_GodnameViaElicitation(t *testing.T) {
	elicit := &stubElicitor{responses: map[string]string{"godname": "ElicitedGod"}}

	aut := auth.NewAuthenticator("", "")
	aut.SetElicitor(elicit)

	godname, err := aut.Godname(context.Background())
	if err != nil {
		t.Fatalf("Godname failed: %v", err)
	}

	if godname != "ElicitedGod" {
		t.Errorf("expected ElicitedGod, got %q", godname)
	}

	if elicit.calls != 1 {
		t.Errorf("expected 1 elicitation call, got %d", elicit.calls)
	}

	// Calling again should hit the cache, not re-elicit.
	_, _ = aut.Godname(context.Background())

	if elicit.calls != 1 {
		t.Errorf("expected godname to be cached after elicitation, got %d calls", elicit.calls)
	}
}

func TestAuthenticator_UserkeyViaElicitationOptional(t *testing.T) {
	// User declines the userkey prompt — that's fine, fall back to public mode.
	elicit := &stubElicitor{declined: true}

	aut := auth.NewAuthenticator("MyGod", "")
	aut.SetElicitor(elicit)

	userkey, err := aut.Userkey(context.Background())
	if err != nil {
		t.Fatalf("Userkey must not error when user declines: %v", err)
	}

	if userkey != "" {
		t.Errorf("expected empty userkey on decline, got %q", userkey)
	}

	// Decline result must be cached so we don't pester the user.
	_, _ = aut.Userkey(context.Background())

	if elicit.calls != 1 {
		t.Errorf("expected decline to be cached, got %d calls", elicit.calls)
	}
}

func TestAuthenticator_UserkeyViaElicitationAccepted(t *testing.T) {
	elicit := &stubElicitor{responses: map[string]string{"userkey": "elicited-secret"}}

	aut := auth.NewAuthenticator("MyGod", "")
	aut.SetElicitor(elicit)

	userkey, err := aut.Userkey(context.Background())
	if err != nil {
		t.Fatalf("Userkey failed: %v", err)
	}

	if userkey != "elicited-secret" {
		t.Errorf("expected elicited-secret, got %q", userkey)
	}
}

// User-facing elicitation messages must be ASCII (English) so the published
// binary reads cleanly in English-locale MCP clients. Cyrillic in code-as-UI
// is a localization gap, not data.
func TestAuthenticator_ElicitPromptsAreASCII(t *testing.T) {
	elicit := &stubElicitor{responses: map[string]string{"godname": "G", "userkey": "K"}}

	aut := auth.NewAuthenticator("", "")
	aut.SetElicitor(elicit)

	_, _ = aut.Godname(context.Background())
	_, _ = aut.Userkey(context.Background())

	if len(elicit.lastMessages) < 2 {
		t.Fatalf("expected 2 elicitation messages, got %d", len(elicit.lastMessages))
	}

	for _, msg := range elicit.lastMessages {
		for _, r := range msg {
			if r > 127 {
				t.Errorf("elicitation prompt contains non-ASCII rune %U in %q", r, msg)

				break
			}
		}
	}
}

// Single-tenant contract: once a credential is resolved, subsequent calls
// return the cached value regardless of who is asking. This is documented
// behaviour — the HTTP transport, if enabled, exposes the SAME hero, not
// per-caller heroes. Multi-tenant operation would need a per-session
// Authenticator, which this server intentionally does not provide.
func TestAuthenticator_SingleTenantContract(t *testing.T) {
	first := &stubElicitor{responses: map[string]string{"godname": "FirstGod"}}

	aut := auth.NewAuthenticator("", "")
	aut.SetElicitor(first)

	got, err := aut.Godname(context.Background())
	if err != nil {
		t.Fatalf("first Godname failed: %v", err)
	}

	if got != "FirstGod" {
		t.Errorf("expected FirstGod, got %q", got)
	}

	// Now swap the elicitor — a fresh hypothetical caller. The cached
	// godname must continue to be served. This is the documented
	// single-tenant guarantee; if it ever flips to "the new elicitor
	// drives a fresh prompt", we are silently advertising multi-tenant
	// behaviour we don't actually deliver in the rest of the stack.
	second := &stubElicitor{responses: map[string]string{"godname": "SecondGod"}}
	aut.SetElicitor(second)

	got, err = aut.Godname(context.Background())
	if err != nil {
		t.Fatalf("second Godname failed: %v", err)
	}

	if got != "FirstGod" {
		t.Errorf("expected cached FirstGod, got %q", got)
	}

	if second.calls != 0 {
		t.Errorf("expected second elicitor untouched, got %d calls", second.calls)
	}
}

// Regression: Authenticator must block tool calls that arrive before
// SetElicitor lands (the race window between server.Connect returning and
// main.go wiring the stdio session). Earlier this returned ErrGodnameRequired
// immediately, so the very first tool call could fail in env-less mode.
func TestAuthenticator_BlocksUntilElicitorWired(t *testing.T) {
	aut := auth.NewAuthenticator("", "")
	aut.ExpectElicitor() // arm the race-window guard

	elicit := &stubElicitor{responses: map[string]string{"godname": "WiredGod"}}

	got := make(chan string, 1)
	errs := make(chan error, 1)

	go func() {
		name, err := aut.Godname(context.Background())
		errs <- err
		got <- name
	}()

	// Give the goroutine a head start so it is actually blocked on
	// awaitElicitor before we wire it.
	time.Sleep(20 * time.Millisecond)

	select {
	case err := <-errs:
		t.Fatalf("Godname returned before SetElicitor wired: %v", err)
	default:
	}

	aut.SetElicitor(elicit)

	select {
	case err := <-errs:
		if err != nil {
			t.Fatalf("Godname failed after SetElicitor: %v", err)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("Godname did not resolve after SetElicitor")
	}

	if name := <-got; name != "WiredGod" {
		t.Errorf("expected WiredGod, got %q", name)
	}
}

// Regression: if SetElicitor is never called and there are no env
// credentials, Godname must bail when the caller's context cancels rather
// than block forever.
// Regression: a Userkey call that bails on ctx-cancel during the
// race-window wait must NOT permanently stickify public mode. After
// SetElicitor lands and the user provides a key on a subsequent call,
// the key must be used — otherwise a one-off startup timeout silently
// degrades the entire process to public mode for its lifetime.
func TestAuthenticator_UserkeyCtxCancelDoesNotStickifyPublicMode(t *testing.T) {
	aut := auth.NewAuthenticator("MyGod", "")
	aut.ExpectElicitor()

	// First call: tight ctx, no SetElicitor yet → ctx-cancels.
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Millisecond)
	defer cancel()

	first, err := aut.Userkey(ctx)
	if err != nil {
		t.Fatalf("first Userkey err: %v", err)
	}

	if first != "" {
		t.Errorf("first call expected to return empty (ctx cancel), got %q", first)
	}

	// Now wire the elicitor. Subsequent call must hit it, NOT serve the
	// stickified empty value.
	elicit := &stubElicitor{responses: map[string]string{"userkey": "RecoveredKey"}}
	aut.SetElicitor(elicit)

	second, err := aut.Userkey(context.Background())
	if err != nil {
		t.Fatalf("second Userkey err: %v", err)
	}

	if second != "RecoveredKey" {
		t.Errorf("expected RecoveredKey after SetElicitor, got %q (public-mode stickified on ctx-cancel)", second)
	}
}

func TestAuthenticator_AwaitElicitorRespectsCtxCancel(t *testing.T) {
	aut := auth.NewAuthenticator("", "")
	aut.ExpectElicitor() // arm so awaitElicitor actually waits

	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()

	_, err := aut.Godname(ctx)
	if err == nil {
		t.Fatal("expected error when ctx times out before SetElicitor")
	}
}

func TestAuthenticator_GodnameDeclinedReturnsError(t *testing.T) {
	elicit := &stubElicitor{declined: true}

	aut := auth.NewAuthenticator("", "")
	aut.SetElicitor(elicit)

	_, err := aut.Godname(context.Background())
	if err == nil {
		t.Fatal("expected error when user declines godname elicitation")
	}

	if !errors.Is(err, auth.ErrGodnameRequired) {
		t.Errorf("expected ErrGodnameRequired, got: %v", err)
	}
}

// Regression: a godname decline must NOT stick — godname is fatal, and the
// user must be able to recover from an accidental decline within the same
// session by retrying.
func TestAuthenticator_GodnameDeclineRePrompts(t *testing.T) {
	elicit := &stubElicitor{declined: true}

	aut := auth.NewAuthenticator("", "")
	aut.SetElicitor(elicit)

	_, _ = aut.Godname(context.Background())
	_, _ = aut.Godname(context.Background())

	if elicit.calls != 2 {
		t.Errorf("expected decline to re-prompt: got %d calls, want 2", elicit.calls)
	}
}

// Regression: a transport failure during userkey elicitation must propagate
// (vs. decline, which is silent fallback to public mode).
func TestAuthenticator_UserkeyTransportErrorPropagates(t *testing.T) {
	boom := errors.New("transport boom")
	elicit := &errElicitor{err: boom}

	aut := auth.NewAuthenticator("", "")
	aut.SetElicitor(elicit)

	_, err := aut.Userkey(context.Background())
	if !errors.Is(err, boom) {
		t.Fatalf("expected transport error to propagate, got: %v", err)
	}
}

// Regression: concurrent first-call elicitations must coalesce so the user
// sees one prompt per credential, not N copies for N parallel tool calls.
func TestAuthenticator_ConcurrentGodnameElicitsCoalesce(t *testing.T) {
	elicit := &delayedElicitor{
		value: "TheGod",
		delay: 50 * time.Millisecond,
	}

	aut := auth.NewAuthenticator("", "")
	aut.SetElicitor(elicit)

	const goroutines = 10

	var wg sync.WaitGroup
	wg.Add(goroutines)

	for range goroutines {
		go func() {
			defer wg.Done()
			_, _ = aut.Godname(context.Background())
		}()
	}

	wg.Wait()

	if elicit.calls.Load() != 1 {
		t.Errorf("expected concurrent elicit to coalesce to 1 prompt, got %d", elicit.calls.Load())
	}
}
