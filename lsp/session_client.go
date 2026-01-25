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
	"sync"
	"sync/atomic"
	"time"

	"rockerboo/mcp-lsp-bridge/logger"
)

// SessionClient connects to LSP Session Manager
type SessionClient struct {
	host string
	port int

	mu      sync.Mutex
	conn    net.Conn
	reader  *bufio.Reader
	reqID   int64
	pending map[int64]chan sessionResponse
	closed  bool // true if explicitly closed (not error)
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

	sc.closed = true
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
func (sc *SessionClient) Diagnostic(ctx context.Context, uri string, identifier string, previousResultId string) (json.RawMessage, error) {
	params := map[string]interface{}{
		"textDocument": map[string]interface{}{
			"uri": uri,
		},
	}
	if identifier != "" {
		params["identifier"] = identifier
	}
	if previousResultId != "" {
		params["previousResultId"] = previousResultId
	}

	var result json.RawMessage
	err := sc.Call(ctx, "textDocument/diagnostic", params, &result)
	return result, err
}

// Formatting sends textDocument/formatting request
func (sc *SessionClient) Formatting(ctx context.Context, uri string, tabSize uint32, insertSpaces bool) (json.RawMessage, error) {
	params := map[string]interface{}{
		"textDocument": map[string]interface{}{
			"uri": uri,
		},
		"options": map[string]interface{}{
			"tabSize":      tabSize,
			"insertSpaces": insertSpaces,
		},
	}
	var result json.RawMessage
	err := sc.Call(ctx, "textDocument/formatting", params, &result)
	return result, err
}

// PrepareRename sends textDocument/prepareRename request
func (sc *SessionClient) PrepareRename(ctx context.Context, uri string, line, character uint32) (json.RawMessage, error) {
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
	err := sc.Call(ctx, "textDocument/prepareRename", params, &result)
	return result, err
}

// Rename sends textDocument/rename request
func (sc *SessionClient) Rename(ctx context.Context, uri string, line, character uint32, newName string) (json.RawMessage, error) {
	params := map[string]interface{}{
		"textDocument": map[string]interface{}{
			"uri": uri,
		},
		"position": map[string]interface{}{
			"line":      line,
			"character": character,
		},
		"newName": newName,
	}
	var result json.RawMessage
	err := sc.Call(ctx, "textDocument/rename", params, &result)
	return result, err
}

// WorkspaceDiagnostic sends workspace/diagnostic request
func (sc *SessionClient) WorkspaceDiagnostic(ctx context.Context, identifier string) (json.RawMessage, error) {
	params := map[string]interface{}{}
	// LSP 3.17 supports identifier; keep it optional
	if identifier != "" {
		params["identifier"] = identifier
	}
	var result json.RawMessage
	err := sc.Call(ctx, "workspace/diagnostic", params, &result)
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
	// Check connection and try to reconnect if needed
	sc.mu.Lock()
	if sc.conn == nil && !sc.closed {
		sc.mu.Unlock()
		if err := sc.reconnect(); err != nil {
			return fmt.Errorf("not connected to Session Manager and reconnect failed: %w", err)
		}
		// Start reader goroutine after reconnect
		go sc.readResponses()
		sc.mu.Lock()
	}

	if sc.conn == nil {
		sc.mu.Unlock()
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

	// Send request (newline-delimited)
	sc.mu.Lock()
	conn := sc.conn
	sc.mu.Unlock()

	if conn == nil {
		return fmt.Errorf("connection lost before sending request")
	}

	_, err = conn.Write(append(reqJSON, '\n'))
	if err != nil {
		// Mark connection as broken
		sc.mu.Lock()
		sc.conn = nil
		sc.reader = nil
		sc.mu.Unlock()
		return fmt.Errorf("failed to send request: %w", err)
	}

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
	for {
		sc.mu.Lock()
		reader := sc.reader
		closed := sc.closed
		sc.mu.Unlock()

		if reader == nil || closed {
			return
		}

		line, err := reader.ReadString('\n')
		if err != nil {
			logger.Error(fmt.Sprintf("Session Manager read error: %v", err))

			// Fail all pending requests
			sc.failAllPending(fmt.Errorf("connection lost: %w", err))

			// Try to reconnect if not explicitly closed
			sc.mu.Lock()
			wasClosed := sc.closed
			sc.mu.Unlock()

			if !wasClosed {
				logger.Info("Attempting to reconnect to Session Manager...")
				if reconnErr := sc.reconnect(); reconnErr != nil {
					logger.Error(fmt.Sprintf("Reconnect failed: %v", reconnErr))
					return
				}
				// Reconnect succeeded, continue reading
				logger.Info("Reconnected to Session Manager")
				continue
			}
			return
		}

		var resp struct {
			JSONRPC string          `json:"jsonrpc"`
			ID      int64           `json:"id"`
			Result  json.RawMessage `json:"result"`
			Error   *sessionError   `json:"error"`
		}

		if err := json.Unmarshal([]byte(line), &resp); err != nil {
			logger.Error(fmt.Sprintf("Failed to parse response: %v", err))
			continue
		}

		sc.mu.Lock()
		if ch, ok := sc.pending[resp.ID]; ok {
			ch <- sessionResponse{
				Result: resp.Result,
				Error:  resp.Error,
			}
		}
		sc.mu.Unlock()
	}
}

// failAllPending fails all pending requests with the given error
func (sc *SessionClient) failAllPending(err error) {
	sc.mu.Lock()
	defer sc.mu.Unlock()

	for _, ch := range sc.pending {
		ch <- sessionResponse{
			Error: &sessionError{
				Code:    -32000,
				Message: err.Error(),
			},
		}
	}
}

// reconnect attempts to reconnect to Session Manager
func (sc *SessionClient) reconnect() error {
	sc.mu.Lock()
	// Close existing connection
	if sc.conn != nil {
		sc.conn.Close()
		sc.conn = nil
		sc.reader = nil
	}
	sc.mu.Unlock()

	addr := fmt.Sprintf("%s:%d", sc.host, sc.port)
	logger.Info(fmt.Sprintf("Reconnecting to Session Manager at %s", addr))

	var conn net.Conn
	var err error

	// Retry connection with backoff
	for i := 0; i < 5; i++ {
		conn, err = net.DialTimeout("tcp", addr, 5*time.Second)
		if err == nil {
			break
		}
		logger.Debug(fmt.Sprintf("Reconnect attempt %d failed: %v", i+1, err))
		time.Sleep(time.Duration(i+1) * time.Second) // Exponential backoff
	}

	if err != nil {
		return fmt.Errorf("failed to reconnect to Session Manager: %w", err)
	}

	sc.mu.Lock()
	sc.conn = conn
	sc.reader = bufio.NewReader(conn)
	sc.mu.Unlock()

	logger.Info("Reconnected to Session Manager")
	return nil
}
