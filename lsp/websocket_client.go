package lsp

import (
	"context"
	"fmt"
	"io"
	"net"
	"net/http"
	"strings"
	"sync"
	"time"

	"rockerboo/mcp-lsp-bridge/logger"

	"github.com/gorilla/websocket"
	"github.com/sourcegraph/jsonrpc2"
)

// NewWebSocketLanguageClient creates a new WebSocket-based Language Server Protocol client.
func NewWebSocketLanguageClient(host string, port int) (*LanguageClient, error) {
	if host == "" {
		host = "localhost"
	}
	if port <= 0 {
		port = 9999
	}

	client := &LanguageClient{
		command: fmt.Sprintf("ws://%s:%d/lsp", host, port),
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

// ConnectWebSocket establishes a WebSocket connection to the LSP server.
func (lc *LanguageClient) ConnectWebSocket() (*LanguageClient, error) {
	if lc.tcpAddress == "" {
		return nil, fmt.Errorf("WebSocket address not configured")
	}

	// Replace localhost with 127.0.0.1 to avoid DNS issues
	addr := strings.Replace(lc.tcpAddress, "localhost", "127.0.0.1", 1)
	wsURL := fmt.Sprintf("ws://%s/lsp", addr)

	logger.Info(fmt.Sprintf("ConnectWebSocket: Starting connection to: %s", wsURL))

	// Retry connection with backoff
	var wsConn *websocket.Conn
	var err error

	for attempt := 1; attempt <= lc.maxConnectionAttempts; attempt++ {
		wsConn, err = dialGorillaWebSocket(wsURL)
		if err == nil {
			break
		}

		logger.Warn(fmt.Sprintf("WebSocket connection attempt %d/%d failed: %v",
			attempt, lc.maxConnectionAttempts, err))

		if attempt < lc.maxConnectionAttempts {
			time.Sleep(lc.restartDelay * time.Duration(attempt))
		}
	}

	if err != nil {
		return nil, fmt.Errorf("failed to connect to LSP server at %s after %d attempts: %w",
			wsURL, lc.maxConnectionAttempts, err)
	}

	logger.Info(fmt.Sprintf("WebSocket connection established to %s", wsURL))

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

	// Wrap gorilla websocket for jsonrpc2
	rwc := newGorillaRWC(wsConn)
	stream := jsonrpc2.NewBufferedStream(rwc, jsonrpc2.VSCodeObjectCodec{})

	jsonrpcLogger := &JSONRPCLogger{}
	rpcConn := jsonrpc2.NewConn(ctx, stream, handler,
		jsonrpc2.LogMessages(jsonrpcLogger),
		jsonrpc2.SetLogger(jsonrpcLogger))

	// Monitor connection disconnects
	go func() {
		disconnectCh := rpcConn.DisconnectNotify()
		select {
		case <-disconnectCh:
			logger.Error("DISCONNECT: WebSocket connection was disconnected")
			lc.status = StatusDisconnected
		case <-ctx.Done():
			logger.Debug("DISCONNECT: Context cancelled")
		}
	}()

	lc.conn = rpcConn
	lc.status = StatusConnected
	lc.lastInitialized = time.Now()

	logger.Info("Successfully connected to LSP server via WebSocket")

	return lc, nil
}

func dialGorillaWebSocket(wsURL string) (*websocket.Conn, error) {

	// Create a custom dialer with TCP settings
	netDialer := &net.Dialer{
		Timeout:   10 * time.Second,
		KeepAlive: 30 * time.Second,
	}

	dialer := websocket.Dialer{
		NetDial: func(network, addr string) (net.Conn, error) {
			conn, err := netDialer.Dial(network, addr)
			if err != nil {
				return nil, err
			}
			// Set TCP_NODELAY for immediate packet sending
			if tcpConn, ok := conn.(*net.TCPConn); ok {
				tcpConn.SetNoDelay(true)
			}
			return conn, nil
		},
		HandshakeTimeout: 45 * time.Second,
		ReadBufferSize:   4096,
		WriteBufferSize:  4096,
	}
	conn, resp, err := dialer.Dial(wsURL, http.Header{})
	if err != nil {
		if resp != nil {
		} else {
		}
		return nil, err
	}
	return conn, nil
}

// gorillaRWC wraps gorilla/websocket for io.ReadWriteCloser
type gorillaRWC struct {
	conn    *websocket.Conn
	readBuf []byte
	mu      sync.Mutex
}

func newGorillaRWC(conn *websocket.Conn) *gorillaRWC {
	return &gorillaRWC{conn: conn}
}

func (g *gorillaRWC) Read(p []byte) (int, error) {
	g.mu.Lock()
	defer g.mu.Unlock()

	// Return buffered data first
	if len(g.readBuf) > 0 {
		n := copy(p, g.readBuf)
		g.readBuf = g.readBuf[n:]
		return n, nil
	}

	// Read next message
	_, msg, err := g.conn.ReadMessage()
	if err != nil {
		return 0, err
	}

	n := copy(p, msg)
	if n < len(msg) {
		g.readBuf = msg[n:]
	}
	return n, nil
}

func (g *gorillaRWC) Write(p []byte) (int, error) {
	err := g.conn.WriteMessage(websocket.TextMessage, p)
	if err != nil {
		return 0, err
	}
	return len(p), nil
}

func (g *gorillaRWC) Close() error {
	return g.conn.Close()
}

var _ io.ReadWriteCloser = (*gorillaRWC)(nil)
