package tools

import (
	"context"
	"encoding/json"

	"github.com/cockroachdb/errors"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// HeroRawParams has no parameters.
type HeroRawParams struct{}

// HeroRawResult is the full Godville API JSON for the configured hero.
// Use this as an escape hatch for fields not modelled by the typed tools.
//
// Data is a generic map for typed access; numbers are float64 and large
// integers (IDs, timestamps past 2^53) lose precision in this form.
// Output is the byte-exact API response, suitable for inspecting fields
// where precision matters or where field ordering is significant.
type HeroRawResult struct {
	Data   map[string]any `json:"data"`
	Output string         `json:"output"`
}

// NewHeroRawHandler returns the handler for the hero_raw tool.
func NewHeroRawHandler(provider HeroProvider) mcp.ToolHandlerFor[HeroRawParams, HeroRawResult] {
	return func(
		ctx context.Context,
		_ *mcp.CallToolRequest,
		_ HeroRawParams,
	) (*mcp.CallToolResult, HeroRawResult, error) {
		hero, err := provider.GetHero(ctx)
		if err != nil {
			return nil, HeroRawResult{}, apiErr("hero_raw", err)
		}

		// Output: byte-exact original API response, preserves field
		// ordering and integer precision past 2^53.
		// Data: convenience-shaped map[string]any deep-copied from the
		// cached hero (the cache shares the map by reference, and any
		// future post-processing mutation would corrupt the cache).
		var copied map[string]any

		if len(hero.RawBytes) > 0 {
			copyErr := json.Unmarshal(hero.RawBytes, &copied)
			if copyErr != nil {
				return nil, HeroRawResult{}, apiErr("hero_raw", errors.Wrap(copyErr, "deep copy raw"))
			}
		}

		result := HeroRawResult{
			Data:   copied,
			Output: string(hero.RawBytes),
		}

		return nil, result, nil
	}
}

// HeroRawTool returns the MCP tool definition.
func HeroRawTool() *mcp.Tool {
	return &mcp.Tool{
		Name:        "hero_raw",
		Description: "Full raw Godville API payload for the configured hero. Use to inspect fields not exposed by the typed tools.",
		Annotations: readOnlyAnnotations(),
	}
}
