// LSP Session Manager - Persistent LSP session daemon
//
// This daemon:
// 1. Starts BSL Language Server once at container startup
// 2. Initializes LSP session and waits for indexing to complete
// 3. Keeps the session alive and ready for requests
// 4. Provides a simple JSON-RPC API for mcp-lsp-bridge to call LSP methods
//
// This solves the problem of repeated initialization - BSL LS indexes once,
// and all subsequent requests use the same initialized session.

package main

import (
	"bufio"
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"log"
	"net"
	"os"
	"os/exec"
	"os/signal"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"time"
)

var (
	port        = flag.Int("port", 9999, "TCP port to listen on")
	command     = flag.String("command", "", "LSP server command to run")
	workspaceDir = flag.String("workspace", "/projects", "Workspace directory for LSP")
)

func main() {
	flag.Parse()

	if *command == "" {
		log.Fatal("--command is required")
	}

	cmdArgs := flag.Args()

	log.Printf("Starting LSP Session Manager on port %d", *port)
	log.Printf("Workspace: %s", *workspaceDir)
	log.Printf("LSP command: %s %v", *command, cmdArgs)

	// Create session manager
	sm := NewSessionManager(*command, cmdArgs, *workspaceDir)

	// Start LSP server and initialize session
	if err := sm.Start(); err != nil {
		log.Fatalf("Failed to start LSP session: %v", err)
	}

	// Start TCP listener for API requests
	listener, err := net.Listen("tcp", fmt.Sprintf(":%d", *port))
	if err != nil {
		log.Fatalf("Failed to listen on port %d: %v", *port, err)
	}
	defer listener.Close()
	log.Printf("API listening on port %d", *port)

	// Handle shutdown
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)

	go func() {
		<-sigCh
		log.Println("Shutting down...")
		sm.Stop()
		listener.Close()
		os.Exit(0)
	}()

	// Accept connections
	for {
		conn, err := listener.Accept()
		if err != nil {
			log.Printf("Accept error: %v", err)
			continue
		}
		go sm.HandleClient(conn)
	}
}

// SessionManager manages a persistent LSP session
type SessionManager struct {
	command      string
	args         []string
	workspaceDir string

	mu           sync.RWMutex
	cmd          *exec.Cmd
	stdin        io.WriteCloser
	stdout       io.Reader
	
	initialized  bool
	initResult   json.RawMessage
	capabilities json.RawMessage
	
	// Request/response handling
	requestID    int64
	pending      map[int64]chan json.RawMessage
	pendingMu    sync.Mutex
	
	// Document tracking
	openDocs     map[string]bool
	openDocsMu   sync.Mutex
}

// NewSessionManager creates a new session manager
func NewSessionManager(command string, args []string, workspaceDir string) *SessionManager {
	return &SessionManager{
		command:      command,
		args:         args,
		workspaceDir: workspaceDir,
		pending:      make(map[int64]chan json.RawMessage),
		openDocs:     make(map[string]bool),
	}
}

// Start starts the LSP server and initializes the session
func (sm *SessionManager) Start() error {
	log.Println("Starting LSP server...")

	sm.cmd = exec.Command(sm.command, sm.args...)

	var err error
	sm.stdin, err = sm.cmd.StdinPipe()
	if err != nil {
		return fmt.Errorf("failed to get stdin pipe: %w", err)
	}

	sm.stdout, err = sm.cmd.StdoutPipe()
	if err != nil {
		return fmt.Errorf("failed to get stdout pipe: %w", err)
	}

	sm.cmd.Stderr = os.Stderr

	if err := sm.cmd.Start(); err != nil {
		return fmt.Errorf("failed to start LSP server: %w", err)
	}
	log.Printf("LSP server started with PID %d", sm.cmd.Process.Pid)

	// Start response reader
	go sm.readResponses()

	// Initialize LSP session
	if err := sm.initialize(); err != nil {
		return fmt.Errorf("failed to initialize LSP session: %w", err)
	}

	return nil
}

// Stop stops the LSP server
func (sm *SessionManager) Stop() {
	if sm.cmd != nil && sm.cmd.Process != nil {
		sm.sendNotification("exit", nil)
		sm.cmd.Process.Kill()
	}
}

