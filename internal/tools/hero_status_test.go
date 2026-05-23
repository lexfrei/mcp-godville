package tools_test

import (
	"context"
	"strings"
	"testing"

	"github.com/cockroachdb/errors"
	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/lexfrei/mcp-godville/internal/godville"
	"github.com/lexfrei/mcp-godville/internal/tools"
)

func TestHeroStatusHandler_Success(t *testing.T) {
	prov := &stubProvider{hero: &godville.Hero{
		Name:      "Sir Testalot",
		Godname:   "TestGod",
		Level:     42,
		Alignment: "светлый",
		Motto:     "По нюху!",
		Health:    80,
		MaxHealth: 100,
		Godpower:  50,
		FightType: "boss",
		TownName:  "Сан-Франциско",
		Distance:  1234,
	}}

	handler := tools.NewHeroStatusHandler(prov)

	_, out, err := handler(context.Background(), &mcp.CallToolRequest{}, tools.HeroStatusParams{})
	if err != nil {
		t.Fatalf("handler failed: %v", err)
	}

	if out.Name != "Sir Testalot" {
		t.Errorf("expected name preserved, got %q", out.Name)
	}

	if out.Level != 42 {
		t.Errorf("expected level 42, got %d", out.Level)
	}

	if !strings.Contains(out.Output, "Sir Testalot") {
		t.Errorf("expected output to contain name, got %q", out.Output)
	}

	if !strings.Contains(out.Output, "80/100") {
		t.Errorf("expected output to contain health, got %q", out.Output)
	}
}

// Regression: experience-toward-next-level is a useful private field; the
// types.go field existed but was never surfaced by any tool.
func TestHeroStatusHandler_SurfacesExperience(t *testing.T) {
	prov := &stubProvider{hero: &godville.Hero{
		Name:       "Sir Testalot",
		Level:      42,
		Experience: 73,
	}}

	handler := tools.NewHeroStatusHandler(prov)

	_, out, err := handler(context.Background(), &mcp.CallToolRequest{}, tools.HeroStatusParams{})
	if err != nil {
		t.Fatalf("handler failed: %v", err)
	}

	if out.Experience != 73 {
		t.Errorf("expected Experience 73, got %d", out.Experience)
	}

	if !strings.Contains(out.Output, "73%") {
		t.Errorf("expected experience in output, got %q", out.Output)
	}
}

// Regression: in public mode Health is zero but MaxHealth is public. Output
// must NOT read "health: 0/100" — that's misleading. Render the cap alone
// with an explicit hint that the live value requires the userkey.
func TestHeroStatusHandler_PublicHealthDoesNotShowZero(t *testing.T) {
	prov := &stubProvider{hero: &godville.Hero{
		Name:      "Sir Testalot",
		MaxHealth: 100,
	}}

	handler := tools.NewHeroStatusHandler(prov)

	_, out, err := handler(context.Background(), &mcp.CallToolRequest{}, tools.HeroStatusParams{})
	if err != nil {
		t.Fatalf("handler failed: %v", err)
	}

	if strings.Contains(out.Output, "0/100") {
		t.Errorf("expected NOT to render '0/100' in public mode, got %q", out.Output)
	}

	if !strings.Contains(out.Output, "?/100") {
		t.Errorf("expected '?/100' placeholder in public mode, got %q", out.Output)
	}

	if !strings.Contains(out.Output, "configure a userkey") {
		t.Errorf("expected neutral 'configure a userkey' hint, got %q", out.Output)
	}
}

func TestHeroStatusHandler_ProviderError(t *testing.T) {
	handler := tools.NewHeroStatusHandler(&errProvider{err: errors.New("boom")})

	_, _, err := handler(context.Background(), &mcp.CallToolRequest{}, tools.HeroStatusParams{})
	if err == nil {
		t.Fatal("expected error to propagate")
	}

	if !errors.Is(err, tools.ErrAPI) {
		t.Errorf("expected ErrAPI, got: %v", err)
	}
}

func TestHeroStatusTool_Definition(t *testing.T) {
	tool := tools.HeroStatusTool()
	if tool.Name != "hero_status" {
		t.Errorf("expected name hero_status, got %q", tool.Name)
	}

	if tool.Description == "" {
		t.Error("expected non-empty description")
	}

	if tool.Annotations == nil || !tool.Annotations.ReadOnlyHint {
		t.Error("expected readOnly hint")
	}
}
