// Package config provides configuration loading from environment variables.
package config

import (
	"net"
	"os"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/cockroachdb/errors"
)

// dnsHostRe is a syntactic colon-rejector, NOT a full hostname validator. It
// accepts anything that looks roughly like a DNS name (letters, digits, dot,
// hyphen) and rejects accidents like "127.0.0:1" where a colon leaked from a
// port. Patterns like "..", ".", "-foo" pass the regex; they're caught (or
// not) by the operating system at bind time. The goal is to fail fast on
// obvious typos, not to be a hostname spec linter.
var dnsHostRe = regexp.MustCompile(`^[A-Za-z0-9._-]+$`)

const (
	maxPort         = 65535
	defaultAPIBase  = "https://godville.net"
	defaultHTTPHost = "127.0.0.1"
	defaultCacheTTL = 60 * time.Second
)

// ErrInvalidCacheTTL is returned when GODVILLE_CACHE_TTL is not a valid duration.
var ErrInvalidCacheTTL = errors.New("GODVILLE_CACHE_TTL must be a valid Go duration (e.g. 60s, 2m)")

// ErrInvalidHTTPPort is returned when MCP_HTTP_PORT is not a valid port number.
var ErrInvalidHTTPPort = errors.New("MCP_HTTP_PORT must be a valid port number (1-65535)")

// ErrInvalidHTTPHost is returned when MCP_HTTP_HOST is not a valid host literal.
var ErrInvalidHTTPHost = errors.New("MCP_HTTP_HOST must be a valid host (IP, hostname, or 'localhost')")

// Config holds the application configuration loaded from environment variables.
type Config struct {
	APIBase  string
	Godname  string
	Userkey  string
	CacheTTL time.Duration
	HTTPPort string
	HTTPHost string
}

// Load reads configuration from environment variables and returns a Config.
func Load() (*Config, error) {
	cacheTTL, err := loadCacheTTL()
	if err != nil {
		return nil, err
	}

	httpPort, err := loadHTTPPort()
	if err != nil {
		return nil, err
	}

	httpHost, err := loadHTTPHost()
	if err != nil {
		return nil, err
	}

	return &Config{
		APIBase:  loadAPIBase(),
		Godname:  os.Getenv("GODVILLE_GODNAME"),
		Userkey:  os.Getenv("GODVILLE_USERKEY"),
		CacheTTL: cacheTTL,
		HTTPPort: httpPort,
		HTTPHost: httpHost,
	}, nil
}

// HasGodname returns true if godname is configured.
func (cfg *Config) HasGodname() bool {
	return cfg.Godname != ""
}

// HasUserkey returns true if a userkey is configured (enables private API fields).
func (cfg *Config) HasUserkey() bool {
	return cfg.Userkey != ""
}

// HTTPEnabled returns true if HTTP transport should be enabled.
func (cfg *Config) HTTPEnabled() bool {
	return cfg.HTTPPort != ""
}

// HTTPAddr returns the host:port for the HTTP server.
func (cfg *Config) HTTPAddr() string {
	return net.JoinHostPort(cfg.HTTPHost, cfg.HTTPPort)
}

func loadAPIBase() string {
	raw := os.Getenv("GODVILLE_API_BASE")
	if raw == "" {
		raw = defaultAPIBase
	}

	return strings.TrimSuffix(raw, "/")
}

func loadCacheTTL() (time.Duration, error) {
	raw := os.Getenv("GODVILLE_CACHE_TTL")
	if raw == "" {
		return defaultCacheTTL, nil
	}

	dur, err := time.ParseDuration(raw)
	if err != nil {
		return 0, errors.Wrapf(ErrInvalidCacheTTL, "%q", raw)
	}

	if dur <= 0 {
		// A zero or negative TTL effectively disables caching; every tool
		// call would hit the upstream and quickly burn the 30 req / 10 min
		// rate-limit budget, then sticky-lock the (god+ip) pair.
		return 0, errors.Wrapf(ErrInvalidCacheTTL, "%q must be positive (got %s)", raw, dur)
	}

	return dur, nil
}

func loadHTTPPort() (string, error) {
	raw := os.Getenv("MCP_HTTP_PORT")
	if raw == "" {
		return "", nil
	}

	port, err := strconv.Atoi(raw)
	if err != nil || port < 1 || port > maxPort {
		return "", errors.Wrapf(ErrInvalidHTTPPort, "%q", raw)
	}

	return raw, nil
}

func loadHTTPHost() (string, error) {
	host := os.Getenv("MCP_HTTP_HOST")
	if host == "" {
		return defaultHTTPHost, nil
	}

	// Accept any IP literal (IPv4 or IPv6) verbatim.
	if net.ParseIP(host) != nil {
		return host, nil
	}

	// Otherwise treat as a DNS name. Reject anything containing characters
	// that wouldn't appear in a real hostname — most importantly ':' so a
	// typo like "127.0.0:1" fails at config load instead of confusingly at
	// socket-bind time.
	if !dnsHostRe.MatchString(host) {
		return "", errors.Wrapf(ErrInvalidHTTPHost, "%q", host)
	}

	return host, nil
}
