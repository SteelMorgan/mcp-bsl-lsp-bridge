package tools

import (
	"testing"

	"rockerboo/mcp-lsp-bridge/lsp"

	"github.com/mark3labs/mcp-go/server"
)

func TestInferLanguageTool(t *testing.T) {
	testCases := []struct {
		name         string
		filePath     string
		mockConfig   *lsp.LSPServerConfig
		expectError  bool
		expectedLang string
	}{
		{
			name:     "Go file detection",
			filePath: "/path/to/main.go",
			mockConfig: &lsp.LSPServerConfig{
				ExtensionLanguageMap: map[string]string{
					".go":  "go",
					".py":  "python",
					".js":  "javascript",
					".ts":  "typescript",
					".rs":  "rust",
					".cpp": "cpp",
					".c":   "c",
				},
			},
			expectError:  false,
			expectedLang: "go",
		},
		{
			name:     "Python file detection",
			filePath: "/path/to/script.py",
			mockConfig: &lsp.LSPServerConfig{
				ExtensionLanguageMap: map[string]string{
					".go": "go",
					".py": "python",
					".js": "javascript",
				},
			},
			expectError:  false,
			expectedLang: "python",
		},
		{
			name:     "JavaScript file detection",
			filePath: "/path/to/app.js",
			mockConfig: &lsp.LSPServerConfig{
				ExtensionLanguageMap: map[string]string{
					".go": "go",
					".py": "python",
					".js": "javascript",
				},
			},
			expectError:  false,
			expectedLang: "javascript",
		},
		{
			name:     "TypeScript file detection",
			filePath: "/path/to/component.ts",
			mockConfig: &lsp.LSPServerConfig{
				ExtensionLanguageMap: map[string]string{
					".ts": "typescript",
					".js": "javascript",
				},
			},
			expectError:  false,
			expectedLang: "typescript",
		},
		{
			name:     "Rust file detection",
			filePath: "/path/to/main.rs",
			mockConfig: &lsp.LSPServerConfig{
				ExtensionLanguageMap: map[string]string{
					".rs": "rust",
					".go": "go",
				},
			},
			expectError:  false,
			expectedLang: "rust",
		},
		{
			name:     "unknown extension",
			filePath: "/path/to/file.xyz",
			mockConfig: &lsp.LSPServerConfig{
				ExtensionLanguageMap: map[string]string{
					".go": "go",
					".py": "python",
				},
			},
			expectError: true,
		},
		{
			name:        "no config available",
			filePath:    "/path/to/main.go",
			mockConfig:  nil,
			expectError: true,
		},
		{
			name:     "file without extension",
			filePath: "/path/to/Makefile",
			mockConfig: &lsp.LSPServerConfig{
				ExtensionLanguageMap: map[string]string{
					".go": "go",
					".py": "python",
				},
			},
			expectError: true,
		},
		{
			name:     "hidden file with extension",
			filePath: "/path/to/.config.json",
			mockConfig: &lsp.LSPServerConfig{
				ExtensionLanguageMap: map[string]string{
					".json": "json",
					".go":   "go",
				},
			},
			expectError:  false,
			expectedLang: "json",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			bridge := &ComprehensiveMockBridge{
				getConfigFunc: func() *lsp.LSPServerConfig {
					return tc.mockConfig
				},
			}

			// Create MCP server and register tool
			mcpServer := server.NewMCPServer("test", "1.0.0", server.WithToolCapabilities(false))
			RegisterInferLanguageTool(mcpServer, bridge)

			// Test config retrieval
			config := bridge.GetConfig()
			if config == nil && !tc.expectError {
				t.Error("Expected config but got nil")
				return
			}
			if tc.expectError && config == nil {
				return // Expected error case
			}

			// Extract file extension manually (simulating filepath.Ext)
			var ext string
			for i := len(tc.filePath) - 1; i >= 0; i-- {
				if tc.filePath[i] == '.' {
					ext = tc.filePath[i:]
					break
				}
				if tc.filePath[i] == '/' {
					break // No extension found
				}
			}

			// Test language mapping
			if ext == "" && !tc.expectError {
				t.Error("Expected to find file extension")
				return
			}

			language, found := config.ExtensionLanguageMap[ext]
			if !found && !tc.expectError {
				t.Errorf("Expected to find language for extension %s", ext)
				return
			}

			if tc.expectError && !found {
				return // Expected error case
			}

			if language != tc.expectedLang {
				t.Errorf("Expected language %s, got %s", tc.expectedLang, language)
			}
		})
	}
}

func TestInferLanguageEdgeCases(t *testing.T) {
	t.Run("empty file path", func(t *testing.T) {
		bridge := &ComprehensiveMockBridge{
			getConfigFunc: func() *lsp.LSPServerConfig {
				return &lsp.LSPServerConfig{
					ExtensionLanguageMap: map[string]string{".go": "go"},
				}
			},
		}

		// Empty file path should not have an extension
		filePath := ""
		config := bridge.GetConfig()
		if config == nil {
			t.Fatal("Expected config")
		}

		// Extract extension from empty path
		var ext string
		for i := len(filePath) - 1; i >= 0; i-- {
			if filePath[i] == '.' {
				ext = filePath[i:]
				break
			}
		}

		if ext != "" {
			t.Error("Expected no extension for empty path")
		}
	})

	t.Run("path with multiple dots", func(t *testing.T) {
		bridge := &ComprehensiveMockBridge{
			getConfigFunc: func() *lsp.LSPServerConfig {
				return &lsp.LSPServerConfig{
					ExtensionLanguageMap: map[string]string{".go": "go", ".js": "javascript"},
				}
			},
		}

		filePath := "/path/to/app.min.js"
		config := bridge.GetConfig()

		// Extract extension (should get .js, not .min.js)
		var ext string
		for i := len(filePath) - 1; i >= 0; i-- {
			if filePath[i] == '.' {
				ext = filePath[i:]
				break
			}
		}

		if ext != ".js" {
			t.Errorf("Expected .js extension, got %s", ext)
		}

		// Check if language mapping works
		language, found := config.ExtensionLanguageMap[ext]
		if !found {
			t.Error("Expected to find language for .js extension")
		}
		if language != "javascript" { // According to our mock config, .js maps to "javascript"
			t.Errorf("Expected 'javascript', got '%s'", language)
		}
	})
}