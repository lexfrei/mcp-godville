package tools_test

import (
	"context"
	"testing"

	"github.com/cockroachdb/errors"
	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/lexfrei/mcp-godville/internal/tools"
)

// Regression: every hero_* tool wraps provider errors as tools.ErrAPI via
// apiErr. A future change that drops the wrap in any single tool would
// silently bypass the sentinel categorisation downstream consumers expect.
// One parametrized table covers all of them.
func TestAllHeroTools_WrapProviderErrorAsErrAPI(t *testing.T) {
	boom := errors.New("provider boom")
	prov := &errProvider{err: boom}

	type call func() error

	cases := []struct {
		name string
		fn   call
	}{
		{
			"hero_status",
			func() error {
				_, _, err := tools.NewHeroStatusHandler(prov)(context.Background(),
					&mcp.CallToolRequest{}, tools.HeroStatusParams{})

				return err
			},
		},
		{
			"hero_diary",
			func() error {
				_, _, err := tools.NewHeroDiaryHandler(prov)(context.Background(),
					&mcp.CallToolRequest{}, tools.HeroDiaryParams{})

				return err
			},
		},
		{
			"hero_inventory",
			func() error {
				_, _, err := tools.NewHeroInventoryHandler(prov)(context.Background(),
					&mcp.CallToolRequest{}, tools.HeroInventoryParams{})

				return err
			},
		},
		{
			"hero_pet",
			func() error {
				_, _, err := tools.NewHeroPetHandler(prov)(context.Background(),
					&mcp.CallToolRequest{}, tools.HeroPetParams{})

				return err
			},
		},
		{
			"hero_quest",
			func() error {
				_, _, err := tools.NewHeroQuestHandler(prov)(context.Background(),
					&mcp.CallToolRequest{}, tools.HeroQuestParams{})

				return err
			},
		},
		{
			"hero_progress",
			func() error {
				_, _, err := tools.NewHeroProgressHandler(prov)(context.Background(),
					&mcp.CallToolRequest{}, tools.HeroProgressParams{})

				return err
			},
		},
		{
			"hero_clan",
			func() error {
				_, _, err := tools.NewHeroClanHandler(prov)(context.Background(),
					&mcp.CallToolRequest{}, tools.HeroClanParams{})

				return err
			},
		},
		{
			"hero_raw",
			func() error {
				_, _, err := tools.NewHeroRawHandler(prov)(context.Background(),
					&mcp.CallToolRequest{}, tools.HeroRawParams{})

				return err
			},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			err := tc.fn()
			if err == nil {
				t.Fatalf("%s: expected error from provider boom", tc.name)
			}

			if !errors.Is(err, tools.ErrAPI) {
				t.Errorf("%s: expected ErrAPI, got: %v", tc.name, err)
			}

			if !errors.Is(err, boom) {
				t.Errorf("%s: expected inner boom preserved, got: %v", tc.name, err)
			}
		})
	}
}