// initialize sends initialize request and waits for response
func (sm *SessionManager) initialize() error {
	log.Println("Initializing LSP session...")

	// Build workspace folders
	workspaceFolders := []map[string]string{
		{
			"uri":  "file://" + sm.workspaceDir,
			"name": "workspace",
		},
	}

	params := map[string]interface{}{
		"processId": nil, // Don't monitor parent process
		"capabilities": map[string]interface{}{
			"textDocument": map[string]interface{}{
				"hover": map[string]interface{}{
					"contentFormat": []string{"markdown", "plaintext"},
				},
				"definition": map[string]interface{}{
					"linkSupport": true,
				},
				"references":     map[string]interface{}{},
				"callHierarchy":  map[string]interface{}{},
				"documentSymbol": map[string]interface{}{},
				"diagnostic":     map[string]interface{}{},
			},
			"workspace": map[string]interface{}{
				"workspaceFolders": true,
			},
		},
		"rootUri":          "file://" + sm.workspaceDir,
		"workspaceFolders": workspaceFolders,
	}

	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	result, err := sm.sendRequest(ctx, "initialize", params)
	if err != nil {
		return fmt.Errorf("initialize request failed: %w", err)
	}

	sm.mu.Lock()
	sm.initResult = result
	sm.initialized = true
	sm.mu.Unlock()

	// Extract capabilities
	var initResp struct {
		Capabilities json.RawMessage `json:"capabilities"`
	}
	if err := json.Unmarshal(result, &initResp); err == nil {
		sm.mu.Lock()
		sm.capabilities = initResp.Capabilities
		sm.mu.Unlock()
	}

	log.Println("LSP session initialized successfully")

	// Send initialized notification
	if err := sm.sendNotification("initialized", map[string]interface{}{}); err != nil {
		log.Printf("Warning: failed to send initialized notification: %v", err)
	}

	log.Println("Waiting for indexing to complete...")
	// Give BSL LS time to index - we'll track progress via $/progress notifications
	time.Sleep(5 * time.Second)

	return nil
}

// sendRequest sends an LSP request and waits for response
func (sm *SessionManager) sendRequest(ctx context.Context, method string, params interface{}) (json.RawMessage, error) {
	sm.pendingMu.Lock()
	sm.requestID++
	id := sm.requestID
	respCh := make(chan json.RawMessage, 1)
	sm.pending[id] = respCh
	sm.pendingMu.Unlock()

	defer func() {
		sm.pendingMu.Lock()
		delete(sm.pending, id)
		sm.pendingMu.Unlock()
	}()

	req := map[string]interface{}{
		"jsonrpc": "2.0",
		"id":      id,
		"method":  method,
		"params":  params,
	}

	if err := sm.writeMessage(req); err != nil {
		return nil, err
	}

	select {
	case resp := <-respCh:
		return resp, nil
	case <-ctx.Done():
		return nil, ctx.Err()
	}
}

// sendNotification sends an LSP notification (no response expected)
func (sm *SessionManager) sendNotification(method string, params interface{}) error {
	req := map[string]interface{}{
		"jsonrpc": "2.0",
		"method":  method,
		"params":  params,
	}
	return sm.writeMessage(req)
}

// writeMessage writes an LSP message to the server
func (sm *SessionManager) writeMessage(msg interface{}) error {
	body, err := json.Marshal(msg)
	if err != nil {
		return err
	}

	header := fmt.Sprintf("Content-Length: %d\r\n\r\n", len(body))
	
	sm.mu.Lock()
	defer sm.mu.Unlock()
	
	if _, err := sm.stdin.Write([]byte(header)); err != nil {
		return err
	}
	if _, err := sm.stdin.Write(body); err != nil {
		return err
	}
	return nil
}

