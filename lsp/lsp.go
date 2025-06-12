package lsp

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"os"
	"os/exec"
	"rockerboo/mcp-lsp-bridge/mcp_lsp_bridge"
	"time"

	"github.com/myleshyson/lsprotocol-go/protocol"
	"github.com/sourcegraph/jsonrpc2"
)

// type JSONRPCError struct {
// 	Code    int         `json:"code"`
// 	Message string      `json:"message"`
// 	Data    interface{} `json:"data,omitempty"`
// }

// stdioReadWriteCloser combines stdin and stdout into a ReadWriteCloser
type stdioReadWriteCloser struct {
	stdin  io.WriteCloser
	stdout io.ReadCloser
}

func (rwc *stdioReadWriteCloser) Read(p []byte) (n int, err error) {
	return rwc.stdout.Read(p)
}

func (rwc *stdioReadWriteCloser) Write(p []byte) (n int, err error) {
	return rwc.stdin.Write(p)
}

func (rwc *stdioReadWriteCloser) Close() error {
	err1 := rwc.stdin.Close()
	err2 := rwc.stdout.Close()
	if err1 != nil {
		return err1
	}
	return err2
}

// Language Client wrapper
type LanguageClient struct {
	conn               *jsonrpc2.Conn
	ctx                context.Context
	cancel             context.CancelFunc
	cmd                *exec.Cmd
	clientCapabilities protocol.ClientCapabilities
	serverCapabilities protocol.ServerCapabilities
}

// Handler for incoming messages from server
type ClientHandler struct {
	client *LanguageClient
}

func (h *ClientHandler) Handle(ctx context.Context, conn *jsonrpc2.Conn, req *jsonrpc2.Request) {
	fmt.Printf("Received from server - Method: %s, ID: %v\n", req.Method, req.ID)

	switch req.Method {
	case "textDocument/publishDiagnostics":
		// Handle diagnostics
		var params any
		if err := json.Unmarshal(*req.Params, &params); err == nil {
			fmt.Printf("Diagnostics: %+v\n", params)
		}

	case "window/showMessage":
		// Handle show message
		var params any
		if err := json.Unmarshal(*req.Params, &params); err == nil {
			fmt.Printf("Server message: %+v\n", params)
		}

	case "window/logMessage":
		// Handle log message
		var params any
		if err := json.Unmarshal(*req.Params, &params); err == nil {
			fmt.Printf("Server log: %+v\n", params)
		}

	case "client/registerCapability":
		// Handle capability registration - reply with success
		if err := conn.Reply(ctx, req.ID, map[string]any{}); err != nil {
			fmt.Printf("Failed to reply to registerCapability: %v\n", err)
		}

	case "workspace/configuration":
		// Handle configuration request - reply with empty config
		if err := conn.Reply(ctx, req.ID, []any{}); err != nil {
			fmt.Printf("Failed to reply to configuration: %v\n", err)
		}

	default:
		fmt.Printf("Unhandled method: %s with params: %s\n", req.Method, string(*req.Params))

		err := &jsonrpc2.Error{
			Code:    jsonrpc2.CodeMethodNotFound,
			Message: "Method not found",
		}
		if replyErr := conn.ReplyWithError(ctx, req.ID, err); replyErr != nil {
			fmt.Printf("Failed to reply with error: %v\n", replyErr)
		}
	}
}

func NewLanguageClient(command string, args ...string) (*LanguageClient, error) {
	// Create cancellable context for the entire session
	ctx, cancel := context.WithCancel(context.Background())

	// Start the external process
	cmd := exec.CommandContext(ctx, command, args...)

	stdin, err := cmd.StdinPipe()
	if err != nil {
		cancel()
		return nil, fmt.Errorf("failed to create stdin pipe: %w", err)
	}

	stdout, err := cmd.StdoutPipe()
	if err != nil {
		stdin.Close()
		cancel()
		return nil, fmt.Errorf("failed to create stdout pipe: %w", err)
	}

	stderr, err := cmd.StderrPipe()
	if err != nil {
		stdin.Close()
		stdout.Close()
		cancel()
		return nil, fmt.Errorf("failed to create stderr pipe: %w", err)
	}

	// Start the process
	if err := cmd.Start(); err != nil {
		stdin.Close()
		stdout.Close()
		stderr.Close()
		cancel()
		return nil, fmt.Errorf("failed to start command: %w", err)
	}

	client := &LanguageClient{
		ctx:                ctx,
		cancel:             cancel,
		cmd:                cmd,
		clientCapabilities: protocol.ClientCapabilities{},
		serverCapabilities: protocol.ServerCapabilities{},
	}

	// Create a ReadWriteCloser that combines stdin and stdout for LSP
	readWriteCloser := &stdioReadWriteCloser{
		stdin:  stdin,
		stdout: stdout,
	}

	// Create handler
	handler := &ClientHandler{client: client}

	// Create JSON-RPC connection using VSCode Object Codec for LSP headers
	stream := jsonrpc2.NewBufferedStream(readWriteCloser, jsonrpc2.VSCodeObjectCodec{})
	conn := jsonrpc2.NewConn(ctx, stream, handler)

	client.conn = conn

	// Handle stderr in background
	go func() {
		buf := make([]byte, 1024)
		for {
			n, err := stderr.Read(buf)
			if err != nil {
				break
			}
			if n > 0 {
				fmt.Fprintf(os.Stderr, "[SERVER STDERR]: %s", buf[:n])
			}
		}
		stderr.Close()
	}()

	return client, nil
}

