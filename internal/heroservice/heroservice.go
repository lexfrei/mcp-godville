// Package heroservice glues auth credential resolution and the Godville cache
// into the HeroProvider contract expected by tools.
package heroservice

import (
	"context"

	"github.com/cockroachdb/errors"

	"github.com/lexfrei/mcp-godville/internal/godville"
)

// CredentialsResolver resolves Godville credentials (godname + optional
// userkey). Godname errors are fatal; Userkey errors are only ever
// transport failures (decline returns "", nil per the auth contract).
type CredentialsResolver interface {
	Godname(ctx context.Context) (string, error)
	Userkey(ctx context.Context) (string, error)
}

// HeroSource fetches hero data from a backing source (typically a cache in
// front of the HTTP client).
type HeroSource interface {
	GetHero(ctx context.Context, godname, userkey string) (*godville.Hero, error)
}

// Service implements tools.HeroProvider by combining credential resolution
// with a cached HTTP backend.
type Service struct {
	creds CredentialsResolver
	src   HeroSource
}

// New returns a Service wired to creds + src.
func New(creds CredentialsResolver, src HeroSource) *Service {
	return &Service{creds: creds, src: src}
}

// GetHero resolves credentials and fetches the hero, returning any
// transport failure rather than silently degrading to public mode.
func (svc *Service) GetHero(ctx context.Context) (*godville.Hero, error) {
	godname, err := svc.creds.Godname(ctx)
	if err != nil {
		return nil, errors.Wrap(err, "resolve godname")
	}

	userkey, err := svc.creds.Userkey(ctx)
	if err != nil {
		return nil, errors.Wrap(err, "resolve userkey")
	}

	// Source already wraps its own errors (cache via singleflight, client
	// via scrubURLError). Re-wrapping with the same label here would produce
	// "fetch hero: fetch hero: …" chains; just pass it through.
	hero, err := svc.src.GetHero(ctx, godname, userkey)
	if err != nil {
		return nil, err //nolint:wrapcheck // already wrapped by the source layer.
	}

	return hero, nil
}