// readResponses reads responses from LSP server
func (sm *SessionManager) readResponses() {
	reader := bufio.NewReader(sm.stdout)

	for {
		msg, err := readLSPMessage(reader)
		if err != nil {
			log.Printf("LSP read error: %v", err)
			return
		}

		// Parse message
		var baseMsg struct {
			ID     *int64          `json:"id"`
			Method string          `json:"method"`
			Result json.RawMessage `json:"result"`
			Error  *struct {
				Code    int    `json:"code"`
				Message string `json:"message"`
			} `json:"error"`
		}

		if err := json.Unmarshal(msg, &baseMsg); err != nil {
			log.Printf("Failed to parse LSP message: %v", err)
			continue
		}

		// Handle response (has id, no method)
		if baseMsg.ID != nil && baseMsg.Method == "" {
			sm.pendingMu.Lock()
			if ch, ok := sm.pending[*baseMsg.ID]; ok {
				if baseMsg.Error != nil {
					// Send error as JSON
					errJSON, _ := json.Marshal(baseMsg.Error)
					ch <- errJSON
				} else {
					ch <- baseMsg.Result
				}
			}
			sm.pendingMu.Unlock()
			continue
		}

		// Handle notification (no id)
		if baseMsg.Method != "" {
			sm.handleNotification(baseMsg.Method, msg)
		}
	}
}

// handleNotification handles LSP notifications from server
func (sm *SessionManager) handleNotification(method string, msg []byte) {
	switch method {
	case "$/progress":
		// Log progress updates
		var progress struct {
			Params struct {
				Token string `json:"token"`
				Value struct {
					Kind       string `json:"kind"`
					Title      string `json:"title"`
					Message    string `json:"message"`
					Percentage int    `json:"percentage"`
				} `json:"value"`
			} `json:"params"`
		}
		if json.Unmarshal(msg, &progress) == nil {
			if progress.Params.Value.Kind != "" {
				log.Printf("Progress [%s]: %s %s (%d%%)",
					progress.Params.Value.Kind,
					progress.Params.Value.Title,
					progress.Params.Value.Message,
					progress.Params.Value.Percentage)
			}
		}
	case "textDocument/publishDiagnostics":
		// Could cache diagnostics here
	default:
		log.Printf("Notification: %s", method)
	}
}

// HandleClient handles an API client connection
func (sm *SessionManager) HandleClient(conn net.Conn) {
	defer conn.Close()
	log.Printf("API client connected: %s", conn.RemoteAddr())

	reader := bufio.NewReader(conn)

	for {
		// Read JSON-RPC request (newline-delimited)
		log.Printf("Waiting for request from %s...", conn.RemoteAddr())
		line, err := reader.ReadString('\n')
		if err != nil {
			if err != io.EOF {
				log.Printf("Client read error: %v", err)
			} else {
				log.Printf("Client %s closed connection (EOF)", conn.RemoteAddr())
			}
			break
		}

		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}

		log.Printf("Received request from %s: %s", conn.RemoteAddr(), line)

		// Parse request
		var req struct {
			JSONRPC string          `json:"jsonrpc"`
			ID      int64           `json:"id"`
			Method  string          `json:"method"`
			Params  json.RawMessage `json:"params"`
		}

		if err := json.Unmarshal([]byte(line), &req); err != nil {
			log.Printf("Parse error for request: %v", err)
			sm.sendAPIError(conn, 0, -32700, "Parse error")
			continue
		}

		log.Printf("Handling method: %s (id=%d)", req.Method, req.ID)

		// Handle request
		result, err := sm.handleAPIRequest(req.Method, req.Params)
		if err != nil {
			log.Printf("Error handling %s: %v", req.Method, err)
			sm.sendAPIError(conn, req.ID, -32603, err.Error())
			continue
		}

		log.Printf("Method %s completed successfully", req.Method)

		// Send response
		resp := map[string]interface{}{
			"jsonrpc": "2.0",
			"id":      req.ID,
			"result":  result,
		}
		respJSON, err := json.Marshal(resp)
		if err != nil {
			log.Printf("Error marshaling response: %v", err)
			continue
		}
		n, err := conn.Write(append(respJSON, '\n'))
		if err != nil {
			log.Printf("Error writing response: %v", err)
		} else {
			log.Printf("Sent response to %s: %d bytes (id=%d)", conn.RemoteAddr(), n, req.ID)
		}
	}

	log.Printf("API client disconnected: %s", conn.RemoteAddr())
}

// sendAPIError sends an error response to API client
func (sm *SessionManager) sendAPIError(conn net.Conn, id int64, code int, message string) {
	resp := map[string]interface{}{
		"jsonrpc": "2.0",
		"id":      id,
		"error": map[string]interface{}{
			"code":    code,
			"message": message,
		},
	}
	respJSON, _ := json.Marshal(resp)
	conn.Write(append(respJSON, '\n'))
}

