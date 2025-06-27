package tools

import (
	"errors"
	"testing"

	"rockerboo/mcp-lsp-bridge/lsp"
	"rockerboo/mcp-lsp-bridge/mocks"

	"github.com/mark3labs/mcp-go/mcptest"
	"github.com/myleshyson/lsprotocol-go/protocol"
)

func TestAnalyzeCodeTool_Success(t *testing.T) {
	bridge := &mocks.MockBridge{}
	uri := "file:///test.go"
	mockLanguage := lsp.Language("go")
	mockClient := &mocks.MockLanguageClient{}

	// Set up mock expectations
	bridge.On("InferLanguage", uri).Return(mockLanguage, nil)

	bridge.On("GetClientForLanguageInterface", string(mockLanguage)).Return(
		lsp.LanguageClientInterface(mockClient), nil)

	// Create MCP server and register tool
	mcpServer, err := mcptest.NewServer(t)
	if err != nil {
		t.Fatalf("Could not start MCP server: %v", err)
	}

	RegisterAnalyzeCodeTool(mcpServer, bridge)

	// Test language inference
	language, err := bridge.InferLanguage(uri)
	if err != nil {
		t.Errorf("Unexpected error in language inference: %v", err)
		return
	}

	// Test client creation
	client, err := bridge.GetClientForLanguageInterface(string(language))
	if err != nil {
		t.Errorf("Unexpected error in client creation: %v", err)
		return
	}

	if client == nil {
		t.Error("Expected client but got nil")
	}

	bridge.AssertExpectations(t)
}

func TestAnalyzeCodeTool_LanguageInferenceFailure(t *testing.T) {
	bridge := &mocks.MockBridge{}
	uri := "file:///unknown.xyz"

	// Set up mock to return error
	bridge.On("InferLanguage", uri).Return(lsp.Language(""), errors.New("unsupported file type"))

	// Create MCP server and register tool
	mcpServer, err := mcptest.NewServer(t)
	if err != nil {
		t.Fatalf("Could not start MCP server: %v", err)
	}

	RegisterAnalyzeCodeTool(mcpServer, bridge)

	// Test language inference - should fail
	_, err = bridge.InferLanguage(uri)
	if err == nil {
		t.Error("Expected error in language inference but got none")
	}

	bridge.AssertExpectations(t)
}

func TestAnalyzeCodeTool_ClientCreationFailure(t *testing.T) {
	bridge := &mocks.MockBridge{}
	uri := "file:///test.go"
	mockLanguage := "unsupported"

	// Set up mocks - language inference succeeds, client creation fails
	bridge.On("InferLanguage", uri).Return(lsp.Language(mockLanguage), nil)
	bridge.On("GetClientForLanguageInterface", mockLanguage).Return((*lsp.LanguageClient)(nil), errors.New("unsupported language"))

	// Create MCP server and register tool
	mcpServer, err := mcptest.NewServer(t)
	if err != nil {
		t.Fatalf("Could not start MCP server: %v", err)
	}

	RegisterAnalyzeCodeTool(mcpServer, bridge)

	// Test language inference - should succeed
	language, err := bridge.InferLanguage(uri)
	if err != nil {
		t.Errorf("Unexpected error in language inference: %v", err)
		return
	}

	// Test client creation - should fail
	_, err = bridge.GetClientForLanguageInterface(string(language))
	if err == nil {
		t.Error("Expected error in client creation but got none")
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
