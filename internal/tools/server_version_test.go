package tools_test

import (
	"context"
	"strings"
	"testing"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/lexfrei/mcp-godville/internal/tools"
)

func TestServerVersionHandler(t *testing.T) {
	handler := tools.NewServerVersionHandler("v1.2.3", "abcdef0", "go1.26.0")

	_, out, err := handler(context.Background(), &mcp.CallToolRequest{}, tools.ServerVersionParams{})
	if err != nil {
		t.Fatalf("handler failed: %v", err)
	}

	if out.Version != "v1.2.3" {
		t.Errorf("expected version v1.2.3, got %q", out.Version)
	}

	if !strings.Contains(out.Output, "v1.2.3") || !strings.Contains(out.Output, "abcdef0") {
		t.Errorf("expected version+revision in output, got %q", out.Output)
	}
}

func TestServerVersionTool_Definition(t *testing.T) {
	tool := tools.ServerVersionTool()

	if tool.Name != tools.ServerVersionToolName {
		t.Errorf("name mismatch")
	}

	if tool.Annotations == nil || !tool.Annotations.ReadOnlyHint {
		t.Error("expected readOnly hint")
	}
}
