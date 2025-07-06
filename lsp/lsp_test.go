package lsp

import (
	"rockerboo/mcp-lsp-bridge/types"
	"testing"
	"time"

	"github.com/myleshyson/lsprotocol-go/protocol"
)

func TestNewLanguageClient(t *testing.T) {
	client, err := NewLanguageClient("mock-lsp-server")
	if err != nil {
		t.Fatalf("Failed to create language client: %v", err)
	}

	_, err = client.Connect()
	if err != nil {
		t.Fatalf("Failed to connect to language client: %v", err)
	}

	defer closeClient(t, client)

	// Check basic initialization
	if client == nil {
		t.Fatal("NewLanguageClient returned nil")
	}

	if !client.IsConnected() {
		t.Error("Client should be marked as connected")
	}
}

func TestLanguageClientMetrics(t *testing.T) {
	client, err := NewLanguageClient("mock-lsp-server")
	if err != nil {
		t.Fatalf("Failed to create language client: %v", err)
	}

	_, err = client.Connect()
	if err != nil {
		t.Fatalf("Failed to connect to language client: %v", err)
	}

	defer closeClient(t, client)

	// Perform some requests to generate metrics
	for range 5 {
		err := client.SendRequest(
			"test/method",
			map[string]any{"key": "value"},
			&map[string]any{},
			1*time.Second,
		)
		// Ignore errors since we're using echo
		_ = err
	}

	// Retrieve and check metrics
	metrics := client.GetMetrics()

	// Verify initial metric properties
	if metrics.GetCommand() != "mock-lsp-server" {
		t.Errorf("Unexpected command: %v", metrics.GetCommand())
	}

	if metrics.GetTotalRequests() != 5 {
		t.Errorf("Expected 5 total requests initially, got %v", metrics.GetTotalRequests())
	}

	if metrics.GetSuccessfulRequests() != 0 {
		t.Errorf("Expected 0 successful requests initially, got %v", metrics.GetSuccessfulRequests())
	}

	if metrics.GetFailedRequests() != 5 {
		t.Errorf("Expected 5 failed requests initially, got %v", metrics.GetFailedRequests())
	}

	if metrics.IsConnected() != false {
		t.Error("Client should not be marked as connected initially")
	}

	if metrics.GetStatus() != StatusError.Status() {
		t.Errorf("Expected initial status StatusError, got %v", metrics.GetStatus())
	}
}

func TestLanguageClientClose(t *testing.T) {
	client, err := NewLanguageClient("echo")
	if err != nil {
		t.Fatalf("Failed to create language client: %v", err)
	}

	_, err = client.Connect()
	if err != nil {
		t.Fatalf("Failed to connect to language client: %v", err)
	}
	// Close the client
	err = client.Close()
	if err != nil {
		t.Errorf("Close() returned an error: %v", err)
	}

	// Verify post-close state
	if client.IsConnected() {
		t.Error("Client should not be connected after Close()")
	}

	if client.Status() != StatusUninitialized.Status() {
		t.Errorf("Expected status Uninitialized after Close(), got %v", client.Status())
	}
}

func TestConnectErrorHandling(t *testing.T) {
	client, err := NewLanguageClient("nonexistent_command")
	if err != nil {
		t.Fatal("Expected error when creating client with nonexistent command")
	}

	connected_client, err := client.Connect()
	if err == nil {
		t.Fatalf("Expected to error when connecting to client: %v", err)
	}

	if connected_client != nil {
		t.Error("Expected nil client when creation fails")
		closeClient(t, client) // Clean up if somehow not nil
	}
}

func TestClientCapabilitiesAndServerCapabilities(t *testing.T) {
	client, err := NewLanguageClient("mock-lsp-server")
	if err != nil {
		t.Fatalf("Failed to create language client: %v", err)
	}

	_, err = client.Connect()
	if err != nil {
		t.Fatalf("Failed to connect to language client: %v", err)
	}

	defer closeClient(t, client)

	// Test client capabilities
	clientCaps := client.ClientCapabilities()
	if clientCaps != (protocol.ClientCapabilities{}) {
		t.Error("Initial client capabilities should be empty")
	}

	// Test server capabilities setting
	testServerCaps := protocol.ServerCapabilities{
		TextDocumentSync: &protocol.Or2[protocol.TextDocumentSyncOptions, protocol.TextDocumentSyncKind]{
			Value: protocol.TextDocumentSyncKind(1),
		},
	}
	client.SetServerCapabilities(testServerCaps)

	serverCaps := client.ServerCapabilities()
	if serverCaps.TextDocumentSync == nil {
		t.Error("Server capabilities not set correctly")
	}
}

// Benchmark client creation and basic operations
func BenchmarkLanguageClientCreation(b *testing.B) {
	for b.Loop() {
		client, err := NewLanguageClient("mock-lsp-server")
		if err != nil {
			b.Fatalf("Failed to create language client: %v", err)
		}

		if err := client.Close(); err != nil {
			b.Logf("failed to close client: %v", err)
		}
	}
}

func BenchmarkSendRequest(b *testing.B) {
	client, err := NewLanguageClient("mock-lsp-server")
	if err != nil {
		b.Fatalf("Failed to create language client: %v", err)
	}

	defer func() {
		if err := client.Close(); err != nil {
			b.Logf("failed to close client: %v", err)
		}
	}()

	for b.Loop() {
		err := client.SendRequest(
			"test/method",
			map[string]any{"key": "value"},
			&map[string]any{},
			1*time.Second,
		)
		// Ignore errors since echo doesn't understand LSP protocol
		_ = err
	}
}

func closeClient(t *testing.T, client types.LanguageClientInterface) func() {
	return func() {
		if err := client.Close(); err != nil {
			t.Logf("failed to close client: %v", err)
		}
	}
}
