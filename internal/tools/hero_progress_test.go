package tools_test

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/lexfrei/mcp-godville/internal/godville"
	"github.com/lexfrei/mcp-godville/internal/tools"
)

func TestHeroProgressHandler_FullProgress(t *testing.T) {
	prov := &stubProvider{hero: &godville.Hero{
		Level:             42,
		GoldApprox:        "~10k",
		Savings:           "1.5 миллиона",
		BricksCnt:         800,
		WoodCnt:           50,
		ArenaWon:          30,
		ArenaLost:         5,
		TLevel:            3,
		ArkMale:           10,
		ArkFemale:         10,
		SoulsPercent:      "25",
		Words:             123,
		ShopName:          "Магазинчик",
		TempleCompletedAt: "2024-12-01",
	}}

	handler := tools.NewHeroProgressHandler(prov)

	_, out, err := handler(context.Background(), &mcp.CallToolRequest{}, tools.HeroProgressParams{})
	if err != nil {
		t.Fatalf("handler failed: %v", err)
	}

	if out.BricksCnt != 800 {
		t.Errorf("expected 800 bricks, got %d", out.BricksCnt)
	}

	if !strings.Contains(out.Output, "Arena: 30W / 5L") {
		t.Errorf("expected arena line in output, got %q", out.Output)
	}

	if !strings.Contains(out.Output, "Temple completed: 2024-12-01") {
		t.Errorf("expected temple line in output, got %q", out.Output)
	}
}

// Regression: the tool description promises "souls/relics percent", so
// non-zero RelicsPercent must appear in the formatted output.
func TestHeroProgressHandler_FormatsRelicsPercent(t *testing.T) {
	prov := &stubProvider{hero: &godville.Hero{RelicsPercent: "73"}}

	handler := tools.NewHeroProgressHandler(prov)

	_, out, err := handler(context.Background(), &mcp.CallToolRequest{}, tools.HeroProgressParams{})
	if err != nil {
		t.Fatalf("handler failed: %v", err)
	}

	if !strings.Contains(out.Output, "73%") {
		t.Errorf("expected relics percent in output, got %q", out.Output)
	}
}

// Regression: souls percentage should render even when SoulsAt (the
// completion milestone) is empty — that's the in-progress state.
func TestHeroProgressHandler_FormatsSoulsPercentWithoutMilestone(t *testing.T) {
	prov := &stubProvider{hero: &godville.Hero{SoulsPercent: "50"}}

	handler := tools.NewHeroProgressHandler(prov)

	_, out, err := handler(context.Background(), &mcp.CallToolRequest{}, tools.HeroProgressParams{})
	if err != nil {
		t.Fatalf("handler failed: %v", err)
	}

	if !strings.Contains(out.Output, "50%") {
		t.Errorf("expected souls percent in output even without milestone, got %q", out.Output)
	}
}

// Documented contract: the Output string OMITS zero counters; the
// structured JSON ALWAYS includes them. Consumers that need to distinguish
// "0 collected" from "data missing" must read the typed struct.
func TestHeroProgressHandler_ZeroCountersDocumentedContract(t *testing.T) {
	prov := &stubProvider{hero: &godville.Hero{
		BricksCnt: 0,
		WoodCnt:   0,
		Level:     5, // at least one non-zero so we don't hit the "no data" branch
	}}

	handler := tools.NewHeroProgressHandler(prov)

	_, out, err := handler(context.Background(), &mcp.CallToolRequest{}, tools.HeroProgressParams{})
	if err != nil {
		t.Fatalf("handler failed: %v", err)
	}

	// Output drops zero counters by design.
	if strings.Contains(out.Output, "Bricks: 0") || strings.Contains(out.Output, "Wood: 0") {
		t.Errorf("expected zero counters omitted from Output, got %q", out.Output)
	}

	// Per the documented contract, omitempty drops zero counters from
	// JSON too — consumers that need "0 vs missing" should call hero_raw
	// instead. Verify by marshalling and checking the keys are absent.
	if out.BricksCnt != 0 || out.WoodCnt != 0 {
		t.Errorf("expected struct fields preserved at zero, got bricks=%d wood=%d",
			out.BricksCnt, out.WoodCnt)
	}

	marshalled, err := json.Marshal(out)
	if err != nil {
		t.Fatalf("marshal failed: %v", err)
	}

	asStr := string(marshalled)
	if strings.Contains(asStr, "bricksCnt") || strings.Contains(asStr, "woodCnt") {
		t.Errorf("expected zero counters dropped from JSON via omitempty, got %s", asStr)
	}
}

// Regression: an in-progress ark has ArkName but no ArkCompletedAt. The
// formatter previously dropped ArkName silently in that state, even though
// the tool description promises "ark name" as part of the output.
func TestHeroProgressHandler_ArkNameWithoutCompletion(t *testing.T) {
	prov := &stubProvider{hero: &godville.Hero{ArkName: "Светлый ковчег"}}

	handler := tools.NewHeroProgressHandler(prov)

	_, out, err := handler(context.Background(), &mcp.CallToolRequest{}, tools.HeroProgressParams{})
	if err != nil {
		t.Fatalf("handler failed: %v", err)
	}

	if !strings.Contains(out.Output, "Светлый ковчег") {
		t.Errorf("expected ArkName in output even without completion, got %q", out.Output)
	}
}

func TestHeroProgressHandler_EmptyHero(t *testing.T) {
	prov := &stubProvider{hero: &godville.Hero{}}
	handler := tools.NewHeroProgressHandler(prov)

	_, out, err := handler(context.Background(), &mcp.CallToolRequest{}, tools.HeroProgressParams{})
	if err != nil {
		t.Fatalf("handler failed: %v", err)
	}

	if out.Output != "No progress data." {
		t.Errorf("expected empty hint, got %q", out.Output)
	}
}
