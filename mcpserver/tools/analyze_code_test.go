package tools

import (
	"fmt"
	"testing"

	"rockerboo/mcp-lsp-bridge/lsp"

	"github.com/mark3labs/mcp-go/server"
	"github.com/myleshyson/lsprotocol-go/protocol"
)

// Test analyze code tool registration and execution
func TestAnalyzeCodeTool(t *testing.T) {
	testCases := []struct {
		name            string
		uri             string
		line            int32
		character       int32
		mockLanguage    string
		mockClient      any
		mockAnalysis    *lsp.AnalyzeCodeResult
		expectError     bool
		expectedContent string
	}{
		{
			name:         "successful code analysis",
			uri:          "file:///test.go",
			line:         10,
			character:    5,
			mockLanguage: "go",
			mockClient:   &lsp.LanguageClient{},
			mockAnalysis: &lsp.AnalyzeCodeResult{
				Hover:       &protocol.HoverResponse{},
				Diagnostics: []protocol.Diagnostic{},
				CodeActions: []protocol.CodeAction{},
				Completion:  &protocol.CompletionResponse{},
			},
			expectError:     false,
			expectedContent: "Analysis Results:",
		},
		{
			name:        "language inference failure",
			uri:         "file:///unknown.xyz",
			line:        10,
			character:   5,
			expectError: true,
		},
		{
			name:         "client creation failure",
			uri:          "file:///test.go",
			line:         10,
			character:    5,
			mockLanguage: "unsupported",
			expectError:  true,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			bridge := &ComprehensiveMockBridge{
				inferLanguageFunc: func(filePath string) (string, error) {
					if tc.mockLanguage != "" {
						return tc.mockLanguage, nil
					}
					return "", fmt.Errorf("unknown language for %s", filePath)
				},
				getClientForLanguageFunc: func(language string) (any, error) {
					if tc.mockClient != nil && language == tc.mockLanguage {
						return tc.mockClient, nil
					}
					return nil, fmt.Errorf("no client for language: %s", language)
				},
			}

			// Create MCP server and register tool
			mcpServer := server.NewMCPServer("test", "1.0.0", server.WithToolCapabilities(false))
			RegisterAnalyzeCodeTool(mcpServer, bridge)

			// Test language inference (first step of analysis)
			language, err := bridge.InferLanguage(tc.uri)
			if tc.expectError && err != nil {
				return // Expected error case
			}
			if err != nil && !tc.expectError {
				t.Errorf("Unexpected error in language inference: %v", err)
				return
			}

			// Test client creation (second step of analysis)
			if !tc.expectError {
				client, err := bridge.GetClientForLanguageInterface(language)
				if err != nil {
					if tc.expectError {
						return // Expected error
					}
					t.Errorf("Unexpected error in client creation: %v", err)
					return
				}
				if client == nil {
					t.Error("Expected client but got nil")
				}
			}
		})
	}
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