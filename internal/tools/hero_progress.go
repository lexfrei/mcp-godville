package tools

import (
	"context"
	"fmt"
	"strings"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// HeroProgressParams has no parameters.
type HeroProgressParams struct{}

// HeroProgressResult bundles long-term progress trackers (mostly public).
type HeroProgressResult struct {
	Level              int    `json:"level,omitempty"`
	GoldApprox         string `json:"goldApprox,omitempty"`
	Savings            string `json:"savings,omitempty"`
	BricksCnt          int    `json:"bricksCnt,omitempty"`
	WoodCnt            int    `json:"woodCnt,omitempty"`
	ArenaWon           int    `json:"arenaWon,omitempty"`
	ArenaLost          int    `json:"arenaLost,omitempty"`
	TLevel             int    `json:"tLevel,omitempty"`
	ArkMale            int    `json:"arkMale,omitempty"`
	ArkFemale          int    `json:"arkFemale,omitempty"`
	SoulsPercent       string `json:"soulsPercent,omitempty"`
	RelicsPercent      string `json:"relicsPercent,omitempty"`
	Words              int    `json:"words,omitempty"`
	ShopName           string `json:"shopName,omitempty"`
	ArkName            string `json:"arkName,omitempty"`
	TempleCompletedAt  string `json:"templeCompletedAt,omitempty"`
	ArkCompletedAt     string `json:"arkCompletedAt,omitempty"`
	SavingsCompletedAt string `json:"savingsCompletedAt,omitempty"`
	BookAt             string `json:"bookAt,omitempty"`
	SoulsAt            string `json:"soulsAt,omitempty"`
	PairsAt            string `json:"pairsAt,omitempty"`
	Output             string `json:"output"`
}

// NewHeroProgressHandler returns the handler for the hero_progress tool.
func NewHeroProgressHandler(provider HeroProvider) mcp.ToolHandlerFor[HeroProgressParams, HeroProgressResult] {
	return func(
		ctx context.Context,
		_ *mcp.CallToolRequest,
		_ HeroProgressParams,
	) (*mcp.CallToolResult, HeroProgressResult, error) {
		hero, err := provider.GetHero(ctx)
		if err != nil {
			return nil, HeroProgressResult{}, apiErr("hero_progress", err)
		}

		result := HeroProgressResult{
			Level:              hero.Level,
			GoldApprox:         hero.GoldApprox,
			Savings:            hero.Savings,
			BricksCnt:          hero.BricksCnt,
			WoodCnt:            hero.WoodCnt,
			ArenaWon:           hero.ArenaWon,
			ArenaLost:          hero.ArenaLost,
			TLevel:             hero.TLevel,
			ArkMale:            hero.ArkMale,
			ArkFemale:          hero.ArkFemale,
			SoulsPercent:       hero.SoulsPercent,
			RelicsPercent:      hero.RelicsPercent,
			Words:              hero.Words,
			ShopName:           hero.ShopName,
			ArkName:            hero.ArkName,
			TempleCompletedAt:  hero.TempleCompletedAt,
			ArkCompletedAt:     hero.ArkCompletedAt,
			SavingsCompletedAt: hero.SavingsCompletedAt,
			BookAt:             hero.BookAt,
			SoulsAt:            hero.SoulsAt,
			PairsAt:            hero.PairsAt,
		}
		result.Output = formatHeroProgress(&result)

		return nil, result, nil
	}
}

// Formatter convention: lines for numeric counters are omitted when the
// value is zero. Zero counters are usually "no data yet" rather than
// "explicitly zero", and rendering every zero-valued field clutters the
// LLM-facing summary.
//
// Result fields are tagged `omitempty`, so JSON serialisation likewise
// drops zero counters (matching the Output string's shape, on purpose).
// If a consumer needs to distinguish "0 confirmed" from "field missing",
// they should call `hero_raw` and inspect the upstream payload directly —
// hero_progress deliberately collapses both into "absence" for the
// summary use case. hero_inventory takes the opposite choice (count/max/
// distinct are NOT omitempty) because there the distinction matters for
// budgeting decisions; that asymmetry is intentional.

// HeroProgressTool returns the MCP tool definition.
func HeroProgressTool() *mcp.Tool {
	return &mcp.Tool{
		Name: "hero_progress",
		Description: "Long-term progress: level, gold, savings, bricks, wood, arena, ark, t-level, " +
			"souls/relics percent, words written, shop name, ark name, " +
			"milestone completion timestamps (temple, ark, savings, book, souls, pairs).",
		Annotations: readOnlyAnnotations(),
	}
}

func formatHeroProgress(res *HeroProgressResult) string {
	var buf strings.Builder

	writeProgressCounters(&buf, res)
	writeProgressMilestones(&buf, res)

	if buf.Len() == 0 {
		return "No progress data."
	}

	return buf.String()
}

func writeProgressCounters(buf *strings.Builder, res *HeroProgressResult) {
	writeIntLine(buf, "Level", res.Level)
	writeStringLine(buf, "Gold (approx)", res.GoldApprox)
	writeStringLine(buf, "Savings", res.Savings)
	writeIntLine(buf, "Bricks", res.BricksCnt)
	writeIntLine(buf, "Wood", res.WoodCnt)

	if res.ArenaWon > 0 || res.ArenaLost > 0 {
		fmt.Fprintf(buf, "Arena: %dW / %dL\n", res.ArenaWon, res.ArenaLost)
	}

	writeIntLine(buf, "T-level", res.TLevel)

	if res.ArkMale > 0 || res.ArkFemale > 0 {
		fmt.Fprintf(buf, "Ark: ♂%d / ♀%d\n", res.ArkMale, res.ArkFemale)
	}

	writeIntLine(buf, "Words", res.Words)
}

func writeProgressMilestones(buf *strings.Builder, res *HeroProgressResult) {
	writeStringLine(buf, "Temple completed", res.TempleCompletedAt)

	switch {
	case res.ArkCompletedAt != "" && res.ArkName != "":
		fmt.Fprintf(buf, "Ark completed: %s (%s)\n", res.ArkCompletedAt, res.ArkName)
	case res.ArkCompletedAt != "":
		fmt.Fprintf(buf, "Ark completed: %s\n", res.ArkCompletedAt)
	case res.ArkName != "":
		// In-progress ark: name picked but not yet completed.
		fmt.Fprintf(buf, "Ark name: %s\n", res.ArkName)
	}

	writeStringLine(buf, "Savings completed", res.SavingsCompletedAt)
	writeStringLine(buf, "Book at", res.BookAt)
	writeSoulsLine(buf, res)
	writeStringLine(buf, "Pairs at", res.PairsAt)
	writeStringLine(buf, "Shop", res.ShopName)
}

func writeSoulsLine(buf *strings.Builder, res *HeroProgressResult) {
	switch {
	case res.SoulsAt != "" && res.SoulsPercent != "":
		fmt.Fprintf(buf, "Souls at: %s (%s%%)\n", res.SoulsAt, res.SoulsPercent)
	case res.SoulsAt != "":
		fmt.Fprintf(buf, "Souls at: %s\n", res.SoulsAt)
	case res.SoulsPercent != "":
		fmt.Fprintf(buf, "Souls: %s%%\n", res.SoulsPercent)
	}

	if res.RelicsPercent != "" {
		fmt.Fprintf(buf, "Relics: %s%%\n", res.RelicsPercent)
	}
}
