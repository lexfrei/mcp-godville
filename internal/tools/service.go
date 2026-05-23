package tools

import (
	"context"

	"github.com/lexfrei/mcp-godville/internal/godville"
)

// HeroProvider resolves credentials and returns hero state. Tools depend on
// this single interface so they can be tested without wiring the auth + cache
// stack.
type HeroProvider interface {
	GetHero(ctx context.Context) (*godville.Hero, error)
}
