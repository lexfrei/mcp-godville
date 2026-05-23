package config_test

import (
	"testing"

	"github.com/cockroachdb/errors"

	"github.com/lexfrei/mcp-godville/internal/config"
)

func clearEnv(t *testing.T) {
	t.Helper()
	t.Setenv("GODVILLE_API_BASE", "")
	t.Setenv("GODVILLE_GODNAME", "")
	t.Setenv("GODVILLE_USERKEY", "")
	t.Setenv("GODVILLE_CACHE_TTL", "")
	t.Setenv("MCP_HTTP_PORT", "")
	t.Setenv("MCP_HTTP_HOST", "")
}

func TestLoad_Defaults(t *testing.T) {
	clearEnv(t)

	cfg, err := config.Load()
	if err != nil {
		t.Fatalf("Load() failed: %v", err)
	}

	if cfg.APIBase != "https://godville.net" {
		t.Errorf("expected default APIBase https://godville.net, got %s", cfg.APIBase)
	}

	if cfg.Godname != "" {
		t.Errorf("expected empty Godname, got %s", cfg.Godname)
	}

	if cfg.Userkey != "" {
		t.Errorf("expected empty Userkey, got %s", cfg.Userkey)
	}

	if cfg.CacheTTL.Seconds() != 60 {
		t.Errorf("expected default CacheTTL 60s, got %s", cfg.CacheTTL)
	}

	if cfg.HTTPPort != "" {
		t.Errorf("expected empty HTTPPort, got %s", cfg.HTTPPort)
	}

	if cfg.HTTPHost != "127.0.0.1" {
		t.Errorf("expected default HTTPHost 127.0.0.1, got %s", cfg.HTTPHost)
	}
}

func TestLoad_CustomValues(t *testing.T) {
	clearEnv(t)
	t.Setenv("GODVILLE_API_BASE", "https://godvillegame.com")
	t.Setenv("GODVILLE_GODNAME", "TestGod")
	t.Setenv("GODVILLE_USERKEY", "abc123")
	t.Setenv("GODVILLE_CACHE_TTL", "30s")
	t.Setenv("MCP_HTTP_PORT", "8080")
	t.Setenv("MCP_HTTP_HOST", "0.0.0.0")

	cfg, err := config.Load()
	if err != nil {
		t.Fatalf("Load() failed: %v", err)
	}

	if cfg.APIBase != "https://godvillegame.com" {
		t.Errorf("expected APIBase https://godvillegame.com, got %s", cfg.APIBase)
	}

	if cfg.Godname != "TestGod" {
		t.Errorf("expected Godname TestGod, got %s", cfg.Godname)
	}

	if cfg.Userkey != "abc123" {
		t.Errorf("expected Userkey abc123, got %s", cfg.Userkey)
	}

	if cfg.CacheTTL.Seconds() != 30 {
		t.Errorf("expected CacheTTL 30s, got %s", cfg.CacheTTL)
	}

	if cfg.HTTPPort != "8080" {
		t.Errorf("expected HTTPPort 8080, got %s", cfg.HTTPPort)
	}

	if cfg.HTTPHost != "0.0.0.0" {
		t.Errorf("expected HTTPHost 0.0.0.0, got %s", cfg.HTTPHost)
	}
}

func TestLoad_TrimsAPIBaseTrailingSlash(t *testing.T) {
	clearEnv(t)
	t.Setenv("GODVILLE_API_BASE", "https://godville.net/")

	cfg, err := config.Load()
	if err != nil {
		t.Fatalf("Load() failed: %v", err)
	}

	if cfg.APIBase != "https://godville.net" {
		t.Errorf("expected trailing slash stripped, got %q", cfg.APIBase)
	}
}

func TestLoad_InvalidCacheTTL(t *testing.T) {
	clearEnv(t)
	t.Setenv("GODVILLE_CACHE_TTL", "not-a-duration")

	_, err := config.Load()
	if err == nil {
		t.Fatal("expected error for invalid GODVILLE_CACHE_TTL")
	}

	if !errors.Is(err, config.ErrInvalidCacheTTL) {
		t.Errorf("expected ErrInvalidCacheTTL, got: %v", err)
	}
}

