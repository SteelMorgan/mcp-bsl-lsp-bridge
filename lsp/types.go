package lsp

import (
	"context"
	"net"
	"os/exec"
	"sync"
	"time"

	"rockerboo/mcp-lsp-bridge/types"

	"github.com/myleshyson/lsprotocol-go/protocol"
)

// Language represents a programming language
type Language string

// GlobalConfig holds global configuration options
type GlobalConfig struct {
	LogPath            string `json:"log_file_path"`
	LogLevel           string `json:"log_level"`
	MaxLogFiles        int    `json:"max_log_files"`
	MaxRestartAttempts int    `json:"max_restart_attempts"`
	RestartDelayMs     int    `json:"restart_delay_ms"`
}

// LanguageServerConfig represents configuration for a single language server
type LanguageServerConfig struct {
	Command               string                 `json:"command"`
	Args                  []string               `json:"args"`
	Languages             []string               `json:"languages,omitempty"`
	Filetypes             []string               `json:"filetypes"`
	InitializationOptions map[string]interface{} `json:"initialization_options,omitempty"`
	
	// WebSocket mode configuration (alternative to command/args)
	Mode string `json:"mode,omitempty"` // "stdio" (default) or "websocket"
	Host string `json:"host,omitempty"` // WebSocket host (e.g., "bsl-ls" or "localhost")
	Port int    `json:"port,omitempty"` // WebSocket port (e.g., 9999)
}

// GetCommand implements types.LanguageServerConfigProvider
func (c *LanguageServerConfig) GetCommand() string {
	return c.Command
}

// GetArgs implements types.LanguageServerConfigProvider
func (c *LanguageServerConfig) GetArgs() []string {
	return c.Args
}

// GetInitializationOptions implements types.LanguageServerConfigProvider
func (c *LanguageServerConfig) GetInitializationOptions() map[string]interface{} {
	return c.InitializationOptions
}

// GetMode implements types.LanguageServerConfigProvider
func (c *LanguageServerConfig) GetMode() string {
	if c.Mode == "" {
		return "stdio"
	}
	return c.Mode
}

// GetHost implements types.LanguageServerConfigProvider
func (c *LanguageServerConfig) GetHost() string {
	return c.Host
}

// GetPort implements types.LanguageServerConfigProvider
func (c *LanguageServerConfig) GetPort() int {
	return c.Port
}

// IsWebSocketMode implements types.LanguageServerConfigProvider
func (c *LanguageServerConfig) IsWebSocketMode() bool {
	return c.Mode == "websocket"
}

// IsTCPMode implements types.LanguageServerConfigProvider
func (c *LanguageServerConfig) IsTCPMode() bool {
	return c.Mode == "tcp"
}

// IsSessionMode returns true if mode is "session" (LSP Session Manager)
func (c *LanguageServerConfig) IsSessionMode() bool {
	return c.Mode == "session"
}

// LSPServerConfig represents the complete LSP server configuration
type LSPServerConfig struct {
	Global               GlobalConfig                                  `json:"global"`
	LanguageServers      map[types.LanguageServer]LanguageServerConfig `json:"language_servers"`
	LanguageServerMap    map[types.LanguageServer][]types.Language     `json:"language_server_map,omitempty"`
	ExtensionLanguageMap map[string]types.Language                     `json:"extension_language_map,omitempty"`
}

// LanguageClient wraps a Language Server Protocol client connection
type LanguageClient struct {
	mu                 sync.RWMutex
	conn               types.LSPConnectionInterface
	ctx                context.Context
	cancel             context.CancelFunc
	cmd                *exec.Cmd
	clientCapabilities protocol.ClientCapabilities
	serverCapabilities protocol.ServerCapabilities

	tokenParser types.SemanticTokensParserProvider
	progress    *ProgressTracker

	workspacePaths []string

	// Connection management
	command         string
	args            []string
	processID       int32
	lastInitialized time.Time
	status          ClientStatus
	lastError       error

	// TCP/Socket mode
	tcpAddress string   // For TCP mode: "host:port"
	tcpConn    net.Conn // Active TCP connection

	// Metrics
	totalRequests      int64
	successfulRequests int64
	failedRequests     int64
	lastErrorTime      time.Time

	// Configuration
	maxConnectionAttempts int
	connectionTimeout     time.Duration
	idleTimeout           time.Duration
	restartDelay          time.Duration
}
