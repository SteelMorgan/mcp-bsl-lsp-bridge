package main

import (
	"os"
	"path/filepath"
	"rockerboo/mcp-lsp-bridge/bridge"
	"rockerboo/mcp-lsp-bridge/lsp"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// createTestConfig creates a minimal test configuration
func createTestConfig() *lsp.LSPServerConfig {
	// Get current working directory for validation
	cwd, _ := os.Getwd()
	
	// Use an allowed directory for config
	allowedDirs := []string{cwd, "."}
	
	config, err := lsp.LoadLSPConfig("lsp_config.example.json", allowedDirs)
	if err != nil {
		// Fallback to a minimal config if file doesn't exist
		return &lsp.LSPServerConfig{
			LanguageServers: map[lsp.Language]lsp.LanguageServerConfig{
				"go": {
					Command:   "gopls",
					Args:      []string{},
					Languages: []string{"go"},
					Filetypes: []string{".go"},
				},
			},
			ExtensionLanguageMap: map[string]lsp.Language{
				".go": "go",
			},
			LanguageExtensionMap: map[lsp.Language][]string{
				"go": {".go"},
			},
		}
	}

	return config
}

func TestNewMCPLSPBridge(t *testing.T) {
	bridgeInstance := bridge.NewMCPLSPBridge(createTestConfig(), []string{})

	if bridgeInstance == nil {
		t.Fatal("NewMCPLSPBridge returned nil")
	}

	config := bridgeInstance.GetConfig()
	if config == nil {
		t.Fatal("Bridge configuration not loaded")
	}

	if len(config.GetLanguageServers()) == 0 {
		t.Fatal("No language servers configured")
	}
}

func TestInferLanguage(t *testing.T) {
	bridgeInstance := bridge.NewMCPLSPBridge(createTestConfig(), []string{})

	testCases := []struct {
		filePath   string
		expected   lsp.Language
		shouldFail bool
	}{
		{"/path/to/example.go", "go", false},
		{"/project/src/main.py", "python", false},
		{"/code/script.ts", "typescript", false},
		{"/repo/lib.rs", "rust", false},
		{"/unknown/file.txt", "", true},
	}

	for _, tc := range testCases {
		t.Run(tc.filePath, func(t *testing.T) {
			language, err := bridgeInstance.InferLanguage(tc.filePath)

			if tc.shouldFail {
				if err == nil {
					t.Errorf("Expected error for file %s", tc.filePath)
				}

				return
			}

			if err != nil {
				t.Errorf("Unexpected error for file %s: %v", tc.filePath, err)
				return
			}

			if *language != tc.expected {
				t.Errorf("Expected language %s, got %s", tc.expected, *language)
			}
		})
	}
}

// Benchmark client creation
func BenchmarkGetClientForLanguage(b *testing.B) {
	bridgeInstance := bridge.NewMCPLSPBridge(createTestConfig(), []string{})
	languages := []string{"go"}

	for i := 0; b.Loop(); i++ {
		language := languages[i%len(languages)]

		_, err := bridgeInstance.GetClientForLanguage(language)
		if err != nil {
			b.Fatalf("Failed to get client for language %s: %v", language, err)
		}
	}
}

func TestTryLoadConfig(t *testing.T) {
	tests := []struct {
		name          string
		setupFunc     func(t *testing.T) (primaryPath, configDir string, cleanup func())
		expectSuccess bool
		expectError   string
	}{
		{
			name: "load from primary path - success",
			setupFunc: func(t *testing.T) (string, string, func()) {
				tempDir := t.TempDir()
				primaryPath := filepath.Join(tempDir, "test_config.json")
				configDir := filepath.Join(tempDir, "config")
				
				// Create a valid config file
				configContent := `{
					"language_servers": {
						"go": {
							"command": "gopls",
							"args": [],
							"languages": ["go"],
							"filetypes": [".go"]
						}
					},
					"extension_language_map": {
						".go": "go"
					},
					"language_extension_map": {
						"go": [".go"]
					}
				}`
				
				err := os.WriteFile(primaryPath, []byte(configContent), 0600)
				require.NoError(t, err)
				
				return primaryPath, configDir, func() {}
			},
			expectSuccess: true,
		},
		{
			name: "primary path fails, load from current directory fallback",
			setupFunc: func(t *testing.T) (string, string, func()) {
				tempDir := t.TempDir()
				originalWd, _ := os.Getwd()
				
				// Change to temp directory
				err := os.Chdir(tempDir)
				require.NoError(t, err)
				
				primaryPath := filepath.Join(tempDir, "nonexistent.json")
				configDir := filepath.Join(tempDir, "config")
				
				// Create fallback config in current directory
				configContent := `{
					"language_servers": {
						"python": {
							"command": "pyright-langserver",
							"args": ["--stdio"],
							"languages": ["python"],
							"filetypes": [".py"]
						}
					},
					"extension_language_map": {
						".py": "python"
					}
				}`
				
				fallbackPath := "lsp_config.json"
				err = os.WriteFile(fallbackPath, []byte(configContent), 0600)
				require.NoError(t, err)
				
				return primaryPath, configDir, func() {
					os.Chdir(originalWd)
				}
			},
			expectSuccess: true,
		},
		{
			name: "primary path fails, load from config dir fallback",
			setupFunc: func(t *testing.T) (string, string, func()) {
				tempDir := t.TempDir()
				primaryPath := filepath.Join(tempDir, "nonexistent.json")
				configDir := filepath.Join(tempDir, "config")
				
				// Create config directory
				err := os.MkdirAll(configDir, 0755)
				require.NoError(t, err)
				
				// Create fallback config in config directory
				configContent := `{
					"language_servers": {
						"typescript": {
							"command": "typescript-language-server",
							"args": ["--stdio"],
							"languages": ["typescript"],
							"filetypes": [".ts", ".tsx"]
						}
					},
					"extension_language_map": {
						".ts": "typescript",
						".tsx": "typescript"
					}
				}`
				
				fallbackPath := filepath.Join(configDir, "config.json")
				err = os.WriteFile(fallbackPath, []byte(configContent), 0600)
				require.NoError(t, err)
				
				return primaryPath, configDir, func() {}
			},
			expectSuccess: true,
		},
		{
			name: "primary path fails, load from example config fallback",
			setupFunc: func(t *testing.T) (string, string, func()) {
				tempDir := t.TempDir()
				originalWd, _ := os.Getwd()
				
				// Change to temp directory
				err := os.Chdir(tempDir)
				require.NoError(t, err)
				
				primaryPath := filepath.Join(tempDir, "nonexistent.json")
				configDir := filepath.Join(tempDir, "config")
				
				// Create example config in current directory
				configContent := `{
					"language_servers": {
						"rust": {
							"command": "rust-analyzer",
							"args": [],
							"languages": ["rust"],
							"filetypes": [".rs"]
						}
					},
					"extension_language_map": {
						".rs": "rust"
					}
				}`
				
				fallbackPath := "example.lsp_config.json"
				err = os.WriteFile(fallbackPath, []byte(configContent), 0600)
				require.NoError(t, err)
				
				return primaryPath, configDir, func() {
					os.Chdir(originalWd)
				}
			},
			expectSuccess: true,
		},
		{
			name: "all paths fail - no valid configuration",
			setupFunc: func(t *testing.T) (string, string, func()) {
				tempDir := t.TempDir()
				originalWd, _ := os.Getwd()
				
				// Change to temp directory where no config files exist
				err := os.Chdir(tempDir)
				require.NoError(t, err)
				
				primaryPath := filepath.Join(tempDir, "nonexistent.json")
				configDir := filepath.Join(tempDir, "config")
				
				// Don't create any config files
				return primaryPath, configDir, func() {
					os.Chdir(originalWd)
				}
			},
			expectSuccess: false,
			expectError:   "no valid configuration found",
		},
		{
			name: "primary path same as fallback - avoid duplicate attempt",
			setupFunc: func(t *testing.T) (string, string, func()) {
				tempDir := t.TempDir()
				originalWd, _ := os.Getwd()
				
				// Change to temp directory
				err := os.Chdir(tempDir)
				require.NoError(t, err)
				
				// Use "lsp_config.json" as primary path (same as fallback)
				primaryPath := "lsp_config.json"
				configDir := filepath.Join(tempDir, "config")
				
				// Create config with primary path name
				configContent := `{
					"language_servers": {
						"javascript": {
							"command": "typescript-language-server",
							"args": ["--stdio"],
							"languages": ["javascript"],
							"filetypes": [".js", ".jsx"]
						}
					},
					"extension_language_map": {
						".js": "javascript",
						".jsx": "javascript"
					}
				}`
				
				err = os.WriteFile(primaryPath, []byte(configContent), 0600)
				require.NoError(t, err)
				
				return primaryPath, configDir, func() {
					os.Chdir(originalWd)
				}
			},
			expectSuccess: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			primaryPath, configDir, cleanup := tt.setupFunc(t)
			defer cleanup()

			config, err := tryLoadConfig(primaryPath, configDir)

			if tt.expectSuccess {
				assert.NoError(t, err)
				assert.NotNil(t, config)
				if config != nil {
					assert.NotEmpty(t, config.LanguageServers)
				}
			} else {
				assert.Error(t, err)
				assert.Nil(t, config)
				if tt.expectError != "" && err != nil {
					assert.Contains(t, err.Error(), tt.expectError)
				}
			}
		})
	}
}
