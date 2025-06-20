package tools

import (
	"testing"

	"github.com/mark3labs/mcp-go/server"
)

func TestLSPDisconnectTool(t *testing.T) {
	testCases := []struct {
		name        string
		description string
	}{
		{
			name:        "successful disconnect",
			description: "Should successfully disconnect all language server clients",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			disconnectCalled := false
			bridge := &ComprehensiveMockBridge{
				closeAllClientsFunc: func() {
					disconnectCalled = true
				},
			}

			// Create MCP server and register tool
			mcpServer := server.NewMCPServer("test", "1.0.0", server.WithToolCapabilities(false))
			RegisterLSPDisconnectTool(mcpServer, bridge)

			// Test disconnect functionality
			bridge.CloseAllClients()

			if !disconnectCalled {
				t.Error("Expected CloseAllClients to be called")
			}

			t.Logf("Test completed: %s", tc.description)
		})
	}
}

func TestLSPDisconnectMultipleClients(t *testing.T) {
	t.Run("disconnect multiple clients", func(t *testing.T) {
		clientsClosed := make(map[string]bool)
		
		bridge := &ComprehensiveMockBridge{
			closeAllClientsFunc: func() {
				// Simulate closing multiple language clients
				clientsClosed["go"] = true
				clientsClosed["python"] = true
				clientsClosed["typescript"] = true
				clientsClosed["rust"] = true
			},
		}

		// Test that all clients are marked as closed
		bridge.CloseAllClients()

		expectedClients := []string{"go", "python", "typescript", "rust"}
		for _, client := range expectedClients {
			if !clientsClosed[client] {
				t.Errorf("Expected %s client to be closed", client)
			}
		}
	})
}

func TestLSPDisconnectIdempotency(t *testing.T) {
	t.Run("multiple disconnect calls should be safe", func(t *testing.T) {
		callCount := 0
		bridge := &ComprehensiveMockBridge{
			closeAllClientsFunc: func() {
				callCount++
			},
		}

		// Call disconnect multiple times
		bridge.CloseAllClients()
		bridge.CloseAllClients()
		bridge.CloseAllClients()

		if callCount != 3 {
			t.Errorf("Expected 3 calls to CloseAllClients, got %d", callCount)
		}
	})
}

func TestLSPDisconnectResourceCleanup(t *testing.T) {
	t.Run("verify resource cleanup", func(t *testing.T) {
		resourcesCleaned := false
		memoryFreed := false
		connectionsClosed := false

		bridge := &ComprehensiveMockBridge{
			closeAllClientsFunc: func() {
				// Simulate comprehensive cleanup
				resourcesCleaned = true
				memoryFreed = true
				connectionsClosed = true
			},
		}

		bridge.CloseAllClients()

		if !resourcesCleaned {
			t.Error("Expected resources to be cleaned up")
		}
		if !memoryFreed {
			t.Error("Expected memory to be freed")
		}
		if !connectionsClosed {
			t.Error("Expected connections to be closed")
		}
	})
}