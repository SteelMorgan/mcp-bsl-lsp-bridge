package main

import (
	"testing"
)

func TestNewMCPLSPBridge(t *testing.T) {
	bridge := NewMCPLSPBridge()

	if bridge == nil {
		t.Fatal("NewMCPLSPBridge returned nil")
	}

	if bridge.config == nil {
		t.Fatal("Bridge configuration not loaded")
	}

	if len(bridge.config.LanguageServers) == 0 {
		t.Fatal("No language servers configured")
	}
}

func TestInferLanguage(t *testing.T) {
	bridge := NewMCPLSPBridge()

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
			language, err := bridge.InferLanguage(tc.filePath)

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
	bridge := NewMCPLSPBridge()

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
			client, err := bridge.GetClientForLanguage(tc.language)
			if err != nil {
				t.Fatalf("Failed to get client for language %s: %v", tc.language, err)
			}

			if client == nil {
				t.Fatalf("Client for language %s is nil", tc.language)
			}

			// Verify the client is stored in the clients map
			storedClient, exists := bridge.clients[tc.language]
			if !exists {
				t.Errorf("Client for language %s not stored in clients map", tc.language)
			}

			if storedClient != client {
				t.Errorf("Stored client does not match retrieved client for language %s", tc.language)
			}
		})
	}
}

func TestCloseAllClients(t *testing.T) {
	bridge := NewMCPLSPBridge()

	// Create clients for multiple languages
	languages := []string{"go", "python", "typescript", "rust"}
	for _, language := range languages {
		_, err := bridge.GetClientForLanguage(language)
		if err != nil {
			t.Fatalf("Failed to get client for language %s: %v", language, err)
		}
	}

	// Verify clients were created
	if len(bridge.clients) != len(languages) {
		t.Errorf("Expected %d clients, got %d", len(languages), len(bridge.clients))
	}

	// Close all clients
	bridge.CloseAllClients()

	// Verify clients are closed and cleared
	if len(bridge.clients) != 0 {
		t.Errorf("Clients map not emptied after CloseAllClients")
	}
}

// Benchmark client creation
func BenchmarkGetClientForLanguage(b *testing.B) {
	bridge := NewMCPLSPBridge()
	languages := []string{"go", "python", "typescript", "rust"}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		language := languages[i%len(languages)]
		
		_, err := bridge.GetClientForLanguage(language)
		if err != nil {
			b.Fatalf("Failed to get client for language %s: %v", language, err)
		}
	}
}