func (lc *LanguageClient) Close() error {
	if lc.conn != nil {
		lc.conn.Close()
	}

	lc.cancel() // Cancel the context

	// Wait for process to exit or kill it
	if lc.cmd != nil && lc.cmd.Process != nil {
		done := make(chan error, 1)
		go func() {
			done <- lc.cmd.Wait()
		}()

		select {
		case <-done:
			// Process exited gracefully
		case <-time.After(5 * time.Second):
			// Force kill if it doesn't exit gracefully
			lc.cmd.Process.Kill()
			<-done // Wait for it to actually exit
		}
	}

	return nil
}

func (lc *LanguageClient) ClientCapabilities() protocol.ClientCapabilities {
	return lc.clientCapabilities
}

func (lc *LanguageClient) ServerCapabilities() protocol.ServerCapabilities {
	return lc.serverCapabilities
}

func (lc *LanguageClient) SetServerCapabilities(capabiltiies protocol.ServerCapabilities) {
	lc.serverCapabilities = capabiltiies
}

// Send request with timeout
func (lc *LanguageClient) SendRequest(method string, params any, result any, timeout time.Duration) error {
	reqCtx, cancel := context.WithTimeout(lc.ctx, timeout)
	defer cancel()

	return lc.conn.Call(reqCtx, method, params, result)
}

// Send request without timeout
func (lc *LanguageClient) SendRequestNoTimeout(method string, params any, result any) error {
	return lc.conn.Call(lc.ctx, method, params, result)
}

// Send notification
func (lc *LanguageClient) SendNotification(method string, params any) error {
	return lc.conn.Notify(lc.ctx, method, params)
}

// Get the context
func (lc *LanguageClient) Context() context.Context {
	return lc.ctx
}

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

// Example of a more structured approach for specific LSP methods
func (lc *LanguageClient) Initialize(params protocol.InitializeParams) (*protocol.InitializeResult, error) {
	var result protocol.InitializeResult
	err := lc.SendRequest("initialize", params, &result, 10*time.Second)
	if err != nil {
		return nil, err
	}
	return &result, nil
}

func (lc *LanguageClient) Initialized() error {
	return lc.SendNotification("initialized", protocol.InitializedParams{})
}

func (lc *LanguageClient) Shutdown() error {
	var result protocol.ShutdownResponse
	return lc.SendRequest("shutdown", nil, &result, 5*time.Second)
}

func (lc *LanguageClient) Exit() error {
	return lc.SendNotification("exit", nil)
}

func (lc *LanguageClient) DidOpen(uri string, languageId protocol.LanguageKind, text string, version int32) error {

	params := protocol.DidOpenTextDocumentParams{
		TextDocument: protocol.TextDocumentItem{
			Uri:        protocol.DocumentUri(uri),
			LanguageId: languageId,
			Version:    version,
			Text:       text,
		},
	}
	return lc.SendNotification("textDocument/didOpen", params)
}

func (lc *LanguageClient) DidChange(uri string, version int32, changes []protocol.TextDocumentContentChangeEvent) error {
	params := protocol.DidChangeTextDocumentParams{
		TextDocument: protocol.VersionedTextDocumentIdentifier{
			Uri:     protocol.DocumentUri(uri),
			Version: version,
		},
		ContentChanges: changes,
	}
	return lc.SendNotification("textDocument/didChange", params)
}

func (lc *LanguageClient) DidSave(uri string, text *string) error {
	params := map[string]interface{}{
		"textDocument": map[string]interface{}{
			"uri": uri,
		},
	}
	if text != nil {
		params["text"] = *text
	}
	return lc.SendNotification("textDocument/didSave", params)
}

func (lc *LanguageClient) DidClose(uri string) error {
	params := map[string]interface{}{
		"textDocument": map[string]interface{}{
			"uri": uri,
		},
	}
	return lc.SendNotification("textDocument/didClose", params)
}
