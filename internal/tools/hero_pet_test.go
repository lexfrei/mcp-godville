package tools_test

import (
	"context"
	"strings"
	"testing"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/lexfrei/mcp-godville/internal/godville"
	"github.com/lexfrei/mcp-godville/internal/tools"
)

func TestHeroPetHandler_Present(t *testing.T) {
	prov := &stubProvider{hero: &godville.Hero{
		Pet: &godville.Pet{
			Name:    "Шушпанчик",
			Class:   "Шушпанчик-вертолёт",
			Level:   7,
			Wounded: true,
		},
	}}

	handler := tools.NewHeroPetHandler(prov)

	_, out, err := handler(context.Background(), &mcp.CallToolRequest{}, tools.HeroPetParams{})
	if err != nil {
		t.Fatalf("handler failed: %v", err)
	}

	if !out.HasPet {
		t.Error("expected HasPet=true")
	}

	if out.Level != 7 {
		t.Errorf("expected level 7, got %d", out.Level)
	}

	if !strings.Contains(out.Output, "Шушпанчик") {
		t.Errorf("expected pet name in output, got %q", out.Output)
	}

	if !strings.Contains(out.Output, "wounded") {
		t.Errorf("expected wounded flag in output, got %q", out.Output)
	}
}

// Regression: {"pet": {}} unmarshalls into a non-nil zero Pet struct; the
// tool must treat that as "no pet" rather than render an empty " ()" header.
func TestHeroPetHandler_EmptyPetObjectTreatedAsAbsent(t *testing.T) {
	prov := &stubProvider{hero: &godville.Hero{Pet: &godville.Pet{}}}

	handler := tools.NewHeroPetHandler(prov)

	_, out, err := handler(context.Background(), &mcp.CallToolRequest{}, tools.HeroPetParams{})
	if err != nil {
		t.Fatalf("handler failed: %v", err)
	}

	if out.HasPet {
		t.Error("expected HasPet=false for zero Pet struct")
	}

	if out.Output != "No pet yet." {
		t.Errorf("expected no-pet message, got %q", out.Output)
	}
}

func TestHeroPetHandler_Absent(t *testing.T) {
	prov := &stubProvider{hero: &godville.Hero{}}

	handler := tools.NewHeroPetHandler(prov)

	_, out, err := handler(context.Background(), &mcp.CallToolRequest{}, tools.HeroPetParams{})
	if err != nil {
		t.Fatalf("handler failed: %v", err)
	}

	if out.HasPet {
		t.Error("expected HasPet=false")
	}

	if out.Output != "No pet yet." {
		t.Errorf("expected no-pet message, got %q", out.Output)
	}
}
