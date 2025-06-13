package main

import (
	"rockerboo/mcp-lsp-bridge/bridge"
	"testing"
)

func TestNewMCPLSPBridge(t *testing.T) {
	bridgeInstance := bridge.NewMCPLSPBridge()

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
	bridgeInstance := bridge.NewMCPLSPBridge()

	testCases := []struct {
		filePath   string
		expected   string
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

func TestGetClientForLanguage(t *testing.T) {
	bridgeInstance := bridge.NewMCPLSPBridge()

	testCases := []struct {
		language string
	}{
		{"go"},
		{"python"},
		{"typescript"},
		{"rust"},
	}

	for _, tc := range testCases {
		t.Run(tc.language, func(t *testing.T) {
			// Get or create the client
			client, err := bridgeInstance.GetClientForLanguage(tc.language)
			if err != nil {
				t.Fatalf("Failed to get client for language %s: %v", tc.language, err)
			}

			if client == nil {
				t.Fatalf("Client for language %s is nil", tc.language)
			}

			// Test the second call returns the same client (cached)
			client2, err := bridgeInstance.GetClientForLanguage(tc.language)
			if err != nil {
				t.Fatalf("Failed to get client for language %s on second call: %v", tc.language, err)
			}

			if client != client2 {
				t.Errorf("Second call to GetClientForLanguage should return the same client instance")
			}
		})
	}
}

func TestCloseAllClients(t *testing.T) {
	bridgeInstance := bridge.NewMCPLSPBridge()

	// Create clients for multiple languages
	languages := []string{"go", "python", "typescript", "rust"}
	for _, language := range languages {
		_, err := bridgeInstance.GetClientForLanguage(language)
		if err != nil {
			t.Fatalf("Failed to get client for language %s: %v", language, err)
		}
	}

	// Close all clients
	bridgeInstance.CloseAllClients()

	// Verify that we can create new clients after closing (tests that cleanup worked)
	for _, language := range languages {
		_, err := bridgeInstance.GetClientForLanguage(language)
		if err != nil {
			t.Fatalf("Failed to recreate client for language %s after CloseAllClients: %v", language, err)
		}
	}
}

// Benchmark client creation
func BenchmarkGetClientForLanguage(b *testing.B) {
	bridgeInstance := bridge.NewMCPLSPBridge()
	languages := []string{"go", "python", "typescript", "rust"}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		language := languages[i%len(languages)]

		_, err := bridgeInstance.GetClientForLanguage(language)
		if err != nil {
			b.Fatalf("Failed to get client for language %s: %v", language, err)
		}
	}
}
