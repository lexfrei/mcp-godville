package tools_test

import (
	"context"
	"strings"
	"testing"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/lexfrei/mcp-godville/internal/godville"
	"github.com/lexfrei/mcp-godville/internal/tools"
)

func TestHeroClanHandler_Present(t *testing.T) {
	prov := &stubProvider{hero: &godville.Hero{
		Clan:    "Орден Кирпича",
		ClanPos: "сенатор",
	}}

	handler := tools.NewHeroClanHandler(prov)

	_, out, err := handler(context.Background(), &mcp.CallToolRequest{}, tools.HeroClanParams{})
	if err != nil {
		t.Fatalf("handler failed: %v", err)
	}

	if !out.InClan {
		t.Error("expected InClan=true")
	}

	if !strings.Contains(out.Output, "Орден Кирпича") {
		t.Errorf("expected clan name in output, got %q", out.Output)
	}

	if !strings.Contains(out.Output, "сенатор") {
		t.Errorf("expected position in output, got %q", out.Output)
	}
}

func TestHeroClanHandler_NoClan(t *testing.T) {
	prov := &stubProvider{hero: &godville.Hero{}}

	handler := tools.NewHeroClanHandler(prov)

	_, out, err := handler(context.Background(), &mcp.CallToolRequest{}, tools.HeroClanParams{})
	if err != nil {
		t.Fatalf("handler failed: %v", err)
	}

	if out.InClan {
		t.Error("expected InClan=false")
	}

	if out.Output != "Not in a clan." {
		t.Errorf("expected no-clan message, got %q", out.Output)
	}
}
