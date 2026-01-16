// Session Client - Client for LSP Session Manager
//
// This client connects to the LSP Session Manager daemon and provides
// a simple interface for making LSP requests through the persistent session.

package lsp

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"net"
	"os"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"rockerboo/mcp-lsp-bridge/logger"
)

// SessionClient connects to LSP Session Manager
type SessionClient struct {
	host string
	port int

	mu       sync.Mutex
	conn     net.Conn
	reader   *bufio.Reader
	reqID    int64
	pending  map[int64]chan sessionResponse
}

type sessionResponse struct {
	Result json.RawMessage `json:"result"`
	Error  *sessionError   `json:"error"`
}

type sessionError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

// NewSessionClient creates a new Session Manager client
func NewSessionClient(host string, port int) *SessionClient {
	return &SessionClient{
		host:    host,
		port:    port,
		pending: make(map[int64]chan sessionResponse),
	}
}

// Connect establishes connection to Session Manager
func (sc *SessionClient) Connect() error {
	addr := fmt.Sprintf("%s:%d", sc.host, sc.port)
	logger.Info(fmt.Sprintf("Connecting to Session Manager at %s", addr))

	var conn net.Conn
	var err error

	// Retry connection
	for i := 0; i < 10; i++ {
		conn, err = net.DialTimeout("tcp", addr, 5*time.Second)
		if err == nil {
			break
		}
		logger.Debug(fmt.Sprintf("Connection attempt %d failed: %v", i+1, err))
		time.Sleep(time.Second)
	}

	if err != nil {
		return fmt.Errorf("failed to connect to Session Manager: %w", err)
	}

	sc.mu.Lock()
	sc.conn = conn
	sc.reader = bufio.NewReader(conn)
	sc.mu.Unlock()

	// Start response reader
	go sc.readResponses()

	logger.Info("Connected to Session Manager")
	return nil
}

// Close closes the connection
func (sc *SessionClient) Close() error {
	sc.mu.Lock()
	defer sc.mu.Unlock()

	if sc.conn != nil {
		return sc.conn.Close()
	}
	return nil
}

// IsConnected returns true if connected
func (sc *SessionClient) IsConnected() bool {
	sc.mu.Lock()
	defer sc.mu.Unlock()
	return sc.conn != nil
}

// GetStatus gets session status
func (sc *SessionClient) GetStatus(ctx context.Context) (map[string]interface{}, error) {
	var result map[string]interface{}
	err := sc.Call(ctx, "session/status", nil, &result)
	return result, err
}

// Hover sends textDocument/hover request
func (sc *SessionClient) Hover(ctx context.Context, uri string, line, character uint32) (json.RawMessage, error) {
	params := map[string]interface{}{
		"textDocument": map[string]interface{}{
			"uri": uri,
		},
		"position": map[string]interface{}{
			"line":      line,
			"character": character,
		},
	}

	var result json.RawMessage
	err := sc.Call(ctx, "textDocument/hover", params, &result)
	return result, err
}

// Definition sends textDocument/definition request
func (sc *SessionClient) Definition(ctx context.Context, uri string, line, character uint32) (json.RawMessage, error) {
	params := map[string]interface{}{
		"textDocument": map[string]interface{}{
			"uri": uri,
		},
		"position": map[string]interface{}{
			"line":      line,
			"character": character,
		},
	}

	var result json.RawMessage
	err := sc.Call(ctx, "textDocument/definition", params, &result)
	return result, err
}

// References sends textDocument/references request
func (sc *SessionClient) References(ctx context.Context, uri string, line, character uint32, includeDeclaration bool) (json.RawMessage, error) {
	params := map[string]interface{}{
		"textDocument": map[string]interface{}{
			"uri": uri,
		},
		"position": map[string]interface{}{
			"line":      line,
			"character": character,
		},
		"context": map[string]interface{}{
			"includeDeclaration": includeDeclaration,
		},
	}

	var result json.RawMessage
	err := sc.Call(ctx, "textDocument/references", params, &result)
	return result, err
}

