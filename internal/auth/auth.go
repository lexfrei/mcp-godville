// Package auth resolves Godville credentials (godname + optional userkey)
// via a cascade: configured env values → MCP elicitation → error/public-mode.
//
// Single-tenant by design: the Authenticator caches resolved credentials
// process-wide and serves the same hero to every caller for the life of the
// process. HTTP transport is therefore not a multi-tenant entry point — it
// is for additional clients reading the SAME hero, not for arbitrary callers
// supplying their own credentials. Multi-tenant operation would require a
// per-session Authenticator and per-session cache keying; that is out of
// scope for this server.
package auth

import (
	"context"
	"strings"
	"sync"

	"github.com/cockroachdb/errors"
	"golang.org/x/sync/singleflight"
)

// ErrGodnameRequired is returned when no godname can be obtained.
var ErrGodnameRequired = errors.New("godname is required: set GODVILLE_GODNAME or accept the elicitation prompt")

// Elicitor is the interface the MCP session must satisfy to request a value
// from the user. Implementations return (value, accepted, error) — accepted
// is false if the user declined the prompt.
type Elicitor interface {
	Elicit(ctx context.Context, field, message string) (string, bool, error)
}

// Authenticator resolves Godville credentials lazily and caches the result.
// Concurrent first-call elicitations are coalesced via singleflight so the
// user only sees one prompt per credential.
type Authenticator struct {
	envGodname string
	envUserkey string

	mu sync.Mutex

	elicitor      Elicitor
	elicitorReady chan struct{}

	// Godname resolution state. godnameDone marks the credential as
	// permanently resolved when a value lands; decline does NOT set it, so
	// the user can retry within the same session.
	resolvedGodname string
	godnameDone     bool

	// Userkey resolution state. Decline IS sticky — userkey is optional and
	// re-prompting would be annoying.
	resolvedUserkey string
	userkeyDone     bool

	group singleflight.Group
}

// NewAuthenticator creates an Authenticator with env-sourced credentials.
// Either may be empty; missing values are resolved on first access via
// elicitation, if an Elicitor is configured.
//
// By default no elicitor is expected — Godname returns ErrGodnameRequired
// immediately when env is empty, Userkey falls through to public mode. Call
// ExpectElicitor() BEFORE server.Connect to arm the race-window guard, then
// SetElicitor() after Connect to land the actual elicitor.
func NewAuthenticator(envGodname, envUserkey string) *Authenticator {
	ready := make(chan struct{})
	close(ready) // default: no elicitor expected, do not block awaiters

	return &Authenticator{
		envGodname:    strings.TrimSpace(envGodname),
		envUserkey:    strings.TrimSpace(envUserkey),
		elicitorReady: ready,
	}
}

// ExpectElicitor arms the race-window guard: subsequent Godname/Userkey
// calls block in awaitElicitor until SetElicitor lands. MUST be called
// BEFORE server.Connect starts accepting tool calls — otherwise an early
// tool call can race the arming and return ErrGodnameRequired without
// ever waiting. Idempotent.
func (aut *Authenticator) ExpectElicitor() {
	aut.mu.Lock()
	defer aut.mu.Unlock()

	select {
	case <-aut.elicitorReady:
		// Either default (closed) or already re-armed; if the elicitor was
		// never set, replace the closed channel with a fresh open one.
		if aut.elicitor == nil {
			aut.elicitorReady = make(chan struct{})
		}
	default:
		// Channel already open and waiting.
	}
}

// SetElicitor wires the MCP session's elicitation capability. Safe to call
// AFTER server.Connect has already started accepting tool calls — any
// in-flight Godname/Userkey waiters block on elicitorReady (armed via
// ExpectElicitor) until this lands.
func (aut *Authenticator) SetElicitor(elicitor Elicitor) {
	aut.mu.Lock()
	aut.elicitor = elicitor

	select {
	case <-aut.elicitorReady:
		// already signalled
	default:
		close(aut.elicitorReady)
	}

	aut.mu.Unlock()
}

// Godname returns the configured godname, falling back to elicitation if not
// set in the environment. Missing godname is fatal (the API path requires it).
// Decline does not stick — the user gets a fresh prompt on the next call.
func (aut *Authenticator) Godname(ctx context.Context) (string, error) {
	if val, ok := aut.cachedGodname(); ok {
		return val, nil
	}

	elicitor := aut.awaitElicitor(ctx)
	if elicitor == nil {
		return "", ErrGodnameRequired
	}

	val, err, _ := aut.group.Do("godname", func() (any, error) {
		return aut.elicitGodname(ctx, elicitor)
	})
	if err != nil {
		return "", err //nolint:wrapcheck // already wrapped inside the singleflight callback.
	}

	str, ok := val.(string)
	if !ok {
		return "", errors.New("unexpected godname value type")
	}

	return str, nil
}

