package tools_test

import (
	"context"
	"strings"
	"testing"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/lexfrei/mcp-godville/internal/godville"
	"github.com/lexfrei/mcp-godville/internal/tools"
)

func TestHeroDiaryHandler_Populated(t *testing.T) {
	prov := &stubProvider{hero: &godville.Hero{
		DiaryLast: "12:00 Нашёл кирпич.",
		EyeLast:   "12:01 Босс злится.",
	}}

	handler := tools.NewHeroDiaryHandler(prov)

	_, out, err := handler(context.Background(), &mcp.CallToolRequest{}, tools.HeroDiaryParams{})
	if err != nil {
		t.Fatalf("handler failed: %v", err)
	}

	if !strings.Contains(out.DiaryLast, "кирпич") {
		t.Errorf("expected diary preserved, got %q", out.DiaryLast)
	}

	if !strings.Contains(out.Output, "Diary:") || !strings.Contains(out.Output, "Eye:") {
		t.Errorf("expected output to contain both sections, got %q", out.Output)
	}
}

func TestHeroDiaryHandler_PublicModeHint(t *testing.T) {
	prov := &stubProvider{hero: &godville.Hero{}}

	handler := tools.NewHeroDiaryHandler(prov)

	_, out, err := handler(context.Background(), &mcp.CallToolRequest{}, tools.HeroDiaryParams{})
	if err != nil {
		t.Fatalf("handler failed: %v", err)
	}

	if !strings.Contains(out.Output, "GODVILLE_USERKEY") {
		t.Errorf("expected hint about userkey, got %q", out.Output)
	}
}
