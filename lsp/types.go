package lsp

import (
	"context"
	"os/exec"
	"sync"
	"time"

	"github.com/myleshyson/lsprotocol-go/protocol"
	"github.com/sourcegraph/jsonrpc2"
)

// ClientStatus represents the current status of a language client
type ClientStatus int

const (
	StatusUninitialized ClientStatus = iota
	StatusConnecting
	StatusConnected
	StatusError
	StatusRestarting
)

// LanguageClient wraps a Language Server Protocol client connection
type LanguageClient struct {
	mu                 sync.RWMutex
	conn               *jsonrpc2.Conn
	ctx                context.Context
	cancel             context.CancelFunc
	cmd                *exec.Cmd
	clientCapabilities protocol.ClientCapabilities
	serverCapabilities protocol.ServerCapabilities

	// Connection management
	command            string
	args               []string
	processID          int32
	isConnected        bool
	lastInitialized    time.Time
	status             ClientStatus
	lastError          error

	// Metrics
	totalRequests      int64
	successfulRequests int64
	failedRequests     int64
	lastErrorTime      time.Time

	// Configuration
	maxConnectionAttempts int
	connectionTimeout     time.Duration
	idleTimeout          time.Duration
	restartDelay         time.Duration
}

// LanguageServerConfig defines the configuration for a specific language server
type LanguageServerConfig struct {
	Command               string         `json:"command"`
	Args                  []string       `json:"args"`
	Languages             []string       `json:"languages"`
	Filetypes             []string       `json:"filetypes"`
	InitializationOptions map[string]any `json:"initialization_options"`
}

// LSPServerConfig represents the complete configuration for language servers
type LSPServerConfig struct {
	LanguageServers map[string]LanguageServerConfig `json:"language_servers"`
	Global          struct {
		LogLevel           string `json:"log_level"`
		MaxRestartAttempts int    `json:"max_restart_attempts"`
		RestartDelayMs     int    `json:"restart_delay_ms"`
	} `json:"global"`
	LanguageExtensionMap map[string][]string `json:"language_extension_map"`
	ExtensionLanguageMap map[string]string   `json:"extension_language_map"`
}