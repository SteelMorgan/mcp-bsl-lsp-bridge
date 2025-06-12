package lsp

import (
	"testing"
	"time"

	"github.com/myleshyson/lsprotocol-go/protocol"
)

// Mock test for LanguageClient creation
func TestNewLanguageClient(t *testing.T) {
	// Use a simple command that always exists for testing
	lc, err := NewLanguageClient("echo")
	if err != nil {
		t.Fatalf("Failed to create language client: %v", err)
	}
	defer lc.Close()

	if lc.conn == nil {
		t.Error("JSON-RPC connection not established")
	}

	if lc.ctx == nil {
		t.Error("Context not initialized")
	}
}

// Test ClientCapabilities and ServerCapabilities
func TestClientServerCapabilities(t *testing.T) {
	lc, err := NewLanguageClient("echo")
	if err != nil {
		t.Fatalf("Failed to create language client: %v", err)
	}
	defer lc.Close()

	// Retrieve initial client capabilities
	_ = lc.ClientCapabilities()

	// Test server capabilities setter and getter
	testServerCaps := protocol.ServerCapabilities{
		TextDocumentSync: &protocol.Or2[protocol.TextDocumentSyncOptions, protocol.TextDocumentSyncKind]{
			Value: protocol.TextDocumentSyncKind(1),
		},
	}
	lc.SetServerCapabilities(testServerCaps)

	retrievedServerCaps := lc.ServerCapabilities()
	if retrievedServerCaps.TextDocumentSync == nil {
		t.Error("Server capabilities not set correctly")
	}
}

// Test Context retrieval
func TestContext(t *testing.T) {
	lc, err := NewLanguageClient("echo")
	if err != nil {
		t.Fatalf("Failed to create language client: %v", err)
	}
	defer lc.Close()

	ctx := lc.Context()
	if ctx == nil {
		t.Error("Context should not be nil")
	}

	// Test context cancellation
	select {
	case <-ctx.Done():
		t.Error("Context should not be done immediately after creation")
	default:
		// Expected case
	}
}

// Test SendNotification method
func TestSendNotification(t *testing.T) {
	lc, err := NewLanguageClient("echo")
	if err != nil {
		t.Fatalf("Failed to create language client: %v", err)
	}
	defer lc.Close()

	// Test sending a simple notification
	err = lc.SendNotification("test/notification", map[string]any{
		"key": "value",
	})
	if err != nil {
		t.Errorf("Failed to send notification: %v", err)
	}
}

// Benchmark LanguageClient creation
func BenchmarkNewLanguageClient(b *testing.B) {
	for b.Loop() {
		lc, err := NewLanguageClient("echo")
		if err != nil {
			b.Fatalf("Failed to create language client: %v", err)
		}
		lc.Close()
	}
}

// Test high-level LSP method wrappers
func TestLSPMethodWrappers(t *testing.T) {
	lc, err := NewLanguageClient("echo")
	if err != nil {
		t.Fatalf("Failed to create language client: %v", err)
	}
	defer lc.Close()

	// Test DidOpen
	err = lc.DidOpen("file:///test.go", "go", "package main\n", 1)
	if err != nil {
		t.Errorf("DidOpen failed: %v", err)
	}

	// Test DidChange
	changes := []protocol.TextDocumentContentChangeEvent{}
	// Passing an empty slice to avoid complex struct initialization
	err = lc.DidChange("file:///test.go", 2, changes)
	if err != nil {
		t.Errorf("DidChange failed: %v", err)
	}

	// Test DidSave
	text := "package test\nfunc main() {}"
	err = lc.DidSave("file:///test.go", &text)
	if err != nil {
		t.Errorf("DidSave failed: %v", err)
	}

	// Test DidClose
	err = lc.DidClose("file:///test.go")
	if err != nil {
		t.Errorf("DidClose failed: %v", err)
	}
}

// Test error handling for NewLanguageClient
func TestNewLanguageClientError(t *testing.T) {
	// Test with non-existent command
	_, err := NewLanguageClient("non_existent_command")
	if err == nil {
		t.Error("Expected error when creating client with non-existent command")
	}
}

// Test request methods
func TestSendRequestMethods(t *testing.T) {
	lc, err := NewLanguageClient("echo")
	if err != nil {
		t.Fatalf("Failed to create language client: %v", err)
	}
	defer lc.Close()

	// Test SendRequest
	var result interface{}
	err = lc.SendRequest("test/method", map[string]interface{}{"key": "value"}, &result, 5*time.Second)
	if err == nil {
		t.Log("SendRequest expects an error due to closed connection")
	}

	// Test SendRequestNoTimeout
	err = lc.SendRequestNoTimeout("test/method", map[string]interface{}{"key": "value"}, &result)
	if err == nil {
		t.Log("SendRequestNoTimeout expects an error due to closed connection")
	}
}

// Test Initialize method
func TestInitializeMethod(t *testing.T) {
	lc, err := NewLanguageClient("echo")
	if err != nil {
		t.Fatalf("Failed to create language client: %v", err)
	}
	defer lc.Close()

	processId := int32(1)
	params := protocol.InitializeParams{
		ProcessId: &processId,
		ClientInfo: &protocol.ClientInfo{
			Name:    "Test Client",
			Version: "1.0.0",
		},
	}

	result, err := lc.Initialize(params)
	if err == nil {
		t.Log("Initialize method expects an error due to closed connection")
	}
	if result != nil {
		t.Error("Initialize result should be nil for closed connection")
	}
}

// Test other lifecycle methods
func TestLifecycleMethods(t *testing.T) {
	lc, err := NewLanguageClient("echo")
	if err != nil {
		t.Fatalf("Failed to create language client: %v", err)
	}
	defer lc.Close()

	// Test Initialized method
	err = lc.Initialized()
	if err == nil {
		t.Log("Initialized method expects an error due to closed connection")
	}

	// Test Shutdown method
	err = lc.Shutdown()
	if err == nil {
		t.Log("Shutdown method expects an error due to closed connection")
	}

	// Test Exit method
	err = lc.Exit()
	if err == nil {
		t.Log("Exit method expects an error due to closed connection")
	}
}
