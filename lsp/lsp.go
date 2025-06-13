package lsp

import (
	"fmt"
	"log"
	"os"
	"time"

	"rockerboo/mcp-lsp-bridge/mcp_lsp_bridge"

	"github.com/myleshyson/lsprotocol-go/protocol"
)

// Usage example
func main() {
	// Create language client - replace with your actual server command
	lc, err := NewLanguageClient("typescript-language-server", "--stdio")
	if err != nil {
		fmt.Printf("Failed to create client: %v\n", err)
		return
	}
	defer lc.Close()

	dir, err := os.Getwd()
	if err != nil {
		log.Fatal(err)
	}

	process_id := int32(os.Getpid())
	root_uri := protocol.DocumentUri(fmt.Sprintf("file://%s", dir))

	// Create initialize request parameters
	params := protocol.InitializeParams{
		ProcessId: &process_id,
		ClientInfo: &protocol.ClientInfo{
			Name:    "MCP LSP Client",
			Version: "1.0.0",
		},
		RootUri:      &root_uri,
		Capabilities: protocol.ClientCapabilities{},
	}

	// Send initialize request
	var result protocol.InitializeResult
	err = lc.SendRequest("initialize", params, &result, 10*time.Second)
	if err != nil {
		fmt.Printf("Initialize failed: %v\n", err)
		return
	}

	fmt.Printf("Initialize result: %+v\n", result)

	// Send initialized notification
	err = lc.SendNotification("initialized", map[string]any{})
	if err != nil {
		fmt.Printf("Failed to send initialized notification: %v\n", err)
		return
	}

	// Example: Send textDocument/didOpen notification
	didOpenParams := protocol.DidOpenTextDocumentParams{
		TextDocument: protocol.TextDocumentItem{
			Uri:        "file:///path/to/file.ts",
			LanguageId: "typescript",
			Version:    1,
			Text:       "console.log('Hello, world!');",
		},
	}

	err = lc.SendNotification("textDocument/didOpen", didOpenParams)
	if err != nil {
		fmt.Printf("Failed to send didOpen: %v\n", err)
		return
	}

	// Example: Send hover request
	hoverParams := protocol.HoverParams{
		TextDocument: protocol.TextDocumentIdentifier{
			Uri: "file:///path/to/file.ts",
		},
		Position: protocol.Position{
			Line:      0,
			Character: 0,
		},
	}

	var hoverResult protocol.HoverResponse
	err = lc.SendRequest("textDocument/hover", hoverParams, &hoverResult, 5*time.Second)
	if err != nil {
		fmt.Printf("Hover request failed: %v\n", err)
	} else {
		fmt.Printf("Hover result: %+v\n", mcp_lsp_bridge.SafePrettyPrint(hoverResult))
	}

	// Example: Send completion request
	completionParams := protocol.CompletionParams{
		TextDocument: protocol.TextDocumentIdentifier{
			Uri: "file:///path/to/file.ts",
		},
		Position: protocol.Position{
			Line:      0,
			Character: 7,
		},
	}

	var completionResult protocol.CompletionResponse
	err = lc.SendRequest("textDocument/completion", completionParams, &completionResult, 5*time.Second)
	if err != nil {
		fmt.Printf("Completion request failed: %v\n", err)
	} else {
		fmt.Printf("Completion result: %+v\n", mcp_lsp_bridge.SafePrettyPrint(completionResult))
	}

	// Keep running to receive notifications
	fmt.Println("Client running... Press Ctrl+C to exit")

	// Wait for connection to close or context cancellation
	select {
	case <-lc.Context().Done():
		fmt.Println("Context cancelled")
	case <-time.After(30 * time.Second):
		fmt.Println("Timeout reached")
	}
}