// DocumentSymbols sends textDocument/documentSymbol request
func (sc *SessionClient) DocumentSymbols(ctx context.Context, uri string) (json.RawMessage, error) {
	params := map[string]interface{}{
		"textDocument": map[string]interface{}{
			"uri": uri,
		},
	}

	var result json.RawMessage
	err := sc.Call(ctx, "textDocument/documentSymbol", params, &result)
	return result, err
}

// PrepareCallHierarchy sends textDocument/prepareCallHierarchy request
func (sc *SessionClient) PrepareCallHierarchy(ctx context.Context, uri string, line, character uint32) (json.RawMessage, error) {
	params := map[string]interface{}{
		"textDocument": map[string]interface{}{
			"uri": uri,
		},
		"position": map[string]interface{}{
			"line":      line,
			"character": character,
		},
	}

	var result json.RawMessage
	err := sc.Call(ctx, "textDocument/prepareCallHierarchy", params, &result)
	return result, err
}

// IncomingCalls sends callHierarchy/incomingCalls request
func (sc *SessionClient) IncomingCalls(ctx context.Context, item json.RawMessage) (json.RawMessage, error) {
	// item is already json.RawMessage - unmarshal to interface{} to avoid double encoding
	var itemObj interface{}
	if err := json.Unmarshal(item, &itemObj); err != nil {
		return nil, err
	}
	
	params := map[string]interface{}{
		"item": itemObj,
	}

	var result json.RawMessage
	err := sc.Call(ctx, "callHierarchy/incomingCalls", params, &result)
	return result, err
}

// OutgoingCalls sends callHierarchy/outgoingCalls request
func (sc *SessionClient) OutgoingCalls(ctx context.Context, item json.RawMessage) (json.RawMessage, error) {
	// item is already json.RawMessage - unmarshal to interface{} to avoid double encoding
	var itemObj interface{}
	if err := json.Unmarshal(item, &itemObj); err != nil {
		return nil, err
	}
	
	params := map[string]interface{}{
		"item": itemObj,
	}

	var result json.RawMessage
	err := sc.Call(ctx, "callHierarchy/outgoingCalls", params, &result)
	return result, err
}

// DidOpen sends textDocument/didOpen notification
func (sc *SessionClient) DidOpen(ctx context.Context, uri, languageID, text string) error {
	params := map[string]interface{}{
		"textDocument": map[string]interface{}{
			"uri":        uri,
			"languageId": languageID,
			"version":    1,
			"text":       text,
		},
	}

	var result interface{}
	return sc.Call(ctx, "textDocument/didOpen", params, &result)
}

// DidClose sends textDocument/didClose notification
func (sc *SessionClient) DidClose(ctx context.Context, uri string) error {
	params := map[string]interface{}{
		"textDocument": map[string]interface{}{
			"uri": uri,
		},
	}

	var result interface{}
	return sc.Call(ctx, "textDocument/didClose", params, &result)
}

// Diagnostic sends textDocument/diagnostic request
func (sc *SessionClient) Diagnostic(ctx context.Context, uri string) (json.RawMessage, error) {
	params := map[string]interface{}{
		"textDocument": map[string]interface{}{
			"uri": uri,
		},
	}

	var result json.RawMessage
	err := sc.Call(ctx, "textDocument/diagnostic", params, &result)
	return result, err
}

// WorkspaceSymbol sends workspace/symbol request
func (sc *SessionClient) WorkspaceSymbol(ctx context.Context, query string) (json.RawMessage, error) {
	params := map[string]interface{}{
		"query": query,
	}

	var result json.RawMessage
	err := sc.Call(ctx, "workspace/symbol", params, &result)
	return result, err
}

