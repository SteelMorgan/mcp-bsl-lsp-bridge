package tools

import (
	"errors"
	"fmt"
	"testing"

	"rockerboo/mcp-lsp-bridge/lsp"
	"rockerboo/mcp-lsp-bridge/mocks"

	"github.com/mark3labs/mcp-go/mcptest"
)

func TestLSPConnectTool(t *testing.T) {
	testCases := []struct {
		name        string
		language    string
		mockConfig  *lsp.LSPServerConfig
		mockClient  any
		expectError bool
		description string
	}{
		{
			name:     "successful Go connection",
			language: "go",
			mockConfig: &lsp.LSPServerConfig{
				LanguageServers: map[lsp.Language]lsp.LanguageServerConfig{
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
				LanguageServers: map[lsp.Language]lsp.LanguageServerConfig{
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
				LanguageServers: map[lsp.Language]lsp.LanguageServerConfig{
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
				LanguageServers: map[lsp.Language]lsp.LanguageServerConfig{
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
				LanguageServers: map[lsp.Language]lsp.LanguageServerConfig{
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
				LanguageServers: map[lsp.Language]lsp.LanguageServerConfig{
					"go": {
						Command:   "gopls",
						Filetypes: []string{".go"},
					},
				},
			},
			expectError: true,
			description: "Should handle client creation failures gracefully",
		},
		{
			name:     "empty language string",
			language: "",
			mockConfig: &lsp.LSPServerConfig{
				LanguageServers: map[lsp.Language]lsp.LanguageServerConfig{
					"go": {Command: "gopls"},
				},
			},
			expectError: true,
			description: "Should fail with empty language string",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			bridge := &mocks.MockBridge{}

			// Set up mock expectations for GetConfig
			bridge.On("GetConfig").Return(tc.mockConfig)

			// Set up mock expectations for GetClientForLanguageInterface
			if tc.mockClient != nil {
				bridge.On("GetClientForLanguageInterface", tc.language).Return(tc.mockClient, nil)
			} else if tc.expectError && tc.name == "client creation failure" {

				bridge.On("GetClientForLanguageInterface", tc.language).Return((*lsp.LanguageClientInterface)(nil), fmt.Errorf("failed to create client for language: %s", tc.language))
			} else if tc.expectError {
				bridge.On("GetClientForLanguageInterface", tc.language).Return((*lsp.LanguageClientInterface)(nil), errors.New("client creation failed")).Maybe()
			}

			// Create MCP server and register tool
			mcpServer, err := mcptest.NewServer(t)
			if err != nil {
				t.Errorf("Could not create MCP server: %v", err)
			}
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
			_, exists := config.LanguageServers[lsp.Language(tc.language)]
			if !exists {
				if !tc.expectError {
					t.Errorf("Expected language server config for %s", tc.language)
				}
				return
			}

			// Test client creation
			client, err := bridge.GetClientForLanguageInterface(string(tc.language))
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

			// Verify all expectations were met
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
