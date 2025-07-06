package lsp

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"sync/atomic"
	"syscall"
	"time"

	"rockerboo/mcp-lsp-bridge/logger"
	"rockerboo/mcp-lsp-bridge/types"

	"github.com/myleshyson/lsprotocol-go/protocol"
	"github.com/sourcegraph/jsonrpc2"
)

// JSONRPCLogger implements jsonrpc2.Logger interface
type JSONRPCLogger struct{}

func (l *JSONRPCLogger) Printf(format string, args ...interface{}) {
	logger.Debug(fmt.Sprintf("JSONRPC: "+format, args...))
}

// NewLanguageClient creates a new Language Server Protocol client
func NewLanguageClient(command string, args ...string) (*LanguageClient, error) {

	err := sanitizeArgs(args)
	if err != nil {
		return nil, err
	}

	client := LanguageClient{
		command:            command,
		args:               args,
		clientCapabilities: protocol.ClientCapabilities{},
		serverCapabilities: protocol.ServerCapabilities{},

		// Default configuration
		maxConnectionAttempts: 3,
		connectionTimeout:     10 * time.Second,
		idleTimeout:           30 * time.Minute,
		restartDelay:          1 * time.Second,

		status: StatusConnecting,
	}

	return &client, nil
}

func sanitizeArgs(args []string) error {
	for _, arg := range args {
		// Block shell metacharacters that could be dangerous if LSP server processes them
		if strings.ContainsAny(arg, ";|&$`") {
			return fmt.Errorf("dangerous character in argument: %s", arg)
		}
		// Block command substitution patterns
		if strings.Contains(arg, "$(") || strings.Contains(arg, "`") {
			return fmt.Errorf("command substitution not allowed: %s", arg)
		}
	}
	return nil
}

