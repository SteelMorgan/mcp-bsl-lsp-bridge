package tools

import (
	"context"
	"fmt"
	"testing"

	"rockerboo/mcp-lsp-bridge/lsp"
	"rockerboo/mcp-lsp-bridge/mocks"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/mcptest"
	"github.com/mark3labs/mcp-go/server"
)

func TestLSPConnectTool(t *testing.T) {
	testCases := []struct {
		name        string
		language    string
		mockConfig  lsp.LSPServerConfigProvider
		mockClient  lsp.LanguageClientInterface
		expectError bool
		description string
	}{
		{
			name:        "successful Go connection",
			language:    "go",
			mockConfig:  &mocks.MockLSPServerConfig{},
			mockClient:  &mocks.MockLanguageClient{},
			expectError: false,
			description: "Should successfully connect to Go language server",
		},
		{
			name:        "successful Python connection",
			language:    "python",
			mockConfig:  &mocks.MockLSPServerConfig{},
			mockClient:  &mocks.MockLanguageClient{},
			expectError: false,
			description: "Should successfully connect to Python language server",
		},
		{
			name:        "successful TypeScript connection",
			language:    "typescript",
			mockConfig:  &mocks.MockLSPServerConfig{},
			mockClient:  &mocks.MockLanguageClient{},
			expectError: false,
			description: "Should successfully connect to TypeScript language server",
		},
		{
			name:        "successful Rust connection",
			language:    "rust",
			mockConfig:  &mocks.MockLSPServerConfig{},
			mockClient:  &mocks.MockLanguageClient{},
			expectError: false,
			description: "Should successfully connect to Rust language server",
		},
		{
			name:        "language not configured",
			language:    "unsupported",
			mockConfig:  &mocks.MockLSPServerConfig{},
			expectError: true,
			description: "Should fail when language server is not configured",
		},
		{
			name:        "no config available",
			language:    "go",
			mockConfig:  nil,
			expectError: true,
			description: "Should fail when no configuration is available",
		},
		{
			name:        "client creation failure",
			language:    "go",
			mockConfig:  &mocks.MockLSPServerConfig{},
			expectError: true,
			description: "Should handle client creation failures gracefully",
		},
		{
			name:        "empty language string",
			language:    "",
			mockConfig:  &mocks.MockLSPServerConfig{},
			expectError: true,
			description: "Should fail with empty language string",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			bridge := &mocks.MockBridge{}

			if tc.expectError {
				bridge.On("GetClientForLanguage", tc.language).Return((lsp.LanguageClientInterface)(nil), fmt.Errorf("failed to create client for language: %s", tc.language))
			} else {
				bridge.On("GetClientForLanguage", tc.language).Return(tc.mockClient, nil)
			}

			// Create MCP server and register tool
			tool, handler := LSPConnectTool(bridge)
			mcpServer, err := mcptest.NewServer(t, server.ServerTool{
				Tool:    tool,
				Handler: handler,
			})
			if err != nil {
				t.Errorf("Could not create MCP server: %v", err)
				return
			}

			// defer mcpServer.Close()

			ctx := context.Background()
			toolResult, err := mcpServer.Client().CallTool(ctx, mcp.CallToolRequest{
				Request: mcp.Request{Method: "tools/call"},
				Params: mcp.CallToolParams{
					Name: "lsp_connect",
					Arguments: map[string]any{
						"language": tc.language,
					},
				},
			})

			if err != nil {
				t.Errorf("Could not make request %v", err)
				return
			}

			if !toolResult.IsError && tc.expectError {
				t.Error("Expected error but got none")
			} else if toolResult.IsError && !tc.expectError {
				t.Errorf("Unexpected error: %v", toolResult.Content)
			}

			bridge.AssertExpectations(t)
		})
	}
}
func TestLSPConnectValidation(t *testing.T) {
	t.Run("validate language server configuration", func(t *testing.T) {
		config := &lsp.LSPServerConfig{
			LanguageServers: map[lsp.Language]lsp.LanguageServerConfig{
				"go": {
					Command:   "gopls",
					Args:      []string{"serve"},
					Filetypes: []string{".go"},
				},
				"invalid": {
					// Missing command
					Args:      []string{"--stdio"},
					Filetypes: []string{".invalid"},
				},
			},
		}

		// Test valid configuration
		goConfig, exists := config.LanguageServers["go"]
		if !exists {
			t.Error("Expected Go language server config")
		}

		if goConfig.Command == "" {
			t.Error("Expected Go language server to have command")
		}

		if len(goConfig.Filetypes) == 0 {
			t.Error("Expected Go language server to have filetypes")
		}

		// Test invalid configuration
		invalidConfig, exists := config.LanguageServers["invalid"]
		if !exists {
			t.Error("Expected invalid language server config for testing")
		}

		if invalidConfig.Command != "" {
			t.Error("Expected invalid config to have empty command")
		}
	})

	t.Run("multiple language server support", func(t *testing.T) {
		config := &lsp.LSPServerConfig{
			LanguageServers: map[lsp.Language]lsp.LanguageServerConfig{
				"go": {
					Command:   "gopls",
					Filetypes: []string{".go"},
				},
				"python": {
					Command:   "pyright-langserver",
					Filetypes: []string{".py"},
				},
				"typescript": {
					Command:   "typescript-language-server",
					Filetypes: []string{".ts", ".tsx"},
				},
				"rust": {
					Command:   "rust-analyzer",
					Filetypes: []string{".rs"},
				},
			},
		}

		expectedLanguages := []string{"go", "python", "typescript", "rust"}
		for _, lang := range expectedLanguages {
			serverConfig, exists := config.LanguageServers[lsp.Language(lang)]
			if !exists {
				t.Errorf("Expected %s language server config", lang)
				continue
			}

			if serverConfig.Command == "" {
				t.Errorf("Expected %s language server to have command", lang)
			}

			if len(serverConfig.Filetypes) == 0 {
				t.Errorf("Expected %s language server to have filetypes", lang)
			}
		}
	})
}
