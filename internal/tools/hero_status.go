package tools

import (
	"context"
	"fmt"
	"strings"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// HeroStatusParams has no parameters — the godname is configured server-side.
type HeroStatusParams struct{}

// HeroStatusResult is the structured output of the hero_status tool.
type HeroStatusResult struct {
	Name       string `json:"name,omitempty"`
	Godname    string `json:"godname,omitempty"`
	Gender     string `json:"gender,omitempty"`
	Motto      string `json:"motto,omitempty"`
	Alignment  string `json:"alignment,omitempty"`
	Level      int    `json:"level,omitempty"`
	Experience int    `json:"experience,omitempty"`
	Health     int    `json:"health,omitempty"`
	MaxHealth  int    `json:"maxHealth,omitempty"`
	Godpower   int    `json:"godpower,omitempty"`
	FightType  string `json:"fightType,omitempty"`
	TownName   string `json:"townName,omitempty"`
	Distance   int    `json:"distance,omitempty"`
	Expired    bool   `json:"expired,omitempty"`
	Output     string `json:"output"`
}

// NewHeroStatusHandler returns the handler for the hero_status tool.
func NewHeroStatusHandler(provider HeroProvider) mcp.ToolHandlerFor[HeroStatusParams, HeroStatusResult] {
	return func(
		ctx context.Context,
		_ *mcp.CallToolRequest,
		_ HeroStatusParams,
	) (*mcp.CallToolResult, HeroStatusResult, error) {
		hero, err := provider.GetHero(ctx)
		if err != nil {
			return nil, HeroStatusResult{}, apiErr("hero_status", err)
		}

		result := HeroStatusResult{
			Name:       hero.Name,
			Godname:    hero.Godname,
			Gender:     hero.Gender,
			Motto:      hero.Motto,
			Alignment:  hero.Alignment,
			Level:      hero.Level,
			Experience: hero.Experience,
			Health:     hero.Health,
			MaxHealth:  hero.MaxHealth,
			Godpower:   hero.Godpower,
			FightType:  hero.FightType,
			TownName:   hero.TownName,
			Distance:   hero.Distance,
			Expired:    hero.Expired,
		}
		result.Output = formatHeroStatus(&result)

		return nil, result, nil
	}
}

// HeroStatusTool returns the MCP tool definition.
func HeroStatusTool() *mcp.Tool {
	return &mcp.Tool{
		Name: "hero_status",
		Description: "Current vital stats of the hero: name, godname, gender, motto, alignment, " +
			"level, experience toward next level, health/maxHealth, godpower, fight type, " +
			"town (current location), distance, and an 'expired' flag indicating stale data.",
		Annotations: readOnlyAnnotations(),
	}
}

func formatHeroStatus(res *HeroStatusResult) string {
	var buf strings.Builder

	fmt.Fprintf(&buf, "%s (%s)\n", res.Name, res.Godname)

	writeStringLine(&buf, "  alignment", res.Alignment)
	writeStringLine(&buf, "  motto", res.Motto)
	writeIntLine(&buf, "  level", res.Level)

	if res.Experience > 0 {
		fmt.Fprintf(&buf, "  experience: %d%% toward next level\n", res.Experience)
	}

	switch {
	case res.MaxHealth > 0 && res.Health > 0:
		fmt.Fprintf(&buf, "  health: %d/%d\n", res.Health, res.MaxHealth)
	case res.MaxHealth > 0:
		// Health is private; in public mode it stays zero. Render the cap
		// alone so the line isn't a misleading "0/100".
		fmt.Fprintf(&buf, "  health: ?/%d (private — configure a userkey)\n", res.MaxHealth)
	}

	writeIntLine(&buf, "  godpower", res.Godpower)
	writeStringLine(&buf, "  fight", res.FightType)
	writeStringLine(&buf, "  town", res.TownName)
	writeIntLine(&buf, "  distance", res.Distance)

	if res.Expired {
		buf.WriteString("  expired: true\n")
	}

	return buf.String()
}
