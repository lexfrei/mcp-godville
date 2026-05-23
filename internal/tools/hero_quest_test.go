package tools_test

import (
	"context"
	"strings"
	"testing"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/lexfrei/mcp-godville/internal/godville"
	"github.com/lexfrei/mcp-godville/internal/tools"
)

func TestHeroQuestHandler_Populated(t *testing.T) {
	prov := &stubProvider{hero: &godville.Hero{
		Quest:         "Найти что-нибудь",
		QuestProgress: 33,
		SideJob:       "Подмести площадь",
		Distance:      1234,
		TownName:      "Сан-Франциско",
		FightType:     "boss",
		BossName:      "Жуткий кашалот",
		BossPower:     20,
		ArenaFight:    true,
	}}

	handler := tools.NewHeroQuestHandler(prov)

	_, out, err := handler(context.Background(), &mcp.CallToolRequest{}, tools.HeroQuestParams{})
	if err != nil {
		t.Fatalf("handler failed: %v", err)
	}

	if out.QuestProgress != 33 {
		t.Errorf("expected progress 33, got %d", out.QuestProgress)
	}

	if !strings.Contains(out.Output, "Найти что-нибудь") {
		t.Errorf("expected quest in output, got %q", out.Output)
	}

	if !strings.Contains(out.Output, "Жуткий кашалот") {
		t.Errorf("expected boss in output, got %q", out.Output)
	}

	// Regression: boss_power is a raw strength counter, NOT a percentage.
	// The output must not append "%" — that would mislead LLM consumers.
	if strings.Contains(out.Output, "20%") {
		t.Errorf("expected boss power as plain integer, got %q (rendered as percent)", out.Output)
	}

	if !strings.Contains(out.Output, "power 20") {
		t.Errorf("expected 'power 20' in output, got %q", out.Output)
	}
}

// Regression: arena_fight is documented as a private field (README's
// Public/Private split). The hasPublic gate must not include it, otherwise
// the README and the formatter contradict each other.
func TestHeroQuestHandler_ArenaFightIsPrivate(t *testing.T) {
	// Hero with arena_fight set but nothing else → must still print the
	// "private fields hidden" hint, because arena_fight on its own is not
	// public output. (In practice arena_fight will never be set without a
	// userkey because the API only emits it on private responses, but the
	// formatter's classification needs to match the documented contract.)
	prov := &stubProvider{hero: &godville.Hero{ArenaFight: true}}

	handler := tools.NewHeroQuestHandler(prov)

	_, out, err := handler(context.Background(), &mcp.CallToolRequest{}, tools.HeroQuestParams{})
	if err != nil {
		t.Fatalf("handler failed: %v", err)
	}

	if !strings.Contains(out.Output, "Arena fight: yes") {
		t.Errorf("expected ArenaFight line, got %q", out.Output)
	}

	// hasPrivate is set (ArenaFight counts as private), so the
	// 'unavailable' branch must not fire.
	if strings.Contains(out.Output, "Quest data unavailable") {
		t.Errorf("expected NOT to print 'unavailable' when ArenaFight is set, got %q", out.Output)
	}
}

// Regression: when BossName is set but BossPower is empty, the output must
// not contain dangling empty parentheses like "Boss: Foo ()".
func TestHeroQuestHandler_BossWithoutPower(t *testing.T) {
	prov := &stubProvider{hero: &godville.Hero{
		Quest:     "Q",
		FightType: "boss",
		BossName:  "Жуткий кашалот",
	}}

	handler := tools.NewHeroQuestHandler(prov)

	_, out, err := handler(context.Background(), &mcp.CallToolRequest{}, tools.HeroQuestParams{})
	if err != nil {
		t.Fatalf("handler failed: %v", err)
	}

	if strings.Contains(out.Output, "()") {
		t.Errorf("expected no dangling empty parens when BossPower is empty, got %q", out.Output)
	}
}

// Regression: `boss_name` is a PUBLIC field per the Godville API — a public
// hero in a boss fight has it populated. Earlier the formatter early-exited
// with the "set GODVILLE_USERKEY" hint when only public fields were set,
// hiding the boss line entirely.
func TestHeroQuestHandler_SurfacesPublicBossWithoutUserkey(t *testing.T) {
	prov := &stubProvider{hero: &godville.Hero{
		BossName:  "Жуткий кашалот",
		BossPower: 35,
	}}

	handler := tools.NewHeroQuestHandler(prov)

	_, out, err := handler(context.Background(), &mcp.CallToolRequest{}, tools.HeroQuestParams{})
	if err != nil {
		t.Fatalf("handler failed: %v", err)
	}

	if !strings.Contains(out.Output, "Жуткий кашалот") {
		t.Errorf("expected public boss visible without userkey, got %q", out.Output)
	}

	if strings.Contains(out.Output, "Quest data unavailable") {
		t.Errorf("expected NOT to print 'unavailable' when public boss is present, got %q", out.Output)
	}
}

// Regression: a hero who is travelling (Distance > 0) between quests, with
// every other private field empty, must not hit the "unavailable" branch.
// Distance is a private field; its presence means userkey IS configured.
func TestHeroQuestHandler_SurfacesDistanceOnlyState(t *testing.T) {
	prov := &stubProvider{hero: &godville.Hero{Distance: 4321}}

	handler := tools.NewHeroQuestHandler(prov)

	_, out, err := handler(context.Background(), &mcp.CallToolRequest{}, tools.HeroQuestParams{})
	if err != nil {
		t.Fatalf("handler failed: %v", err)
	}

	if strings.Contains(out.Output, "Quest data unavailable") {
		t.Errorf("expected not to print 'unavailable' when Distance is set, got %q", out.Output)
	}

	if !strings.Contains(out.Output, "Distance: 4321") {
		t.Errorf("expected Distance line in output, got %q", out.Output)
	}
}

// Regression: the tool description must not categorically claim "requires a
// userkey" because boss_name/boss_power are public. LLM routers read the
// description verbatim and a flat "requires userkey" claim would steer them
// away from a tool that is in fact useful in public mode.
func TestHeroQuestTool_DescriptionAcknowledgesPublicFields(t *testing.T) {
	desc := tools.HeroQuestTool().Description
	lower := strings.ToLower(desc)

	if !strings.Contains(lower, "public") {
		t.Errorf("expected description to mention public fields, got %q", desc)
	}

	if !strings.Contains(lower, "require a userkey") && !strings.Contains(lower, "requires a userkey") {
		t.Errorf("expected description to mention userkey requirement for private fields, got %q", desc)
	}

	// arena_fight must be classified as private (matches README).
	if !strings.Contains(lower, "arena-fight") && !strings.Contains(lower, "arena fight") {
		t.Errorf("expected description to mention arena-fight (private), got %q", desc)
	}
}

func TestHeroQuestHandler_PublicHint(t *testing.T) {
	prov := &stubProvider{hero: &godville.Hero{}}

	handler := tools.NewHeroQuestHandler(prov)

	_, out, err := handler(context.Background(), &mcp.CallToolRequest{}, tools.HeroQuestParams{})
	if err != nil {
		t.Fatalf("handler failed: %v", err)
	}

	if !strings.Contains(out.Output, "GODVILLE_USERKEY") {
		t.Errorf("expected hint about userkey, got %q", out.Output)
	}
}
