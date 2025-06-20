package tools

import (
	"fmt"
	"testing"

	"rockerboo/mcp-lsp-bridge/lsp"

	"github.com/mark3labs/mcp-go/server"
)

func TestLSPConnectTool(t *testing.T) {
	testCases := []struct {
		name         string
		language     string
		mockConfig   *lsp.LSPServerConfig
		mockClient   any
		expectError  bool
		description  string
	}{
		{
			name:     "successful Go connection",
			language: "go",
			mockConfig: &lsp.LSPServerConfig{
				LanguageServers: map[string]lsp.LanguageServerConfig{
					"go": {
						Command:   "gopls",
						Args:      []string{"serve"},
						Filetypes: []string{".go"},
					},
					"python": {
						Command:   "pyright-langserver",
						Args:      []string{"--stdio"},
						Filetypes: []string{".py"},
					},
				},
			},
			mockClient:  &lsp.LanguageClient{},
			expectError: false,
			description: "Should successfully connect to Go language server",
		},
		{
			name:     "successful Python connection",
			language: "python",
			mockConfig: &lsp.LSPServerConfig{
				LanguageServers: map[string]lsp.LanguageServerConfig{
					"go": {
						Command:   "gopls",
						Filetypes: []string{".go"},
					},
					"python": {
						Command:   "pyright-langserver",
						Args:      []string{"--stdio"},
						Filetypes: []string{".py"},
					},
				},
			},
			mockClient:  &lsp.LanguageClient{},
			expectError: false,
			description: "Should successfully connect to Python language server",
		},
		{
			name:     "successful TypeScript connection",
			language: "typescript",
			mockConfig: &lsp.LSPServerConfig{
				LanguageServers: map[string]lsp.LanguageServerConfig{
					"typescript": {
						Command:   "typescript-language-server",
						Args:      []string{"--stdio"},
						Filetypes: []string{".ts", ".tsx"},
					},
				},
			},
			mockClient:  &lsp.LanguageClient{},
			expectError: false,
			description: "Should successfully connect to TypeScript language server",
		},
		{
			name:     "successful Rust connection",
			language: "rust",
			mockConfig: &lsp.LSPServerConfig{
				LanguageServers: map[string]lsp.LanguageServerConfig{
					"rust": {
						Command:   "rust-analyzer",
						Filetypes: []string{".rs"},
					},
				},
			},
			mockClient:  &lsp.LanguageClient{},
			expectError: false,
			description: "Should successfully connect to Rust language server",
		},
		{
			name:     "language not configured",
			language: "unsupported",
			mockConfig: &lsp.LSPServerConfig{
				LanguageServers: map[string]lsp.LanguageServerConfig{
					"go": {
						Command:   "gopls",
						Filetypes: []string{".go"},
					},
				},
			},
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
			name:     "client creation failure",
			language: "go",
			mockConfig: &lsp.LSPServerConfig{
				LanguageServers: map[string]lsp.LanguageServerConfig{
					"go": {
						Command:   "gopls",
						Filetypes: []string{".go"},
					},
				},
			},
			mockClient:  nil, // Simulate client creation failure
			expectError: true,
			description: "Should handle client creation failures gracefully",
		},
		{
			name:     "empty language string",
			language: "",
			mockConfig: &lsp.LSPServerConfig{
				LanguageServers: map[string]lsp.LanguageServerConfig{
					"go": {Command: "gopls"},
				},
			},
			expectError: true,
			description: "Should fail with empty language string",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			bridge := &ComprehensiveMockBridge{
				getConfigFunc: func() *lsp.LSPServerConfig {
					return tc.mockConfig
				},
				getClientForLanguageFunc: func(language string) (any, error) {
					if tc.mockClient != nil && language == tc.language {
						return tc.mockClient, nil
					}
					return nil, fmt.Errorf("failed to create client for language: %s", language)
				},
			}

			// Create MCP server and register tool
			mcpServer := server.NewMCPServer("test", "1.0.0", server.WithToolCapabilities(false))
			RegisterLSPConnectTool(mcpServer, bridge)

			// Test configuration retrieval
			config := bridge.GetConfig()
			if config == nil {
				if !tc.expectError {
					t.Error("Expected config but got nil")
				}
				return
			}

			// Test language server configuration existence
			_, exists := config.LanguageServers[tc.language]
			if !exists {
				if !tc.expectError {
					t.Errorf("Expected language server config for %s", tc.language)
				}
				return
			}

			// Test client creation
			client, err := bridge.GetClientForLanguageInterface(tc.language)
			if tc.expectError {
				if err == nil {
					t.Error("Expected error but got none")
				}
			} else {
				if err != nil {
					t.Errorf("Unexpected error: %v", err)
					return
				}
				if client == nil {
					t.Error("Expected client but got nil")
				}
			}

			t.Logf("Test completed: %s", tc.description)
		})
	}
}

func TestLSPConnectValidation(t *testing.T) {
	t.Run("validate language server configuration", func(t *testing.T) {
		config := &lsp.LSPServerConfig{
			LanguageServers: map[string]lsp.LanguageServerConfig{
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
			LanguageServers: map[string]lsp.LanguageServerConfig{
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
			serverConfig, exists := config.LanguageServers[lang]
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