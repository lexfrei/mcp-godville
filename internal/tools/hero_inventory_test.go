package tools_test

import (
	"context"
	"strings"
	"testing"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/lexfrei/mcp-godville/internal/godville"
	"github.com/lexfrei/mcp-godville/internal/tools"
)

func TestHeroInventoryHandler_PrivateMode(t *testing.T) {
	prov := &stubProvider{hero: &godville.Hero{
		InventoryNum: 12,
		InventoryMax: 20,
		Inventory: map[string]int{
			"меч":    1,
			"зелье":  3,
			"кирпич": 4,
		},
	}}

	handler := tools.NewHeroInventoryHandler(prov)

	_, out, err := handler(context.Background(), &mcp.CallToolRequest{}, tools.HeroInventoryParams{})
	if err != nil {
		t.Fatalf("handler failed: %v", err)
	}

	if out.Count != 12 || out.Max != 20 {
		t.Errorf("expected 12/20, got %d/%d", out.Count, out.Max)
	}

	if out.Distinct != 3 {
		t.Errorf("expected 3 distinct items, got %d", out.Distinct)
	}

	if !strings.Contains(out.Output, "12/20") {
		t.Errorf("expected output to contain capacity, got %q", out.Output)
	}

	if !strings.Contains(out.Output, "зелье × 3") {
		t.Errorf("expected itemised inventory, got %q", out.Output)
	}
}

func TestHeroInventoryHandler_PublicMode(t *testing.T) {
	prov := &stubProvider{hero: &godville.Hero{InventoryMax: 30}}

	handler := tools.NewHeroInventoryHandler(prov)

	_, out, err := handler(context.Background(), &mcp.CallToolRequest{}, tools.HeroInventoryParams{})
	if err != nil {
		t.Fatalf("handler failed: %v", err)
	}

	if len(out.Items) != 0 {
		t.Errorf("expected no items in public mode, got %d", len(out.Items))
	}

	// Symmetric with hero_status: avoid the misleading "0/30" shape (the
	// hero isn't on death's door, the live count is just hidden).
	if strings.Contains(out.Output, "0/30") {
		t.Errorf("expected NOT to render '0/30' in public mode, got %q", out.Output)
	}

	if !strings.Contains(out.Output, "?/30") {
		t.Errorf("expected '?/30' placeholder in public mode, got %q", out.Output)
	}

	if !strings.Contains(out.Output, "configure a userkey") {
		t.Errorf("expected 'configure a userkey' hint, got %q", out.Output)
	}
}

// Regression: the tool must not hand out the cache's underlying maps and
// slices. Mutating result.Items must NOT corrupt the cached hero state.
func TestHeroInventoryHandler_DefensiveCopies(t *testing.T) {
	hero := &godville.Hero{
		InventoryNum: 1,
		InventoryMax: 30,
		Inventory:    map[string]int{"меч": 1},
		Activatables: []string{"амулет"},
	}

	prov := &stubProvider{hero: hero}
	handler := tools.NewHeroInventoryHandler(prov)

	_, out, err := handler(context.Background(), &mcp.CallToolRequest{}, tools.HeroInventoryParams{})
	if err != nil {
		t.Fatalf("handler failed: %v", err)
	}

	// Mutate the result's collections.
	out.Items["меч"] = 999
	out.Items["новое"] = 42
	out.Activatables[0] = "испорчено"
	out.Activatables = append(out.Activatables, "и ещё")

	if hero.Inventory["меч"] != 1 || len(hero.Inventory) != 1 {
		t.Errorf("cached Inventory was mutated: %+v", hero.Inventory)
	}

	if hero.Activatables[0] != "амулет" || len(hero.Activatables) != 1 {
		t.Errorf("cached Activatables was mutated: %+v", hero.Activatables)
	}
}

// Regression: Godville removed the itemised `inventory` field upstream and
// replaced it with `activatables`. The tool must surface activatables in the
// output so the README's promise about itemisation is not a lie.
func TestHeroInventoryHandler_SurfacesActivatables(t *testing.T) {
	prov := &stubProvider{hero: &godville.Hero{
		InventoryNum: 5,
		InventoryMax: 30,
		Activatables: []string{"амулет небытия", "посох с глазом"},
	}}

	handler := tools.NewHeroInventoryHandler(prov)

	_, out, err := handler(context.Background(), &mcp.CallToolRequest{}, tools.HeroInventoryParams{})
	if err != nil {
		t.Fatalf("handler failed: %v", err)
	}

	if len(out.Activatables) != 2 {
		t.Errorf("expected 2 activatables, got %d", len(out.Activatables))
	}

	if !strings.Contains(out.Output, "амулет небытия") {
		t.Errorf("expected activatables in output, got %q", out.Output)
	}
}
