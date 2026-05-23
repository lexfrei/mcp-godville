package tools

import (
	"context"
	"fmt"
	"strings"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// HeroClanParams has no parameters.
type HeroClanParams struct{}

// HeroClanResult holds clan membership info.
type HeroClanResult struct {
	Clan     string `json:"clan,omitempty"`
	Position string `json:"position,omitempty"`
	InClan   bool   `json:"inClan"`
	Output   string `json:"output"`
}

// NewHeroClanHandler returns the handler for the hero_clan tool.
func NewHeroClanHandler(provider HeroProvider) mcp.ToolHandlerFor[HeroClanParams, HeroClanResult] {
	return func(
		ctx context.Context,
		_ *mcp.CallToolRequest,
		_ HeroClanParams,
	) (*mcp.CallToolResult, HeroClanResult, error) {
		hero, err := provider.GetHero(ctx)
		if err != nil {
			return nil, HeroClanResult{}, apiErr("hero_clan", err)
		}

		result := HeroClanResult{
			Clan:     hero.Clan,
			Position: hero.ClanPos,
			InClan:   hero.Clan != "",
		}
		result.Output = formatHeroClan(&result)

		return nil, result, nil
	}
}

// HeroClanTool returns the MCP tool definition.
func HeroClanTool() *mcp.Tool {
	return &mcp.Tool{
		Name:        "hero_clan",
		Description: "Clan membership and rank.",
		Annotations: readOnlyAnnotations(),
	}
}

func formatHeroClan(res *HeroClanResult) string {
	if !res.InClan {
		return "Not in a clan."
	}

	var buf strings.Builder

	fmt.Fprintf(&buf, "Clan: %s\n", res.Clan)

	if res.Position != "" {
		fmt.Fprintf(&buf, "Position: %s\n", res.Position)
	}

	return buf.String()
}
