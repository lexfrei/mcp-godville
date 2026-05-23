package tools

import (
	"context"
	"fmt"
	"strings"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// HeroPetParams has no parameters.
type HeroPetParams struct{}

// HeroPetResult contains pet info.
type HeroPetResult struct {
	Name    string `json:"name,omitempty"`
	Class   string `json:"class,omitempty"`
	Level   int    `json:"level,omitempty"`
	Wounded bool   `json:"wounded,omitempty"`
	HasPet  bool   `json:"hasPet"`
	Output  string `json:"output"`
}

// NewHeroPetHandler returns the handler for the hero_pet tool.
func NewHeroPetHandler(provider HeroProvider) mcp.ToolHandlerFor[HeroPetParams, HeroPetResult] {
	return func(
		ctx context.Context,
		_ *mcp.CallToolRequest,
		_ HeroPetParams,
	) (*mcp.CallToolResult, HeroPetResult, error) {
		hero, err := provider.GetHero(ctx)
		if err != nil {
			return nil, HeroPetResult{}, apiErr("hero_pet", err)
		}

		result := HeroPetResult{HasPet: hero.Pet.HasContent()}

		if result.HasPet {
			result.Name = hero.Pet.Name
			result.Class = hero.Pet.Class
			result.Level = hero.Pet.Level
			result.Wounded = hero.Pet.Wounded
		}

		result.Output = formatHeroPet(&result)

		return nil, result, nil
	}
}

// HeroPetTool returns the MCP tool definition.
func HeroPetTool() *mcp.Tool {
	return &mcp.Tool{
		Name:        "hero_pet",
		Description: "Hero's pet: name, class, level, wounded state.",
		Annotations: readOnlyAnnotations(),
	}
}

func formatHeroPet(res *HeroPetResult) string {
	if !res.HasPet {
		return "No pet yet."
	}

	var buf strings.Builder

	fmt.Fprintf(&buf, "%s (%s)\n", res.Name, res.Class)

	if res.Level > 0 {
		fmt.Fprintf(&buf, "  level: %d\n", res.Level)
	}

	if res.Wounded {
		buf.WriteString("  wounded: true\n")
	}

	return buf.String()
}
