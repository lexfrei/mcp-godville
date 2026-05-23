package tools

import (
	"context"
	"fmt"
	"maps"
	"sort"
	"strings"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// HeroInventoryParams has no parameters.
type HeroInventoryParams struct{}

// HeroInventoryResult contains inventory metrics, the legacy itemised map
// (kept for backward compat — Godville removed the itemised `inventory`
// field upstream in favour of `activatables`), and the current activatables
// list. All non-counter fields require GODVILLE_USERKEY.
//
// Count, Max, and Distinct are NOT marked omitempty: a private-mode caller
// MUST be able to distinguish "0 items confirmed" from "field missing".
// The display formatter still skips zero counters in the Output string,
// but the structured JSON carries them so consumers can introspect.
type HeroInventoryResult struct {
	Count        int            `json:"count"`
	Max          int            `json:"max"`
	Items        map[string]int `json:"items,omitempty"`
	Distinct     int            `json:"distinct"`
	Activatables []string       `json:"activatables,omitempty"`
	Output       string         `json:"output"`
}

// NewHeroInventoryHandler returns the handler for the hero_inventory tool.
func NewHeroInventoryHandler(provider HeroProvider) mcp.ToolHandlerFor[HeroInventoryParams, HeroInventoryResult] {
	return func(
		ctx context.Context,
		_ *mcp.CallToolRequest,
		_ HeroInventoryParams,
	) (*mcp.CallToolResult, HeroInventoryResult, error) {
		hero, err := provider.GetHero(ctx)
		if err != nil {
			return nil, HeroInventoryResult{}, apiErr("hero_inventory", err)
		}

		// Defensive copies: hero.Inventory / Activatables live on the
		// cached *Hero shared across all concurrent readers. Handing the
		// originals out risks future code mutating them and corrupting
		// the cache.
		result := HeroInventoryResult{
			Count:        hero.InventoryNum,
			Max:          hero.InventoryMax,
			Items:        copyStringIntMap(hero.Inventory),
			Distinct:     len(hero.Inventory),
			Activatables: append([]string(nil), hero.Activatables...),
		}
		result.Output = formatHeroInventory(&result)

		return nil, result, nil
	}
}

// copyStringIntMap returns a shallow copy of src, or nil for nil input. Used
// at the cache-boundary handoff to keep cached hero state immutable from
// the perspective of every tool that reads from it.
func copyStringIntMap(src map[string]int) map[string]int {
	if src == nil {
		return nil
	}

	out := make(map[string]int, len(src))
	maps.Copy(out, src)

	return out
}

// HeroInventoryTool returns the MCP tool definition.
func HeroInventoryTool() *mcp.Tool {
	return &mcp.Tool{
		Name: "hero_inventory",
		Description: "Inventory usage (count, capacity) and activatables list. " +
			"Counts require GODVILLE_USERKEY; the legacy itemised 'inventory' " +
			"field is deprecated upstream and usually empty on current accounts.",
		Annotations: readOnlyAnnotations(),
	}
}

func formatHeroInventory(res *HeroInventoryResult) string {
	var buf strings.Builder

	switch {
	case res.Max > 0 && res.Count > 0:
		fmt.Fprintf(&buf, "Inventory: %d/%d\n", res.Count, res.Max)
	case res.Max > 0:
		// Count is private; in public mode the cap is known but the live
		// fill is not. Render the placeholder shape that hero_status uses
		// for the symmetric health field, so the two tools stay consistent.
		fmt.Fprintf(&buf, "Inventory: ?/%d (private — configure a userkey)\n", res.Max)
	default:
		fmt.Fprintf(&buf, "Inventory: %d slots\n", res.Count)
	}

	if len(res.Items) > 0 {
		keys := make([]string, 0, len(res.Items))
		for name := range res.Items {
			keys = append(keys, name)
		}

		sort.Strings(keys)

		buf.WriteString("Items (legacy):\n")

		for _, name := range keys {
			fmt.Fprintf(&buf, "  %s × %d\n", name, res.Items[name])
		}
	}

	if len(res.Activatables) > 0 {
		buf.WriteString("Activatables:\n")

		for _, name := range res.Activatables {
			fmt.Fprintf(&buf, "  - %s\n", name)
		}
	}

	return buf.String()
}
