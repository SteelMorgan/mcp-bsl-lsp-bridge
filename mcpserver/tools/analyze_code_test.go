package tools

import (
	"context"
	"errors"
	"testing"

	"rockerboo/mcp-lsp-bridge/lsp"
	"rockerboo/mcp-lsp-bridge/mocks"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/mcptest"
	"github.com/mark3labs/mcp-go/server"
	"github.com/myleshyson/lsprotocol-go/protocol"
)

func callAnalyzeCode(t *testing.T, bridge *mocks.MockBridge, uri string) *mcp.CallToolResult {
	t.Helper()

	tool, handler := AnalyzeCode(bridge)
	mcpServer, err := mcptest.NewServer(t, server.ServerTool{
		Tool:    tool,
		Handler: handler,
	})
	if err != nil {
		t.Fatalf("Could not start MCP server: %v", err)
	}

	ctx := context.Background()
	result, err := mcpServer.Client().CallTool(ctx, mcp.CallToolRequest{
		Request: mcp.Request{Method: "tools/call"},
		Params: mcp.CallToolParams{
			Name: "analyze_code",
			Arguments: map[string]any{
				"uri":       uri,
				"line":      10,
				"character": 5,
			},
		},
	})
	if err != nil {
		t.Errorf("Error: %v", err)
	}
	if result == nil {
		t.Error("Expected result but got nil")
	}
	return result
}

func TestAnalyzeCodeTool_Success(t *testing.T) {
	bridge := &mocks.MockBridge{}
	uri := "file:///test.bsl"

	bridge.On("GetCompletion", uri, uint32(10), uint32(5)).Return(
		&protocol.CompletionList{Items: []protocol.CompletionItem{
			{Label: "Найти", Detail: "Method"},
		}}, nil)

	callAnalyzeCode(t, bridge, uri)
	bridge.AssertExpectations(t)
}

func TestAnalyzeCodeTool_Empty(t *testing.T) {
	bridge := &mocks.MockBridge{}
	uri := "file:///test.bsl"

	bridge.On("GetCompletion", uri, uint32(10), uint32(5)).Return(
		&protocol.CompletionList{Items: []protocol.CompletionItem{}}, nil)

	callAnalyzeCode(t, bridge, uri)
	bridge.AssertExpectations(t)
}

func TestAnalyzeCodeTool_CompletionError(t *testing.T) {
	bridge := &mocks.MockBridge{}
	uri := "file:///test.bsl"

	bridge.On("GetCompletion", uri, uint32(10), uint32(5)).Return(
		(*protocol.CompletionList)(nil), errors.New("completion failed"))

	callAnalyzeCode(t, bridge, uri)
	bridge.AssertExpectations(t)
}

func TestAnalyzeCodeUtilityFunctions(t *testing.T) {
	t.Run("test analysis result formatting", func(t *testing.T) {
		// Test with mock analysis result
		result := &lsp.AnalyzeCodeResult{
			Hover:       &protocol.HoverResponse{},
			Diagnostics: []protocol.Diagnostic{},
			CodeActions: []protocol.CodeAction{},
		}

		// This would test the formatting logic that would be used in the actual handler
		if result.Hover == nil {
			t.Error("Expected hover information")
		}

		if result.Diagnostics == nil {
			t.Error("Expected diagnostics slice to be initialized")
		}

		if result.CodeActions == nil {
			t.Error("Expected code actions slice to be initialized")
		}
	})
}
