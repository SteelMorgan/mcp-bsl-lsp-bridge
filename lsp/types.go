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
	conn               LSPConnectionInterface
	ctx                LSPProcessInterface
	cancel             context.CancelFunc
	cmd                *exec.Cmd
	clientCapabilities protocol.ClientCapabilities
	serverCapabilities protocol.ServerCapabilities

	tokenParser *SemanticTokenParser

	// Connection management
	command         string
	args            []string
	processID       int32
	lastInitialized time.Time
	status          ClientStatus
	lastError       error

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

// LanguageServerConfig defines the configuration for a specific language server
type LanguageServerConfig struct {
	Command               string         `json:"command"`
	Args                  []string       `json:"args"`
	Languages             []string       `json:"languages"`
	Filetypes             []string       `json:"filetypes"`
	InitializationOptions map[string]any `json:"initialization_options"`
}

// LanguageClientInterface defines the methods required for a language client.
// This interface abstracts the concrete LanguageClient type for better testability.
type LanguageClientInterface interface {
	// Core client methods
	Connect() (LanguageClientInterface, error)
	SendRequest(method string, params any, result any, timeout time.Duration) error
	SendNotification(method string, params any) error
	Close() error
	Context() context.Context
	GetMetrics() ClientMetrics
	IsConnected() bool
	Status() ClientStatus

	// Lifecycle methods
	Initialize(params protocol.InitializeParams) (*protocol.InitializeResult, error)
	Initialized() error
	Shutdown() error
	Exit() error

	// Capabilities
	ClientCapabilities() protocol.ClientCapabilities
	ServerCapabilities() protocol.ServerCapabilities
	SetServerCapabilities(capabilities protocol.ServerCapabilities)
	SetupSemanticTokens() error
	TokenParser() *SemanticTokenParser

	// Text document synchronization
	DidOpen(uri string, languageId protocol.LanguageKind, text string, version int32) error
	DidChange(uri string, version int32, changes []protocol.TextDocumentContentChangeEvent) error
	DidSave(uri string, text *string) error
	DidClose(uri string) error

	// Language features
	WorkspaceSymbols(query string) ([]protocol.WorkspaceSymbol, error)
	Definition(uri string, line, character uint32) ([]protocol.Or2[protocol.LocationLink, protocol.Location], error)
	References(uri string, line, character uint32, includeDeclaration bool) ([]protocol.Location, error)
	Hover(uri string, line, character uint32) (*protocol.Hover, error)
	DocumentSymbols(uri string) ([]protocol.DocumentSymbol, error)
	Implementation(uri string, line, character uint32) ([]protocol.Location, error)
	SignatureHelp(uri string, line, character uint32) (*protocol.SignatureHelp, error)
	SemanticTokens(uri string) (*protocol.SemanticTokens, error)
	SemanticTokensRange(uri string, startLine, startCharacter, endLine, endCharacter uint32) (*protocol.SemanticTokens, error)
}

type LSPConnectionInterface interface {
	Call(ctx context.Context, method string, params, result any, opts ...jsonrpc2.CallOption) error
	Notify(ctx context.Context, method string, params any, opts ...jsonrpc2.CallOption) error
	Reply(ctx context.Context, id jsonrpc2.ID, result any) error
	Close() error
}

type LSPProcessInterface interface {
	Deadline() (time.Time, bool)
	Done() <-chan struct{}
	Err() error
	Value(key any) any
}

type Language string

type ClientMetrics struct {
	Command            string       `json:"command"`
	Status             ClientStatus `json:"status"`
	TotalRequests      int64        `json:"total_requests"`
	SuccessfulRequests int64        `json:"successful_requests"`
	FailedRequests     int64        `json:"failed_requests"`
	LastInitialized    time.Time    `json:"last_initialized"`
	LastErrorTime      time.Time    `json:"last_error_time"`
	LastError          string       `json:"last_error,omitempty"`
	IsConnected        bool         `json:"is_connected"`
	ProcessID          int32        `json:"process_id"`
}

// LSPServerConfig represents the complete configuration for language servers
type LSPServerConfig struct {
	LanguageServers map[Language]LanguageServerConfig `json:"language_servers"`
	Global          struct {
		LogPath            string `json:"log_file_path"`
		LogLevel           string `json:"log_level"`
		MaxLogFiles        int    `json:"max_log_files"`
		MaxRestartAttempts int    `json:"max_restart_attempts"`
		RestartDelayMs     int    `json:"restart_delay_ms"`
	} `json:"global"`
	LanguageExtensionMap map[Language][]string `json:"language_extension_map"`
	ExtensionLanguageMap map[string]Language   `json:"extension_language_map"`
}
