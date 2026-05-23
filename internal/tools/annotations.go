package tools

import "github.com/modelcontextprotocol/go-sdk/mcp"

// readOnlyAnnotations marks a tool as a pure read operation.
func readOnlyAnnotations() *mcp.ToolAnnotations {
	return &mcp.ToolAnnotations{
		ReadOnlyHint: true,
	}
}
