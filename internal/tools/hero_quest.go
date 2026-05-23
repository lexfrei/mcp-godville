package tools

import (
	"context"
	"fmt"
	"strings"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// HeroQuestParams has no parameters.
type HeroQuestParams struct{}

// HeroQuestResult contains quest, side job, and battle context. All fields
// are private (require GODVILLE_USERKEY).
type HeroQuestResult struct {
	Quest           string `json:"quest,omitempty"`
	QuestProgress   int    `json:"questProgress,omitempty"`
	SideJob         string `json:"sideJob,omitempty"`
	SideJobProgress int    `json:"sideJobProgress,omitempty"`
	Distance        int    `json:"distance,omitempty"`
	TownName        string `json:"townName,omitempty"`
	FightType       string `json:"fightType,omitempty"`
	BossName        string `json:"bossName,omitempty"`
	BossPower       int    `json:"bossPower,omitempty"`
	ArenaFight      bool   `json:"arenaFight,omitempty"`
	Output          string `json:"output"`
}

// NewHeroQuestHandler returns the handler for the hero_quest tool.
func NewHeroQuestHandler(provider HeroProvider) mcp.ToolHandlerFor[HeroQuestParams, HeroQuestResult] {
	return func(
		ctx context.Context,
		_ *mcp.CallToolRequest,
		_ HeroQuestParams,
	) (*mcp.CallToolResult, HeroQuestResult, error) {
		hero, err := provider.GetHero(ctx)
		if err != nil {
			return nil, HeroQuestResult{}, apiErr("hero_quest", err)
		}

		result := HeroQuestResult{
			Quest:           hero.Quest,
			QuestProgress:   hero.QuestProgress,
			SideJob:         hero.SideJob,
			SideJobProgress: hero.SideJobProgress,
			Distance:        hero.Distance,
			TownName:        hero.TownName,
			FightType:       hero.FightType,
			BossName:        hero.BossName,
			BossPower:       hero.BossPower,
			ArenaFight:      hero.ArenaFight,
		}

		result.Output = formatHeroQuest(&result)

		return nil, result, nil
	}
}

// HeroQuestTool returns the MCP tool definition.
func HeroQuestTool() *mcp.Tool {
	return &mcp.Tool{
		Name: "hero_quest",
		Description: "Battle and quest context. Boss name and boss power are public; " +
			"active quest, side job, current town, distance, fight type, and arena-fight " +
			"flag require a userkey.",
		Annotations: readOnlyAnnotations(),
	}
}

func formatHeroQuest(res *HeroQuestResult) string {
	hasPrivate := res.Quest != "" || res.SideJob != "" || res.TownName != "" ||
		res.FightType != "" || res.Distance > 0 || res.ArenaFight
	hasPublic := res.BossName != ""

	if !hasPrivate && !hasPublic {
		return "Quest data unavailable — configure a userkey to enable private fields " +
			"(env GODVILLE_USERKEY or accept the elicitation prompt)."
	}

	var buf strings.Builder

	writeQuestNarrative(&buf, res)
	writeQuestBattle(&buf, res)

	if !hasPrivate {
		buf.WriteString("\n(private fields hidden — configure a userkey to see quest, town, distance, side job)\n")
	}

	return buf.String()
}

func writeQuestNarrative(buf *strings.Builder, res *HeroQuestResult) {
	if res.Quest != "" {
		fmt.Fprintf(buf, "Quest: %s (%d%%)\n", res.Quest, res.QuestProgress)
	}

	if res.SideJob != "" {
		fmt.Fprintf(buf, "Side job: %s (%d%%)\n", res.SideJob, res.SideJobProgress)
	}

	if res.TownName != "" {
		fmt.Fprintf(buf, "Town: %s\n", res.TownName)
	}

	if res.Distance > 0 {
		fmt.Fprintf(buf, "Distance: %d\n", res.Distance)
	}
}

func writeQuestBattle(buf *strings.Builder, res *HeroQuestResult) {
	if res.FightType != "" {
		fmt.Fprintf(buf, "Fight: %s\n", res.FightType)
	}

	if res.BossName != "" {
		if res.BossPower > 0 {
			// boss_power is a raw strength counter, NOT a percentage —
			// rendering "%%" here previously misled LLM consumers into
			// summarising "boss is at 20%" for what is actually a
			// HP-style integer.
			fmt.Fprintf(buf, "Boss: %s (power %d)\n", res.BossName, res.BossPower)
		} else {
			fmt.Fprintf(buf, "Boss: %s\n", res.BossName)
		}
	}

	if res.ArenaFight {
		buf.WriteString("Arena fight: yes\n")
	}
}
