package tools

import "github.com/modelcontextprotocol/go-sdk/mcp"

// Register wires all hero tools onto the given MCP server.
func Register(server *mcp.Server, provider HeroProvider, version, revision, runtime string) {
	mcp.AddTool(server, ServerVersionTool(), NewServerVersionHandler(version, revision, runtime))
	mcp.AddTool(server, HeroStatusTool(), NewHeroStatusHandler(provider))
	mcp.AddTool(server, HeroDiaryTool(), NewHeroDiaryHandler(provider))
	mcp.AddTool(server, HeroInventoryTool(), NewHeroInventoryHandler(provider))
	mcp.AddTool(server, HeroPetTool(), NewHeroPetHandler(provider))
	mcp.AddTool(server, HeroQuestTool(), NewHeroQuestHandler(provider))
	mcp.AddTool(server, HeroProgressTool(), NewHeroProgressHandler(provider))
	mcp.AddTool(server, HeroClanTool(), NewHeroClanHandler(provider))
	mcp.AddTool(server, HeroRawTool(), NewHeroRawHandler(provider))
}