// Userkey returns the configured userkey, falling back to elicitation if not
// set. A declined or empty result is NOT an error — it just disables private
// API fields. Decline is sticky to avoid re-prompting for an optional value.
// A transport failure during elicitation IS surfaced — callers must
// distinguish "user said no" from "we never reached the user".
func (aut *Authenticator) Userkey(ctx context.Context) (string, error) {
	if val, ok := aut.cachedUserkey(); ok {
		return val, nil
	}

	elicitor, ctxCancelled := aut.awaitElicitorWithReason(ctx)
	if ctxCancelled {
		// Caller bailed out mid-wait. Don't stickify public mode — a
		// subsequent call (with a fresh ctx, possibly after SetElicitor
		// lands) gets to retry. Return public-mode result for THIS call.
		return "", nil
	}

	if elicitor == nil {
		aut.markUserkeyResolved("")

		return "", nil
	}

	val, err, _ := aut.group.Do("userkey", func() (any, error) {
		return aut.elicitUserkey(ctx, elicitor)
	})
	if err != nil {
		return "", err //nolint:wrapcheck // already wrapped inside the singleflight callback.
	}

	str, ok := val.(string)
	if !ok {
		return "", errors.New("unexpected userkey value type")
	}

	return str, nil
}

// cachedGodname returns (value, true) if godname is already resolved or if
// the env source can populate it. Otherwise (_, false) to signal elicitation.
func (aut *Authenticator) cachedGodname() (string, bool) {
	aut.mu.Lock()
	defer aut.mu.Unlock()

	if aut.godnameDone {
		return aut.resolvedGodname, true
	}

	if aut.envGodname != "" {
		aut.resolvedGodname = aut.envGodname
		aut.godnameDone = true

		return aut.envGodname, true
	}

	return "", false
}

func (aut *Authenticator) cachedUserkey() (string, bool) {
	aut.mu.Lock()
	defer aut.mu.Unlock()

	if aut.userkeyDone {
		return aut.resolvedUserkey, true
	}

	if aut.envUserkey != "" {
		aut.resolvedUserkey = aut.envUserkey
		aut.userkeyDone = true

		return aut.envUserkey, true
	}

	return "", false
}

// awaitElicitor blocks until SetElicitor has installed an elicitor or the
// caller's context is done. Returns nil on ctx cancellation. Closes the
// race window between server.Connect returning (tool calls now accepted)
// and main.go calling SetElicitor.
func (aut *Authenticator) awaitElicitor(ctx context.Context) Elicitor {
	elic, _ := aut.awaitElicitorWithReason(ctx)

	return elic
}

// awaitElicitorWithReason is like awaitElicitor but also reports whether
// the nil return was due to caller ctx cancellation. Userkey needs the
// distinction because ctx-cancel must NOT stickify public mode — a future
// call may still resolve via the (eventual) elicitor.
func (aut *Authenticator) awaitElicitorWithReason(ctx context.Context) (Elicitor, bool) {
	// Read the ready channel under the mutex: ExpectElicitor may reassign
	// elicitorReady, and reading it unlocked would race with that store.
	aut.mu.Lock()
	ready := aut.elicitorReady
	aut.mu.Unlock()

	select {
	case <-ready:
		aut.mu.Lock()
		defer aut.mu.Unlock()

		return aut.elicitor, false
	case <-ctx.Done():
		return nil, true
	}
}

func (aut *Authenticator) elicitGodname(ctx context.Context, elicitor Elicitor) (string, error) {
	if cached, ok := aut.cachedGodname(); ok {
		return cached, nil
	}

	got, accepted, err := elicitor.Elicit(ctx, "godname",
		"Enter the Godville god name (GODVILLE_GODNAME)")
	if err != nil {
		return "", errors.Wrap(err, "godname elicitation")
	}

	got = strings.TrimSpace(got)

	if !accepted || got == "" {
		return "", ErrGodnameRequired
	}

	aut.mu.Lock()
	aut.godnameDone = true
	aut.resolvedGodname = got
	aut.mu.Unlock()

	return got, nil
}

func (aut *Authenticator) elicitUserkey(ctx context.Context, elicitor Elicitor) (string, error) {
	if cached, ok := aut.cachedUserkey(); ok {
		return cached, nil
	}

	got, accepted, err := elicitor.Elicit(ctx, "userkey",
		"Enter the Godville userkey for private API access (optional, GODVILLE_USERKEY)")
	if err != nil {
		return "", errors.Wrap(err, "userkey elicitation")
	}

	got = strings.TrimSpace(got)

	if !accepted {
		aut.markUserkeyResolved("")

		return "", nil
	}

	aut.markUserkeyResolved(got)

	return got, nil
}

func (aut *Authenticator) markUserkeyResolved(val string) {
	aut.mu.Lock()
	defer aut.mu.Unlock()

	aut.userkeyDone = true
	aut.resolvedUserkey = val
}
