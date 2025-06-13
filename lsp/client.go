package lsp

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"sync/atomic"
	"time"

	"github.com/myleshyson/lsprotocol-go/protocol"
	"github.com/sourcegraph/jsonrpc2"
)

// NewLanguageClient creates a new Language Server Protocol client
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
		command:            command,
		args:               args,
		processID:          int32(cmd.Process.Pid),
		clientCapabilities: protocol.ClientCapabilities{},
		serverCapabilities: protocol.ServerCapabilities{},

		// Default configuration
		maxConnectionAttempts: 3,
		connectionTimeout:     10 * time.Second,
		idleTimeout:          30 * time.Minute,
		restartDelay:         1 * time.Second,

		status: StatusConnecting,
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
	client.isConnected = true
	client.status = StatusConnected
	client.lastInitialized = time.Now()

	// Handle stderr in background
	go func() {
		buf := make([]byte, 1024)
		for {
			n, err := stderr.Read(buf)
			if err != nil {
				break
			}
			if n > 0 {
				fmt.Fprintf(os.Stderr, "[%s SERVER STDERR]: %s", command, buf[:n])
			}
		}
		stderr.Close()
	}()

	return client, nil
}

// Close closes the language client and cleans up resources
func (lc *LanguageClient) Close() error {
	lc.mu.Lock()
	defer lc.mu.Unlock()

	// Close JSON-RPC connection
	if lc.conn != nil {
		lc.conn.Close()
	}

	// Cancel context
	lc.cancel()

	// Attempt graceful shutdown of language server
	if lc.cmd != nil && lc.cmd.Process != nil {
		// Only send shutdown/exit for real LSP servers, not echo
		if lc.command != "echo" {
			// Send shutdown notification
			shutdownErr := lc.Shutdown()
			if shutdownErr != nil {
				fmt.Printf("Failed to send shutdown: %v", shutdownErr)
			}

			// Send exit notification
			exitErr := lc.Exit()
			if exitErr != nil {
				fmt.Printf("Failed to send exit: %v", exitErr)
			}
		}

		// Wait for process to exit or kill it
		done := make(chan error, 1)
		go func() {
			done <- lc.cmd.Wait()
		}()

		select {
		case <-done:
			// Process exited gracefully
		case <-time.After(2 * time.Second):
			// Force kill if it doesn't exit
			lc.cmd.Process.Kill()
			<-done // Wait for it to actually exit
		}
	}

	// Reset connection state
	lc.isConnected = false
	lc.status = StatusUninitialized
	lc.lastError = nil

	return nil
}

// ClientCapabilities returns the client's capabilities
func (lc *LanguageClient) ClientCapabilities() protocol.ClientCapabilities {
	return lc.clientCapabilities
}

// ServerCapabilities returns the server's capabilities
func (lc *LanguageClient) ServerCapabilities() protocol.ServerCapabilities {
	return lc.serverCapabilities
}

// SetServerCapabilities sets the server's capabilities
func (lc *LanguageClient) SetServerCapabilities(capabilities protocol.ServerCapabilities) {
	lc.serverCapabilities = capabilities
}

// SendRequest sends a request with timeout
func (lc *LanguageClient) SendRequest(method string, params any, result any, timeout time.Duration) error {
	lc.mu.Lock()
	defer lc.mu.Unlock()

	// Increment total requests
	atomic.AddInt64(&lc.totalRequests, 1)

	// Ensure connection is still valid
	if !lc.isConnected || lc.status != StatusConnected {
		return fmt.Errorf("language server not connected")
	}

	reqCtx, cancel := context.WithTimeout(lc.ctx, timeout)
	defer cancel()

	err := lc.conn.Call(reqCtx, method, params, result)
	if err != nil {
		// Increment failed requests
		atomic.AddInt64(&lc.failedRequests, 1)
		lc.lastErrorTime = time.Now()
		lc.lastError = err
		lc.status = StatusError

		// Log the error
		fmt.Printf("LSP Request Error: method=%s, error=%v", method, err)
	} else {
		// Increment successful requests
		atomic.AddInt64(&lc.successfulRequests, 1)
	}

	return err
}

// SendRequestNoTimeout sends a request without timeout
func (lc *LanguageClient) SendRequestNoTimeout(method string, params any, result any) error {
	return lc.conn.Call(lc.ctx, method, params, result)
}

// SendNotification sends a notification
func (lc *LanguageClient) SendNotification(method string, params any) error {
	return lc.conn.Notify(lc.ctx, method, params)
}

// Context returns the client's context
func (lc *LanguageClient) Context() context.Context {
	return lc.ctx
}

// GetMetrics returns the current metrics for the language client
func (lc *LanguageClient) GetMetrics() map[string]interface{} {
	lc.mu.RLock()
	defer lc.mu.RUnlock()

	return map[string]interface{}{
		"command":               lc.command,
		"status":               lc.status,
		"total_requests":       atomic.LoadInt64(&lc.totalRequests),
		"successful_requests":  atomic.LoadInt64(&lc.successfulRequests),
		"failed_requests":      atomic.LoadInt64(&lc.failedRequests),
		"last_initialized":     lc.lastInitialized,
		"last_error_time":      lc.lastErrorTime,
		"last_error":           lc.lastError,
		"is_connected":         lc.isConnected,
		"process_id":           lc.processID,
	}
}