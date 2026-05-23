// Package main is the entry point for the mcp-godville MCP server.
package main

import (
	"context"
	"log"
	"log/slog"
	"net"
	"net/http"
	"os"
	"os/signal"
	"runtime"
	"syscall"
	"time"

	"github.com/cockroachdb/errors"
	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/lexfrei/mcp-godville/internal/auth"
	"github.com/lexfrei/mcp-godville/internal/config"
	"github.com/lexfrei/mcp-godville/internal/godville"
	"github.com/lexfrei/mcp-godville/internal/heroservice"
	"github.com/lexfrei/mcp-godville/internal/tools"
)

const (
	serverName        = "mcp-godville"
	readHeaderTimeout = 10 * time.Second
	shutdownTimeout   = 5 * time.Second
	keepAliveInterval = 30 * time.Second
	httpIdleTimeout   = 60 * time.Second
	// elicitationTimeout bounds the wait on a stdio peer that may not be
	// there (e.g. HTTP-only deployment that never received a stdio peer).
	// Without this the HTTP handler would hang for the lifetime of the
	// process on the first request that triggers elicitation.
	elicitationTimeout = 30 * time.Second
)

// version and revision are populated at build time via ldflags. See
// Containerfile (-X main.version=... -X main.revision=...) and the release
// workflow's Docker meta step. Defaults below are what runs from `go run`.
var (
	version  = "dev"
	revision = "unknown"
)

func main() {
	err := run()
	if err != nil {
		log.Printf("server error: %v", err)
		os.Exit(1)
	}
}

func run() error {
	cfg, err := config.Load()
	if err != nil {
		return errors.Wrap(err, "invalid configuration")
	}

	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{
		Level: slog.LevelInfo,
	}))

	godville.SetUserAgent("mcp-godville/" + version + "+" + revision)

	client := godville.NewClient(cfg.APIBase)
	cache := godville.NewCache(client, cfg.CacheTTL)
	authenticator := auth.NewAuthenticator(cfg.Godname, cfg.Userkey)
	// Arm the race-window guard BEFORE server.Connect: any tool call that
	// lands between Connect returning and SetElicitor blocks in
	// awaitElicitor instead of failing with ErrGodnameRequired.
	authenticator.ExpectElicitor()

	provider := heroservice.New(authenticator, cache)

	server := newServer(logger)
	tools.Register(server, provider, version, revision, runtime.Version())

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	setupSignalHandler(ctx, cancel)

	stdioSession, connectErr := server.Connect(ctx, &mcp.StdioTransport{}, nil)
	if connectErr != nil {
		return errors.Wrap(connectErr, "connecting stdio transport")
	}

	authenticator.SetElicitor(&sessionElicitor{session: stdioSession})

	// HTTP transport is single-tenant by design — it shares the one set of
	// credentials elicited via stdio (or set via env). Warn loudly when HTTP
	// is enabled without env credentials so the operator knows the only path
	// to resolve them is the stdio peer.
	if cfg.HTTPEnabled() && !cfg.HasGodname() {
		logger.Warn(
			"HTTP transport enabled without GODVILLE_GODNAME — " +
				"set credentials via env, since HTTP callers cannot elicit themselves",
		)
	}

	// The HTTP transport has no authentication; bind scope is the only
	// access control. Bind to a wildcard or public-looking address →
	// anyone on the network can read the configured hero's private state.
	if cfg.HTTPEnabled() && isWildcardOrPublicHost(cfg.HTTPHost) {
		logger.Warn(
			"HTTP transport bound to a non-loopback host — there is no built-in "+
				"authentication; restrict access via firewall or front it with a reverse "+
				"proxy that enforces auth",
			"host", cfg.HTTPHost,
		)
	}

	if !cfg.HasUserkey() {
		logger.Info("running in public mode (no GODVILLE_USERKEY) — private fields will be empty")
	}

	return waitForTransports(ctx, cancel, server, stdioSession, cfg)
}

func newServer(logger *slog.Logger) *mcp.Server {
	return mcp.NewServer(
		&mcp.Implementation{
			Name:    serverName,
			Version: version + "+" + revision,
		},
		&mcp.ServerOptions{
			Instructions: "MCP server for the Godville public/private API. " +
				"Tools surface hero status, diary, inventory, pet, quest, progress, " +
				"clan and raw payload. Set GODVILLE_GODNAME (and optionally GODVILLE_USERKEY) " +
				"via env or via the elicitation prompt. Data is cached for 60s by default.",
			Logger:    logger,
			KeepAlive: keepAliveInterval,
		},
	)
}

func waitForTransports(
	ctx context.Context,
	cancel context.CancelFunc,
	server *mcp.Server,
	stdioSession *mcp.ServerSession,
	cfg *config.Config,
) error {
	if !cfg.HTTPEnabled() {
		// Close stdio on ctx cancellation: stdioSession.Wait reads stdin
		// and does not observe ctx, so a SIGINT/SIGTERM would otherwise
		// hang the process until the MCP client happened to close stdin.
		go func() {
			<-ctx.Done()

			_ = stdioSession.Close()
		}()

		waitErr := stdioSession.Wait()
		if waitErr != nil && ctx.Err() == nil {
			return errors.Wrap(waitErr, "stdio session ended")
		}

		return nil
	}

	errCh := make(chan error, 2)

	go func() {
		errCh <- runHTTPServer(ctx, server, cfg.HTTPAddr())
	}()

	go func() {
		waitErr := stdioSession.Wait()
		if waitErr != nil && ctx.Err() == nil {
			errCh <- errors.Wrap(waitErr, "stdio session ended")

			return
		}

		errCh <- nil
	}()

	first := <-errCh

	cancel()

	// stdioSession.Wait reads stdin and does not observe ctx. If the HTTP
	// goroutine returned first (e.g. bind failure), Wait will hang until
	// stdin closes — which, for a headless server, may be never. Close
	// the session explicitly so the second errCh receive completes.
	_ = stdioSession.Close()

	<-errCh

	return first
}

