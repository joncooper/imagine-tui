package mcp_test

import (
	"testing"

	"github.com/mark3labs/mcp-go/mcp"
)

func TestMCPGoImport(t *testing.T) {
	// Verify mcp-go dependency is available and the core types are usable.
	tool := mcp.NewTool("test_tool",
		mcp.WithDescription("A test tool"),
	)
	if tool.Name != "test_tool" {
		t.Fatalf("expected tool name 'test_tool', got %q", tool.Name)
	}
	t.Log("mcp-go import OK")
}
