package tools

import (
	"context"
	"strings"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// HeroDiaryParams has no parameters.
type HeroDiaryParams struct{}

// HeroDiaryResult contains the most recent diary entry and "third eye" log.
// Both fields are only populated when a userkey is configured.
type HeroDiaryResult struct {
	DiaryLast string `json:"diaryLast,omitempty"`
	EyeLast   string `json:"eyeLast,omitempty"`
	Output    string `json:"output"`
}

// NewHeroDiaryHandler returns the handler for the hero_diary tool.
func NewHeroDiaryHandler(provider HeroProvider) mcp.ToolHandlerFor[HeroDiaryParams, HeroDiaryResult] {
	return func(
		ctx context.Context,
		_ *mcp.CallToolRequest,
		_ HeroDiaryParams,
	) (*mcp.CallToolResult, HeroDiaryResult, error) {
		hero, err := provider.GetHero(ctx)
		if err != nil {
			return nil, HeroDiaryResult{}, apiErr("hero_diary", err)
		}

		result := HeroDiaryResult{
			DiaryLast: hero.DiaryLast,
			EyeLast:   hero.EyeLast,
		}
		result.Output = formatHeroDiary(&result)

		return nil, result, nil
	}
}

// HeroDiaryTool returns the MCP tool definition.
func HeroDiaryTool() *mcp.Tool {
	return &mcp.Tool{
		Name:        "hero_diary",
		Description: "Last diary entry and 'third eye' log line. Requires a userkey (private API) for non-empty values.",
		Annotations: readOnlyAnnotations(),
	}
}

func formatHeroDiary(res *HeroDiaryResult) string {
	if res.DiaryLast == "" && res.EyeLast == "" {
		return "Diary unavailable — configure a userkey to enable private fields " +
			"(env GODVILLE_USERKEY or accept the elicitation prompt)."
	}

	var buf strings.Builder

	if res.DiaryLast != "" {
		buf.WriteString("Diary: ")
		buf.WriteString(res.DiaryLast)
		buf.WriteString("\n")
	}

	if res.EyeLast != "" {
		buf.WriteString("Eye: ")
		buf.WriteString(res.EyeLast)
		buf.WriteString("\n")
	}

	return buf.String()
}