func runHTTPServer(ctx context.Context, server *mcp.Server, addr string) error {
	handler := mcp.NewStreamableHTTPHandler(
		func(_ *http.Request) *mcp.Server { return server },
		nil,
	)

	// Addr deliberately omitted: we use Serve(listener), not ListenAndServe,
	// so http.Server.Addr is unused. Keeping it would imply Listen happens
	// inside Server, which would mask where the bind actually occurs.
	httpServer := &http.Server{
		Handler:           http.NewCrossOriginProtection().Handler(handler),
		ReadHeaderTimeout: readHeaderTimeout,
		IdleTimeout:       httpIdleTimeout,
	}

	// Bind FIRST so we can fail fast without leaving a shutdown goroutine
	// waiting on ctx.Done() for the rest of the process lifetime.
	listener, listenErr := (&net.ListenConfig{}).Listen(ctx, "tcp", addr)
	if listenErr != nil {
		return errors.Wrapf(listenErr, "HTTP port %s unavailable", addr)
	}

	//nolint:gosec // G118: ctx is cancelled when this goroutine runs; needs a fresh context for graceful shutdown.
	go func() {
		<-ctx.Done()

		shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), shutdownTimeout)
		defer shutdownCancel()

		shutdownErr := httpServer.Shutdown(shutdownCtx) //nolint:contextcheck // fresh context for graceful shutdown.
		if shutdownErr != nil {
			log.Printf("HTTP server shutdown error: %v", shutdownErr)
		}
	}()

	log.Printf("HTTP server listening on %s", addr)

	serveErr := httpServer.Serve(listener)
	if errors.Is(serveErr, http.ErrServerClosed) {
		return nil
	}

	return errors.Wrap(serveErr, "HTTP serve failed")
}

const (
	wildcardIPv4 = "0.0.0.0"
	wildcardIPv6 = "::"
	loopbackDNS  = "localhost"
)

// isWildcardOrPublicHost reports whether the configured HTTP bind host is
// reachable from outside loopback. Used to decide whether to warn about
// the unauthenticated HTTP transport surface.
func isWildcardOrPublicHost(host string) bool {
	if host == "" || host == wildcardIPv4 || host == wildcardIPv6 {
		return true
	}

	parsed := net.ParseIP(host)
	if parsed == nil {
		// DNS hostnames could resolve to anything — assume the operator
		// knew what they were doing if they wrote a name, but treat
		// anything other than "localhost" as worth a warning.
		return host != loopbackDNS
	}

	return !parsed.IsLoopback()
}

// setupSignalHandler installs a one-shot handler for SIGINT/SIGTERM that
// cancels ctx for graceful shutdown. A second Ctrl-C falls through to Go's
// default SIGINT behaviour (immediate exit) — that's intentional: if the
// graceful path is hung, the operator should be able to abort.
func setupSignalHandler(ctx context.Context, cancel context.CancelFunc) {
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	go func() {
		select {
		case <-sigChan:
			cancel()
		case <-ctx.Done():
		}

		signal.Stop(sigChan)
	}()
}

// sessionElicitor adapts mcp.ServerSession to the auth.Elicitor interface.
type sessionElicitor struct {
	session *mcp.ServerSession
}

//nolint:gocritic // (string, bool, error) is the Elicitor interface contract; naming results adds no value here.
func (elicitor *sessionElicitor) Elicit(ctx context.Context, field, message string) (string, bool, error) {
	// Bound the underlying MCP elicit call: an HTTP-only deployment where
	// the stdio peer never connected would otherwise hang indefinitely on
	// the first private-field tool call.
	ctx, cancel := context.WithTimeout(ctx, elicitationTimeout)
	defer cancel()

	if elicitor.session == nil {
		return "", false, errors.New("no MCP session available for elicitation")
	}

	//nolint:goconst // "type" is a JSON Schema keyword; a constant adds no clarity over the literal.
	result, elicitErr := elicitor.session.Elicit(ctx, &mcp.ElicitParams{
		Message: message,
		RequestedSchema: map[string]any{
			"type": "object",
			"properties": map[string]any{
				field: map[string]any{
					"type":        "string",
					"description": message,
				},
			},
			"required": []string{field},
		},
	})
	if elicitErr != nil {
		return "", false, errors.Wrap(elicitErr, "elicitation failed")
	}

	if result == nil || result.Action != "accept" {
		return "", false, nil
	}

	val, ok := result.Content[field]
	if !ok {
		return "", false, errors.New("field not found in elicitation response")
	}

	str, ok := val.(string)
	if !ok {
		return "", false, errors.New("unexpected type in elicitation response")
	}

	return str, true, nil
}
