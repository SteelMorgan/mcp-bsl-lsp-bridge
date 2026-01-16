package lsp

import (
	"context"
	"fmt"
	"net"
	"os"
	"strings"
	"time"

	"rockerboo/mcp-lsp-bridge/logger"

	"github.com/sourcegraph/jsonrpc2"
)

// NewTCPLanguageClient creates a new TCP-based Language Server Protocol client.
// This connects to an LSP server running behind lsp-proxy daemon.
func NewTCPLanguageClient(host string, port int) (*LanguageClient, error) {
	if host == "" {
		host = "localhost"
	}
	if port <= 0 {
		port = 9999
	}

	client := &LanguageClient{
		command: fmt.Sprintf("tcp://%s:%d", host, port),
		args:    []string{},

		maxConnectionAttempts: 5,
		connectionTimeout:     30 * time.Second,
		idleTimeout:           30 * time.Minute,
		restartDelay:          2 * time.Second,

		status:     StatusConnecting,
		tcpAddress: fmt.Sprintf("%s:%d", host, port),
	}

	return client, nil
}

// ConnectTCP establishes a TCP connection to the LSP proxy server.
func (lc *LanguageClient) ConnectTCP() (*LanguageClient, error) {
	if lc.tcpAddress == "" {
		return nil, fmt.Errorf("TCP address not configured")
	}

	// Replace localhost with 127.0.0.1 to avoid DNS issues in containers
	addr := strings.Replace(lc.tcpAddress, "localhost", "127.0.0.1", 1)

	logger.Info(fmt.Sprintf("ConnectTCP: Connecting to LSP proxy at %s", addr))

	// Retry connection with backoff
	var conn net.Conn
	var err error

	for attempt := 1; attempt <= lc.maxConnectionAttempts; attempt++ {
		conn, err = net.DialTimeout("tcp", addr, 10*time.Second)
		if err == nil {
			break
		}

		logger.Warn(fmt.Sprintf("TCP connection attempt %d/%d failed: %v",
			attempt, lc.maxConnectionAttempts, err))

		if attempt < lc.maxConnectionAttempts {
			time.Sleep(lc.restartDelay * time.Duration(attempt))
		}
	}

	if err != nil {
		return nil, fmt.Errorf("failed to connect to LSP proxy at %s after %d attempts: %w",
			addr, lc.maxConnectionAttempts, err)
	}

	// Set TCP keepalive
	if tcpConn, ok := conn.(*net.TCPConn); ok {
		tcpConn.SetKeepAlive(true)
		tcpConn.SetKeepAlivePeriod(30 * time.Second)
		tcpConn.SetNoDelay(true)
	}

	logger.Info(fmt.Sprintf("TCP connection established to %s", addr))
	fmt.Fprintf(os.Stderr, "DEBUG TCP: connection established to %s\n", addr)

	// Create cancellable context
	ctx, cancel := context.WithCancel(context.Background())
	lc.ctx = ctx
	lc.cancel = cancel

	// Create handler for LSP notifications
	if lc.progress == nil {
		lc.progress = NewProgressTracker()
	}
	handler := &ClientHandler{
		progress: lc.progress,
	}

	fmt.Fprintf(os.Stderr, "DEBUG TCP: creating JSON-RPC stream...\n")

	// Create JSON-RPC stream over TCP connection
	// LSP uses Content-Length headers, which VSCodeObjectCodec handles
	stream := jsonrpc2.NewBufferedStream(conn, jsonrpc2.VSCodeObjectCodec{})

	fmt.Fprintf(os.Stderr, "DEBUG TCP: creating JSON-RPC connection...\n")

	jsonrpcLogger := &JSONRPCLogger{}
	rpcConn := jsonrpc2.NewConn(ctx, stream, handler,
		jsonrpc2.LogMessages(jsonrpcLogger),
		jsonrpc2.SetLogger(jsonrpcLogger))

	fmt.Fprintf(os.Stderr, "DEBUG TCP: JSON-RPC connection created\n")

	// Check if connection is already closed
	select {
	case <-rpcConn.DisconnectNotify():
		fmt.Fprintf(os.Stderr, "DEBUG TCP: Connection already disconnected!\n")
		return nil, fmt.Errorf("connection closed immediately after creation")
	default:
		fmt.Fprintf(os.Stderr, "DEBUG TCP: Connection still alive\n")
	}

	// Monitor connection disconnects
	go func() {
		fmt.Fprintf(os.Stderr, "DEBUG TCP: Monitor goroutine started\n")
		disconnectCh := rpcConn.DisconnectNotify()
		select {
		case <-disconnectCh:
			logger.Error("DISCONNECT: TCP connection to LSP proxy was disconnected")
			fmt.Fprintf(os.Stderr, "DEBUG TCP: DISCONNECT notified! Connection closed unexpectedly\n")
			lc.status = StatusDisconnected
		case <-ctx.Done():
			logger.Debug("DISCONNECT: Context cancelled for TCP connection")
			fmt.Fprintf(os.Stderr, "DEBUG TCP: Context cancelled reason=%v\n", ctx.Err())
		}
		fmt.Fprintf(os.Stderr, "DEBUG TCP: Monitor goroutine exiting\n")
	}()

	fmt.Fprintf(os.Stderr, "DEBUG TCP: Setting lc.conn...\n")
	lc.conn = rpcConn
	lc.status = StatusConnected
	lc.lastInitialized = time.Now()

	logger.Info("Successfully connected to LSP server via TCP proxy")
	fmt.Fprintf(os.Stderr, "DEBUG TCP: ConnectTCP completed successfully\n")

	return lc, nil
}