// Call makes a JSON-RPC call to Session Manager
func (sc *SessionClient) Call(ctx context.Context, method string, params interface{}, result interface{}) error {
	fmt.Fprintf(os.Stderr, "DEBUG SessionClient.Call: method=%s\n", method)
	
	sc.mu.Lock()
	if sc.conn == nil {
		sc.mu.Unlock()
		fmt.Fprintf(os.Stderr, "DEBUG SessionClient.Call: NOT CONNECTED!\n")
		return fmt.Errorf("not connected to Session Manager")
	}

	id := atomic.AddInt64(&sc.reqID, 1)
	respCh := make(chan sessionResponse, 1)
	sc.pending[id] = respCh
	sc.mu.Unlock()

	defer func() {
		sc.mu.Lock()
		delete(sc.pending, id)
		sc.mu.Unlock()
	}()

	// Build request
	req := map[string]interface{}{
		"jsonrpc": "2.0",
		"id":      id,
		"method":  method,
		"params":  params,
	}

	reqJSON, err := json.Marshal(req)
	if err != nil {
		return fmt.Errorf("failed to marshal request: %w", err)
	}

	fmt.Fprintf(os.Stderr, "DEBUG SessionClient.Call: sending request id=%d\n", id)

	// Send request (newline-delimited)
	sc.mu.Lock()
	_, err = sc.conn.Write(append(reqJSON, '\n'))
	sc.mu.Unlock()

	if err != nil {
		fmt.Fprintf(os.Stderr, "DEBUG SessionClient.Call: write error: %v\n", err)
		return fmt.Errorf("failed to send request: %w", err)
	}
	
	fmt.Fprintf(os.Stderr, "DEBUG SessionClient.Call: waiting for response...\n")

	// Wait for response
	select {
	case resp := <-respCh:
		if resp.Error != nil {
			return fmt.Errorf("session manager error: %s", resp.Error.Message)
		}
		if result != nil && resp.Result != nil {
			return json.Unmarshal(resp.Result, result)
		}
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

// readResponses reads responses from Session Manager
func (sc *SessionClient) readResponses() {
	fmt.Fprintf(os.Stderr, "DEBUG readResponses: goroutine started\n")
	for {
		sc.mu.Lock()
		reader := sc.reader
		sc.mu.Unlock()

		if reader == nil {
			fmt.Fprintf(os.Stderr, "DEBUG readResponses: reader is nil, exiting\n")
			return
		}

		fmt.Fprintf(os.Stderr, "DEBUG readResponses: waiting for line...\n")
		line, err := reader.ReadString('\n')
		if err != nil {
			fmt.Fprintf(os.Stderr, "DEBUG readResponses: read error: %v\n", err)
			logger.Error(fmt.Sprintf("Session Manager read error: %v", err))
			return
		}

		fmt.Fprintf(os.Stderr, "DEBUG readResponses: received line: %s\n", strings.TrimSpace(line))

		var resp struct {
			JSONRPC string          `json:"jsonrpc"`
			ID      int64           `json:"id"`
			Result  json.RawMessage `json:"result"`
			Error   *sessionError   `json:"error"`
		}

		if err := json.Unmarshal([]byte(line), &resp); err != nil {
			fmt.Fprintf(os.Stderr, "DEBUG readResponses: parse error: %v\n", err)
			logger.Error(fmt.Sprintf("Failed to parse response: %v", err))
			continue
		}

		fmt.Fprintf(os.Stderr, "DEBUG readResponses: parsed response id=%d\n", resp.ID)

		sc.mu.Lock()
		if ch, ok := sc.pending[resp.ID]; ok {
			fmt.Fprintf(os.Stderr, "DEBUG readResponses: sending to pending channel id=%d\n", resp.ID)
			ch <- sessionResponse{
				Result: resp.Result,
				Error:  resp.Error,
			}
		} else {
			fmt.Fprintf(os.Stderr, "DEBUG readResponses: no pending request for id=%d\n", resp.ID)
		}
		sc.mu.Unlock()
	}
}
