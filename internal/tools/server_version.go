package tools

import (
	"context"
	"fmt"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// ServerVersionToolName is the tool's registered name.
const ServerVersionToolName = "server_version"

// ServerVersionParams has no parameters.
type ServerVersionParams struct{}

// ServerVersionResult reports build info.
type ServerVersionResult struct {
	Version  string `json:"version"`
	Revision string `json:"revision"`
	Runtime  string `json:"runtime"`
	Output   string `json:"output"`
}

// NewServerVersionHandler returns the handler for the server_version tool.
// Unlike hero tools it does not depend on the Godville API.
func NewServerVersionHandler(
	version, revision, runtime string,
) mcp.ToolHandlerFor[ServerVersionParams, ServerVersionResult] {
	return func(
		_ context.Context,
		_ *mcp.CallToolRequest,
		_ ServerVersionParams,
	) (*mcp.CallToolResult, ServerVersionResult, error) {
		out := ServerVersionResult{
			Version:  version,
			Revision: revision,
			Runtime:  runtime,
			Output:   fmt.Sprintf("mcp-godville %s (rev %s, %s)", version, revision, runtime),
		}

		return nil, out, nil
	}
}

// ServerVersionTool returns the MCP tool definition.
func ServerVersionTool() *mcp.Tool {
	return &mcp.Tool{
		Name:        ServerVersionToolName,
		Description: "Server build info: version, git revision, Go runtime. Does not call the Godville API.",
		Annotations: readOnlyAnnotations(),
	}
}