func (lc *LanguageClient) Connect() (types.LanguageClientInterface, error) {
	// Log the LSP server connection attempt
	logger.Info(fmt.Sprintf("Connecting to LSP server: %s %v", lc.command, lc.args))

	var conn types.LSPConnectionInterface

	// Create cancellable context for the entire session
	ctx, cancel := context.WithCancel(context.Background())

	// Start the external process
	cmd := exec.CommandContext(ctx, lc.command, lc.args...) // #nosec G204

	// CRITICAL: Set up the command to run in a clean environment
	// This prevents terminal echo and other interactive behavior
	cmd.Env = append(os.Environ(),
		"TERM=dumb",  // Prevent terminal control sequences
		"NO_COLOR=1", // Disable color output
	)

	// IMPORTANT: Put language server in its own process group so it doesn't get killed
	// when the parent receives SIGINT
	cmd.SysProcAttr = &syscall.SysProcAttr{
		Setpgid: true, // Create new process group
		Pgid:    0,    // 0 means use the PID as PGID
	}

	stdin, err := cmd.StdinPipe()
	if err != nil {
		cancel()
		return nil, fmt.Errorf("failed to create stdin pipe: %w", err)
	}

	stdout, err := cmd.StdoutPipe()
	if err != nil {
		closeErr := stdin.Close()
		cancel()

		if closeErr != nil {
			return nil, fmt.Errorf("failed to close stdin pipe: %w", closeErr)
		}

		return nil, fmt.Errorf("failed to create stdout pipe: %w", err)
	}

	stderr, err := cmd.StderrPipe()
	if err != nil {
		stdCloseErr := stdin.Close()
		if stdCloseErr != nil {
			cancel()
			return nil, fmt.Errorf("failed to close stdin pipe: %w", stdCloseErr)
		}

		stdOutClose := stdout.Close()
		if stdOutClose != nil {
			cancel()
			return nil, fmt.Errorf("failed to close stdout pipe: %w", stdOutClose)
		}

		cancel()

		return nil, fmt.Errorf("failed to create stderr pipe: %w", err)
	}

	// Start the process
	if err := cmd.Start(); err != nil {
		stdinCloseErr := stdin.Close()
		if stdinCloseErr != nil {
			cancel()
			return nil, fmt.Errorf("failed to close stdin pipe: %w", stdinCloseErr)
		}

		stdoutCloseErr := stdout.Close()
		if stdoutCloseErr != nil {
			cancel()
			return nil, fmt.Errorf("failed to close stdout pipe: %w", stdoutCloseErr)
		}

		stderrCloseErr := stderr.Close()
		if stderrCloseErr != nil {
			cancel()
			return nil, fmt.Errorf("failed to close stderr pipe: %w", stderrCloseErr)
		}

		cancel()

		return nil, fmt.Errorf("failed to start command: %w", err)
	}

	// Create a ReadWriteCloser that combines stdin and stdout for LSP
	readWriteCloser := &stdioReadWriteCloser{
		stdin:  stdin,
		stdout: stdout,
	}

	// Create handler
	handler := &ClientHandler{}

	// Create JSON-RPC connection using VSCode Object Codec for LSP headers
	stream := jsonrpc2.NewBufferedStream(readWriteCloser, jsonrpc2.VSCodeObjectCodec{})
	logger.Debug(fmt.Sprintf("STATUS: About to create jsonrpc2.NewConn with ctx.Err()=%v", ctx.Err()))

	// Add JSON-RPC message logging
	jsonrpcLogger := &JSONRPCLogger{}
	conn = jsonrpc2.NewConn(ctx, stream, handler,
		jsonrpc2.LogMessages(jsonrpcLogger),
		jsonrpc2.SetLogger(jsonrpcLogger))

	// Check connection status immediately
	select {
	case <-conn.DisconnectNotify():
		logger.Error("STATUS: Connection already disconnected immediately after creation!")
	default:
		logger.Debug("STATUS: Connection appears healthy immediately after creation")
	}

	// Monitor connection disconnects
	go func() {
		disconnectCh := conn.DisconnectNotify()
		select {
		case <-disconnectCh:
			logger.Error(fmt.Sprintf("DISCONNECT: Connection to %s was disconnected unexpectedly", lc.command))
		case <-ctx.Done():
			logger.Debug(fmt.Sprintf("DISCONNECT: Context cancelled for %s: %v", lc.command, ctx.Err()))
		}
	}()

	logger.Debug(fmt.Sprintf("Successfully started LSP server: %v", lc.conn))

	lc.conn = conn
	lc.status = StatusConnected
	lc.lastInitialized = time.Now()
	lc.ctx = ctx
	lc.cancel = cancel

	// Check connection status after assignment
	select {
	case <-lc.conn.DisconnectNotify():
		logger.Error("STATUS: Connection disconnected after assignment to client!")
	default:
		logger.Debug(fmt.Sprintf("STATUS: Connection healthy after assignment, ctx.Err()=%v", lc.ctx.Err()))
	}

	// Log successful LSP server connection
	logger.Info(fmt.Sprintf("Successfully connected to LSP server: %s %v", lc.command, lc.args))

	// Handle stderr in background
	go func() {
		buf := make([]byte, 1024)

		for {
			n, err := stderr.Read(buf)
			if err != nil {
				logger.Error(fmt.Sprintf("STDERR: Error reading from %s stderr: %v", lc.command, err))
				break
			}

			if n > 0 {
				logger.Debug(fmt.Sprintf("[%s SERVER STDERR]: %s", lc.command, buf[:n]))
			}
		}

		stderrCloseErr := stderr.Close()
		if stderrCloseErr != nil {
			logger.Warn(fmt.Errorf("failed to close stderr pipe: %w", stderrCloseErr))
		}
	}()

	// Monitor process status in background
	go func() {
		if lc.cmd != nil && lc.cmd.Process != nil {
			logger.Debug(fmt.Sprintf("PROCESS: Started %s with PID %d", lc.command, lc.cmd.Process.Pid))

			// Wait for process to exit and log the result
			err := lc.cmd.Wait()
			if err != nil {
				logger.Error(fmt.Sprintf("PROCESS: %s (PID %d) exited with error: %v", lc.command, lc.cmd.Process.Pid, err))
			} else {
				logger.Debug(fmt.Sprintf("PROCESS: %s (PID %d) exited successfully", lc.command, lc.cmd.Process.Pid))
			}
		}
	}()

	return lc, nil
}

func (ls *LanguageClient) IsConnected() bool {
	return ls.status == StatusConnected
}

func (ls *LanguageClient) Status() int {
	return int(ls.status)
}

func (lc *LanguageClient) ProjectRoots() []string {
	return lc.workspacePaths
}
func (lc *LanguageClient) SetProjectRoots(paths []string) {
	lc.workspacePaths = paths
}