// Regression: 0 and negative durations would disable the cache and blow
// through the upstream rate-limit budget. Reject at load time.
func TestLoad_NonPositiveCacheTTL(t *testing.T) {
	for _, raw := range []string{"0s", "0", "-1s", "-1h"} {
		t.Run(raw, func(t *testing.T) {
			clearEnv(t)
			t.Setenv("GODVILLE_CACHE_TTL", raw)

			_, err := config.Load()
			if err == nil {
				t.Fatalf("expected error for non-positive TTL %q", raw)
			}

			if !errors.Is(err, config.ErrInvalidCacheTTL) {
				t.Errorf("expected ErrInvalidCacheTTL, got: %v", err)
			}
		})
	}
}

func TestLoad_InvalidHTTPPort(t *testing.T) {
	clearEnv(t)
	t.Setenv("MCP_HTTP_PORT", "not-a-port")

	_, err := config.Load()
	if err == nil {
		t.Fatal("expected error for invalid MCP_HTTP_PORT")
	}

	if !errors.Is(err, config.ErrInvalidHTTPPort) {
		t.Errorf("expected ErrInvalidHTTPPort, got: %v", err)
	}
}

// Regression: a typo like "127.0.0:1" should fail at config load time, not
// confusingly at bind time.
func TestLoad_InvalidHTTPHost(t *testing.T) {
	clearEnv(t)
	t.Setenv("MCP_HTTP_HOST", "127.0.0:1") // malformed — colon inside host

	_, err := config.Load()
	if err == nil {
		t.Fatal("expected error for malformed MCP_HTTP_HOST")
	}

	if !errors.Is(err, config.ErrInvalidHTTPHost) {
		t.Errorf("expected ErrInvalidHTTPHost, got: %v", err)
	}
}

func TestLoad_ValidHTTPHostForms(t *testing.T) {
	tests := []string{"127.0.0.1", "0.0.0.0", "localhost", "::1", "example.invalid"}
	for _, host := range tests {
		t.Run(host, func(t *testing.T) {
			clearEnv(t)
			t.Setenv("MCP_HTTP_HOST", host)

			cfg, err := config.Load()
			if err != nil {
				t.Errorf("expected %q to be accepted, got: %v", host, err)
			}

			if cfg.HTTPHost != host {
				t.Errorf("expected HTTPHost %q, got %q", host, cfg.HTTPHost)
			}
		})
	}
}

func TestLoad_HTTPPortOutOfRange(t *testing.T) {
	clearEnv(t)
	t.Setenv("MCP_HTTP_PORT", "99999")

	_, err := config.Load()
	if err == nil {
		t.Fatal("expected error for out-of-range MCP_HTTP_PORT")
	}
}

func TestConfig_HasUserkey(t *testing.T) {
	tests := []struct {
		name    string
		userkey string
		want    bool
	}{
		{"set", "abc123", true},
		{"empty", "", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := &config.Config{Userkey: tt.userkey}
			if got := cfg.HasUserkey(); got != tt.want {
				t.Errorf("HasUserkey() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestConfig_HasGodname(t *testing.T) {
	tests := []struct {
		name    string
		godname string
		want    bool
	}{
		{"set", "TestGod", true},
		{"empty", "", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := &config.Config{Godname: tt.godname}
			if got := cfg.HasGodname(); got != tt.want {
				t.Errorf("HasGodname() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestConfig_HTTPEnabled(t *testing.T) {
	tests := []struct {
		name string
		port string
		want bool
	}{
		{"set", "8080", true},
		{"empty", "", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := &config.Config{HTTPPort: tt.port}
			if got := cfg.HTTPEnabled(); got != tt.want {
				t.Errorf("HTTPEnabled() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestConfig_HTTPAddr(t *testing.T) {
	cfg := &config.Config{HTTPHost: "127.0.0.1", HTTPPort: "8080"}
	if got := cfg.HTTPAddr(); got != "127.0.0.1:8080" {
		t.Errorf("HTTPAddr() = %q, want 127.0.0.1:8080", got)
	}
}
