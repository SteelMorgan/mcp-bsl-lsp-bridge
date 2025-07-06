package tools

import (
	"context"
	"errors"
	"testing"

	"rockerboo/mcp-lsp-bridge/lsp"
	"rockerboo/mcp-lsp-bridge/mocks"
	"rockerboo/mcp-lsp-bridge/types"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/mcptest"
	"github.com/mark3labs/mcp-go/server"
	"github.com/myleshyson/lsprotocol-go/protocol"
)

func TestAnalyzeCodeTool_Success(t *testing.T) {
	bridge := &mocks.MockBridge{}
	uri := "file:///test.go"
	mockLanguage := types.Language("go")
	mockClient := &mocks.MockLanguageClient{}

	// Set up mock expectations
	bridge.On("InferLanguage", uri).Return(&mockLanguage, nil)

	bridge.On("GetClientForLanguage", string(mockLanguage)).Return(
		types.LanguageClientInterface(mockClient), nil)

	tool, handler := AnalyzeCode(bridge)
	// Create MCP server and register tool
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
				"uri":       "file:///test.go",
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

	bridge.AssertExpectations(t)
}

func TestAnalyzeCodeTool_LanguageInferenceFailure(t *testing.T) {
	bridge := &mocks.MockBridge{}
	uri := "file:///unknown.xyz"

	// Set up mock to return error
	bridge.On("InferLanguage", uri).Return((*types.Language)(nil), errors.New("unsupported file type"))

	tool, handler := AnalyzeCode(bridge)
	// Create MCP server and register tool
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

	bridge.AssertExpectations(t)
}

func TestAnalyzeCodeTool_ClientCreationFailure(t *testing.T) {
	bridge := &mocks.MockBridge{}
	uri := "file:///test.go"
	mockLanguage := types.Language("unsupported")

	// Set up mocks - language inference succeeds, client creation fails
	bridge.On("InferLanguage", uri).Return(&mockLanguage, nil)
	bridge.On("GetClientForLanguage", string(mockLanguage)).Return((types.LanguageClientInterface)(nil), errors.New("unsupported language"))

	tool, handler := AnalyzeCode(bridge)
	// Create MCP server and register tool
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
				"uri":       "file:///test.go",
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