// handleAPIRequest handles an API request from mcp-lsp-bridge
func (sm *SessionManager) handleAPIRequest(method string, params json.RawMessage) (interface{}, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
	defer cancel()

	switch method {
	case "session/status":
		return sm.getStatus(), nil

	case "session/capabilities":
		sm.mu.RLock()
		caps := sm.capabilities
		sm.mu.RUnlock()
		return caps, nil

	case "textDocument/didOpen":
		return sm.handleDidOpen(params)

	case "textDocument/didClose":
		return sm.handleDidClose(params)

	case "textDocument/hover",
		"textDocument/definition",
		"textDocument/references",
		"textDocument/documentSymbol",
		"textDocument/diagnostic",
		"textDocument/implementation",
		"textDocument/codeAction",
		"textDocument/formatting",
		"textDocument/rename",
		"textDocument/prepareRename",
		"textDocument/prepareCallHierarchy":
		// Forward directly to LSP server
		var p interface{}
		json.Unmarshal(params, &p)
		return sm.sendRequest(ctx, method, p)

	case "callHierarchy/incomingCalls",
		"callHierarchy/outgoingCalls":
		var p interface{}
		json.Unmarshal(params, &p)
		return sm.sendRequest(ctx, method, p)

	case "workspace/symbol":
		var p interface{}
		json.Unmarshal(params, &p)
		return sm.sendRequest(ctx, method, p)

	default:
		return nil, fmt.Errorf("unknown method: %s", method)
	}
}

// getStatus returns current session status
func (sm *SessionManager) getStatus() map[string]interface{} {
	sm.mu.RLock()
	initialized := sm.initialized
	sm.mu.RUnlock()

	sm.openDocsMu.Lock()
	openDocsCount := len(sm.openDocs)
	sm.openDocsMu.Unlock()

	return map[string]interface{}{
		"initialized":   initialized,
		"openDocuments": openDocsCount,
		"pid":           sm.cmd.Process.Pid,
	}
}

// handleDidOpen handles textDocument/didOpen
func (sm *SessionManager) handleDidOpen(params json.RawMessage) (interface{}, error) {
	var p struct {
		TextDocument struct {
			URI        string `json:"uri"`
			LanguageID string `json:"languageId"`
			Version    int    `json:"version"`
			Text       string `json:"text"`
		} `json:"textDocument"`
	}

	if err := json.Unmarshal(params, &p); err != nil {
		return nil, err
	}

	sm.openDocsMu.Lock()
	alreadyOpen := sm.openDocs[p.TextDocument.URI]
	sm.openDocs[p.TextDocument.URI] = true
	sm.openDocsMu.Unlock()

	if alreadyOpen {
		// Document already open, no need to send again
		return map[string]interface{}{"status": "already_open"}, nil
	}

	// Send to LSP server
	return nil, sm.sendNotification("textDocument/didOpen", p)
}

// handleDidClose handles textDocument/didClose
func (sm *SessionManager) handleDidClose(params json.RawMessage) (interface{}, error) {
	var p struct {
		TextDocument struct {
			URI string `json:"uri"`
		} `json:"textDocument"`
	}

	if err := json.Unmarshal(params, &p); err != nil {
		return nil, err
	}

	sm.openDocsMu.Lock()
	delete(sm.openDocs, p.TextDocument.URI)
	sm.openDocsMu.Unlock()

	return nil, sm.sendNotification("textDocument/didClose", p)
}

// readLSPMessage reads a complete LSP message
func readLSPMessage(reader *bufio.Reader) ([]byte, error) {
	var contentLength int

	for {
		line, err := reader.ReadString('\n')
		if err != nil {
			return nil, err
		}

		line = strings.TrimSpace(line)
		if line == "" {
			break
		}

		if strings.HasPrefix(line, "Content-Length:") {
			lengthStr := strings.TrimSpace(strings.TrimPrefix(line, "Content-Length:"))
			contentLength, err = strconv.Atoi(lengthStr)
			if err != nil {
				return nil, fmt.Errorf("invalid Content-Length: %v", err)
			}
		}
	}

	if contentLength == 0 {
		return nil, fmt.Errorf("missing Content-Length header")
	}

	body := make([]byte, contentLength)
	_, err := io.ReadFull(reader, body)
	if err != nil {
		return nil, err
	}

	return body, nil
}
