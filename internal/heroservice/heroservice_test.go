package heroservice_test

import (
	"context"
	"testing"

	"github.com/cockroachdb/errors"

	"github.com/lexfrei/mcp-godville/internal/godville"
	"github.com/lexfrei/mcp-godville/internal/heroservice"
)

type stubCreds struct {
	godname    string
	godnameErr error
	userkey    string
	userkeyErr error
}

func (s *stubCreds) Godname(_ context.Context) (string, error) { return s.godname, s.godnameErr }
func (s *stubCreds) Userkey(_ context.Context) (string, error) { return s.userkey, s.userkeyErr }

type stubSrc struct {
	hero *godville.Hero
	err  error
	// recorded args
	godname string
	userkey string
}

func (s *stubSrc) GetHero(_ context.Context, godname, userkey string) (*godville.Hero, error) {
	s.godname = godname
	s.userkey = userkey

	if s.err != nil {
		return nil, s.err
	}

	return s.hero, nil
}

func TestService_GetHero_Success(t *testing.T) {
	creds := &stubCreds{godname: "TestGod", userkey: "secret"}
	src := &stubSrc{hero: &godville.Hero{Name: "X"}}

	svc := heroservice.New(creds, src)

	hero, err := svc.GetHero(context.Background())
	if err != nil {
		t.Fatalf("unexpected: %v", err)
	}

	if hero.Name != "X" {
		t.Errorf("unexpected hero: %+v", hero)
	}

	if src.godname != "TestGod" || src.userkey != "secret" {
		t.Errorf("expected credentials forwarded, got (%q, %q)", src.godname, src.userkey)
	}
}

func TestService_GetHero_GodnameErrorPropagates(t *testing.T) {
	boom := errors.New("godname boom")
	svc := heroservice.New(&stubCreds{godnameErr: boom}, &stubSrc{})

	_, err := svc.GetHero(context.Background())
	if !errors.Is(err, boom) {
		t.Fatalf("expected godname error to propagate, got: %v", err)
	}
}

// Regression: a userkey transport failure (not a decline) MUST propagate.
// Silently degrading to public mode hides real elicitation breakage from
// the caller, who expected either private data or an explicit failure.
func TestService_GetHero_UserkeyErrorPropagates(t *testing.T) {
	boom := errors.New("userkey transport boom")
	creds := &stubCreds{godname: "TestGod", userkeyErr: boom}
	svc := heroservice.New(creds, &stubSrc{})

	_, err := svc.GetHero(context.Background())
	if !errors.Is(err, boom) {
		t.Fatalf("expected userkey error to propagate, got: %v", err)
	}
}

func TestService_GetHero_DeclinedUserkeyIsPublicMode(t *testing.T) {
	// Empty userkey + nil error == decline (auth contract). Service must
	// continue and call the source with an empty userkey.
	creds := &stubCreds{godname: "TestGod", userkey: ""}
	src := &stubSrc{hero: &godville.Hero{}}

	svc := heroservice.New(creds, src)

	_, err := svc.GetHero(context.Background())
	if err != nil {
		t.Fatalf("expected nil error in public mode, got: %v", err)
	}

	if src.userkey != "" {
		t.Errorf("expected empty userkey forwarded to source, got %q", src.userkey)
	}
}

func TestService_GetHero_SourceErrorPropagates(t *testing.T) {
	boom := errors.New("upstream boom")
	svc := heroservice.New(&stubCreds{godname: "TestGod"}, &stubSrc{err: boom})

	_, err := svc.GetHero(context.Background())
	if !errors.Is(err, boom) {
		t.Fatalf("expected source error to propagate, got: %v", err)
	}
}