// Close closes the language client and cleans up resources
func (lc *LanguageClient) Close() error {
	lc.mu.Lock()
	defer lc.mu.Unlock()

	// Close JSON-RPC connection
	if lc.conn != nil {
		err := lc.conn.Close()
		if err != nil {
			return err
		}
	}

	// Cancel context
	lc.cancel()

	// Attempt graceful shutdown of language server
	if lc.cmd != nil && lc.cmd.Process != nil {
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
			_ = lc.cmd.Process.Kill()

			<-done // Wait for it to actually exit
		}
	}

	// // Attempt graceful shutdown of language server process
	// if lc.cmd != nil && lc.cmd.Process != nil {
	// 	// Wait for process to exit or kill it
	// 	done := make(chan error, 1)
	// 	go func() {
	// 		done <- lc.cmd.Wait()
	// 	}()
	//
	// 	select {
	// 	case waitErr := <-done:
	// 		// Process exited, check if there was an error
	// 		if waitErr != nil {
	// 			errors = append(errors, fmt.Errorf("process wait failed: %w", waitErr))
	// 		}
	// 	case <-time.After(2 * time.Second):
	// 		// Force kill if it doesn't exit
	// 		if lc.cmd.Process != nil {
	// 			if killErr := lc.cmd.Process.Kill(); killErr != nil {
	// 				errors = append(errors, fmt.Errorf("failed to kill process: %w", killErr))
	// 			}
	// 			// Wait for it to actually exit after kill
	// 			waitErr := <-done
	// 			if waitErr != nil {
	// 				errors = append(errors, fmt.Errorf("process wait after kill failed: %w", waitErr))
	// 			}
	// 		}
	// 	}
	// 	lc.cmd = nil
	// }

	// Reset connection state
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
	// Increment total requests
	atomic.AddInt64(&lc.totalRequests, 1)

	// Ensure connection is still valid by checking context and connection
	if lc.ctx.Err() != nil || lc.conn == nil {
		return errors.New("language server connection is closed")
	}

	// Reset status to connected if we have a valid connection
	if lc.status == StatusError && lc.ctx.Err() == nil && lc.conn != nil {
		lc.status = StatusConnected

		logger.Info("LSP client status reset from error to connected")
	}

	// Debug the parent context state
	logger.Debug(fmt.Sprintf("SendRequest: Parent context error: %v", lc.ctx.Err()))

	p, e := json.Marshal(params)
	if e != nil {
		return fmt.Errorf("failed to marshal initialize params: %w", e)
	}

	logger.Debug(fmt.Sprintf("LSP Request: method=%s params=%s", method, p))

	// Check connection state before call
	select {
	case <-lc.conn.DisconnectNotify():
		logger.Error("DISCONNECT: Connection already disconnected before Call")
		return errors.New("connection already disconnected")
	default:
		logger.Debug("DISCONNECT: Connection appears healthy before Call")
	}

	reqCtx, cancel := context.WithTimeout(lc.ctx, timeout)
	defer cancel()

	logger.Debug("DISCONNECT: About to make jsonrpc2.Call")
	// Call WITHOUT holding any locks to avoid deadlock
	err := lc.conn.Call(reqCtx, method, params, result)
	logger.Debug(fmt.Sprintf("DISCONNECT: jsonrpc2.Call completed with error: %v", err))

	// Update status and metrics with brief locks
	if err != nil {
		// Increment failed requests
		atomic.AddInt64(&lc.failedRequests, 1)

		lc.mu.Lock()
		lc.lastErrorTime = time.Now()
		lc.lastError = err
		lc.status = StatusError
		lc.mu.Unlock()

		// Log the error
		logger.Error(fmt.Sprintf("LSP Request Error: method=%s, error=%v", method, err))
	} else {
		// Increment successful requests
		atomic.AddInt64(&lc.successfulRequests, 1)

		// Reset status to connected if we had an error before
		lc.mu.Lock()
		if lc.status == StatusError {
			lc.status = StatusConnected
			logger.Info("LSP client status reset from error to connected")
		}
		lc.mu.Unlock()
	}

	return err
}

// SendNotification sends a notification
func (lc *LanguageClient) SendNotification(method string, params any) error {
	if len(method) == 0 {
		return errors.New("empty notification method")
	}

	return lc.conn.Notify(lc.ctx, method, params)
}

// Context returns the client's context
func (lc *LanguageClient) Context() context.Context {
	return lc.ctx
}

// GetMetrics returns the current metrics for the language client
func (lc *LanguageClient) GetMetrics() types.ClientMetricsProvider {
	lc.mu.RLock()
	defer lc.mu.RUnlock()

	return &ClientMetrics{
		Command:            lc.command,
		Status:             lc.status.Status(),
		TotalRequests:      atomic.LoadInt64(&lc.totalRequests),
		SuccessfulRequests: atomic.LoadInt64(&lc.successfulRequests),
		FailedRequests:     atomic.LoadInt64(&lc.failedRequests),
		LastInitialized:    lc.lastInitialized,
		LastErrorTime:      lc.lastErrorTime,
		LastError:          fmt.Sprintf("%v", lc.lastError),
		Connected:          lc.IsConnected(),
		ProcessID:          lc.processID,
	}
}

func (lc *LanguageClient) SetupSemanticTokens() error {
	tokenTypes, tokenModifiers, err := GetTokenTypeFromServerCapabilities(&lc.serverCapabilities)
	if err != nil {
		logger.Warn(fmt.Sprintf("SetupSemanticTokens: Failed to get token types from server capabilities: %v", err))
		return err
	}

	logger.Debug(fmt.Sprintf("SetupSemanticTokens: Token Types: %+v, Token Modifiers: %+v", tokenTypes, tokenModifiers))

	lc.tokenParser = NewSemanticTokenParser(tokenTypes, tokenModifiers)

	return nil
}

func (lc *LanguageClient) TokenParser() types.SemanticTokensParserProvider {
	return lc.tokenParser
}
