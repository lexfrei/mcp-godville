package tools_test

import (
	"context"
	"strings"
	"testing"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/lexfrei/mcp-godville/internal/godville"
	"github.com/lexfrei/mcp-godville/internal/tools"
)

func TestHeroRawHandler_ReturnsRawPayload(t *testing.T) {
	body := `{"name":"Sir Testalot","some_unknown":"field","nested":{"a":1.0}}`

	prov := &stubProvider{hero: &godville.Hero{
		Raw: map[string]any{
			"name":         "Sir Testalot",
			"some_unknown": "field",
			"nested":       map[string]any{"a": 1.0},
		},
		RawBytes: []byte(body),
	}}

	handler := tools.NewHeroRawHandler(prov)

	_, out, err := handler(context.Background(), &mcp.CallToolRequest{}, tools.HeroRawParams{})
	if err != nil {
		t.Fatalf("handler failed: %v", err)
	}

	if out.Data["some_unknown"] != "field" {
		t.Errorf("expected unknown field preserved in Data, got %v", out.Data["some_unknown"])
	}

	if !strings.Contains(out.Output, "some_unknown") {
		t.Errorf("expected unknown field in JSON output, got %q", out.Output)
	}
}

// Regression: hero_raw's Output must preserve byte-exact integer precision.
// Decoding through map[string]any turns numbers into float64; an integer
// past 2^53 round-trips lossily. Output uses the original bytes, so the
// canary stays intact.
func TestHeroRawHandler_OutputPreservesLargeIntegerPrecision(t *testing.T) {
	body := `{"large_id":9007199254740993}`

	prov := &stubProvider{hero: &godville.Hero{
		Raw:      map[string]any{"large_id": float64(9007199254740993)}, // lossy, by construction
		RawBytes: []byte(body),
	}}

	handler := tools.NewHeroRawHandler(prov)

	_, out, err := handler(context.Background(), &mcp.CallToolRequest{}, tools.HeroRawParams{})
	if err != nil {
		t.Fatalf("handler failed: %v", err)
	}

	if !strings.Contains(out.Output, "9007199254740993") {
		t.Errorf("expected exact integer preserved in Output, got %q", out.Output)
	}
}
