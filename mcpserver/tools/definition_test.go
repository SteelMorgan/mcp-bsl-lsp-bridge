package tools

import (
	"context"
	"errors"
	"strings"
	"testing"

	"rockerboo/mcp-lsp-bridge/mocks"
	"rockerboo/mcp-lsp-bridge/types"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/mcptest"
	"github.com/mark3labs/mcp-go/server"
	"github.com/myleshyson/lsprotocol-go/protocol"
)

func TestDefinitionTool_Success_InferLanguage(t *testing.T) {
	bridge := &mocks.MockBridge{}

	uri := "file:///test.bsl"
	lang := types.Language("bsl")

	defs := []protocol.Or2[protocol.LocationLink, protocol.Location]{
		{
			Value: protocol.Location{
				Uri: protocol.DocumentUri("file:///a.bsl"),
				Range: protocol.Range{
					Start: protocol.Position{Line: 1, Character: 2},
					End:   protocol.Position{Line: 1, Character: 5},
				},
			},
		},
		{
			Value: protocol.LocationLink{
				TargetUri: protocol.DocumentUri("file:///b.bsl"),
				TargetRange: protocol.Range{
					Start: protocol.Position{Line: 10, Character: 0},
					End:   protocol.Position{Line: 10, Character: 3},
				},
			},
		},
	}

	bridge.On("InferLanguage", uri).Return(&lang, nil)
	bridge.On("FindSymbolDefinitions", "bsl", uri, uint32(5), uint32(7)).Return(defs, nil)

	tool, handler := DefinitionTool(bridge)
	mcpServer, err := mcptest.NewServer(t, server.ServerTool{Tool: tool, Handler: handler})
	if err != nil {
		t.Fatalf("Could not create MCP server: %v", err)
	}

	ctx := context.Background()
	toolResult, err := mcpServer.Client().CallTool(ctx, mcp.CallToolRequest{
		Request: mcp.Request{Method: "tools/call"},
		Params: mcp.CallToolParams{
			Name: "definition",
			Arguments: map[string]any{
				"uri":       uri,
				"line":      5,
				"character": 7,
			},
		},
	})
	if err != nil {
		t.Fatalf("Error calling tool: %v", err)
	}
	if toolResult.IsError {
		t.Fatalf("Expected success, got error: %#v", toolResult.Content)
	}
	if len(toolResult.Content) == 0 {
		t.Fatalf("Expected content, got empty")
	}
	text, ok := toolResult.Content[0].(mcp.TextContent)
	if !ok {
		t.Fatalf("Expected text content, got: %T", toolResult.Content[0])
	}
	if !strings.Contains(text.Text, "DEFINITION:") || !strings.Contains(text.Text, "Count: 2") {
		t.Fatalf("Unexpected output: %q", text.Text)
	}

	bridge.AssertExpectations(t)
}

func TestDefinitionTool_Success_LanguageOverride(t *testing.T) {
	bridge := &mocks.MockBridge{}

	uri := "file:///test.bsl"
	defs := []protocol.Or2[protocol.LocationLink, protocol.Location]{}

	// Override should skip InferLanguage and use provided language (lowercased).
	bridge.On("FindSymbolDefinitions", "bsl", uri, uint32(0), uint32(0)).Return(defs, nil)

	tool, handler := DefinitionTool(bridge)
	mcpServer, err := mcptest.NewServer(t, server.ServerTool{Tool: tool, Handler: handler})
	if err != nil {
		t.Fatalf("Could not create MCP server: %v", err)
	}

	ctx := context.Background()
	toolResult, err := mcpServer.Client().CallTool(ctx, mcp.CallToolRequest{
		Request: mcp.Request{Method: "tools/call"},
		Params: mcp.CallToolParams{
			Name: "definition",
			Arguments: map[string]any{
				"uri":       uri,
				"line":      0,
				"character": 0,
				"language":  "BSL",
			},
		},
	})
	if err != nil {
		t.Fatalf("Error calling tool: %v", err)
	}
	if toolResult.IsError {
		t.Fatalf("Expected success, got error: %#v", toolResult.Content)
	}

	bridge.AssertExpectations(t)
}

func TestDefinitionTool_Error_InferLanguageFailure(t *testing.T) {
	bridge := &mocks.MockBridge{}

	uri := "file:///unknown"
	bridge.On("InferLanguage", uri).Return((*types.Language)(nil), errors.New("no extension"))

	tool, handler := DefinitionTool(bridge)
	mcpServer, err := mcptest.NewServer(t, server.ServerTool{Tool: tool, Handler: handler})
	if err != nil {
		t.Fatalf("Could not create MCP server: %v", err)
	}

	ctx := context.Background()
	toolResult, err := mcpServer.Client().CallTool(ctx, mcp.CallToolRequest{
		Request: mcp.Request{Method: "tools/call"},
		Params: mcp.CallToolParams{
			Name: "definition",
			Arguments: map[string]any{
				"uri":       uri,
				"line":      0,
				"character": 0,
			},
		},
	})
	if err != nil {
		t.Fatalf("Error calling tool: %v", err)
	}
	if !toolResult.IsError {
		t.Fatalf("Expected error, got success: %#v", toolResult.Content)
	}

	bridge.AssertExpectations(t)
}
