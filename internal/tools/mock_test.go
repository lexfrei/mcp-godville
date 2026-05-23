package tools_test

import (
	"context"

	"github.com/cockroachdb/errors"

	"github.com/lexfrei/mcp-godville/internal/godville"
)

// stubProvider is a minimal HeroProvider for tests.
type stubProvider struct {
	hero *godville.Hero
	err  error
}

func (s *stubProvider) GetHero(_ context.Context) (*godville.Hero, error) {
	if s.err != nil {
		return nil, s.err
	}

	return s.hero, nil
}

// errProvider always returns the configured error.
type errProvider struct{ err error }

func (e *errProvider) GetHero(_ context.Context) (*godville.Hero, error) {
	if e.err == nil {
		return nil, errors.New("stub error")
	}

	return nil, e.err
}
