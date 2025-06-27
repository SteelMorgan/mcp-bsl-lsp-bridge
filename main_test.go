package main

import (
	"rockerboo/mcp-lsp-bridge/bridge"
	"rockerboo/mcp-lsp-bridge/lsp"
	"testing"
)

// createTestConfig creates a minimal test configuration
func createTestConfig() *lsp.LSPServerConfig {
	config, err := lsp.LoadLSPConfig("lsp_config.example.json")
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

	if len(config.LanguageServers) == 0 {
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

			if language != tc.expected {
				t.Errorf("Expected language %s, got %s", tc.expected, language)
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
