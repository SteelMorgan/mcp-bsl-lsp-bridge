package types

import (
	"context"
	"time"

	"github.com/myleshyson/lsprotocol-go/protocol"
	"github.com/sourcegraph/jsonrpc2"
)

type TokenPosition struct {
	TokenType string
	Text      string // The actual text if available
	Range     protocol.Range
}

type SemanticTokensParserProvider interface {
	FindTokensByType(
		tokens *protocol.SemanticTokens,
		targetTypes []string,
		baseRange protocol.Range,
	) ([]TokenPosition, error)

	// FindFunctionNames finds all function/method names in the semantic tokens
	FindFunctionNames(
		tokens *protocol.SemanticTokens,
		baseRange protocol.Range,
	) ([]TokenPosition, error)
	// FindParameters finds all parameters in the semantic tokens
	FindParameters(
		tokens *protocol.SemanticTokens,
		baseRange protocol.Range,
	) ([]TokenPosition, error)
	// FindVariables finds all variables in the semantic tokens
	FindVariables(
		tokens *protocol.SemanticTokens,
		baseRange protocol.Range,
	) ([]TokenPosition, error)
	// FindTypes finds all type references in the semantic tokens
	FindTypes(
		tokens *protocol.SemanticTokens,
		baseRange protocol.Range,
	) ([]TokenPosition, error)

	SemanticTokensCapabilitiesProvider
}

type SemanticTokensCapabilitiesProvider interface {
	TokenTypes() []string
	TokenModifiers() []string
}

type LanguageServerConfigProvider interface {
	GetCommand() string
	GetArgs() []string
	GetInitializationOptions() map[string]interface{}
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
	GetMetrics() ClientMetricsProvider
	IsConnected() bool
	Status() int
	ProjectRoots() []string
	SetProjectRoots(paths []string)

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
	TokenParser() SemanticTokensParserProvider

	// Text document synchronization
	DidOpen(uri string, languageId protocol.LanguageKind, text string, version int32) error
	DidChange(uri string, version int32, changes []protocol.TextDocumentContentChangeEvent) error
	DidSave(uri string, text *string) error
	DidClose(uri string) error

	// Language features
	WorkspaceSymbols(query string) ([]protocol.WorkspaceSymbol, error)
	CodeActions(uri string, line, character, endLine, endCharacter uint32) ([]protocol.CodeAction, error)
	Formatting(uri string, tabSize uint32, insertSpaces bool) ([]protocol.TextEdit, error)
	Rename(uri string, line, character uint32, newName string) (*protocol.WorkspaceEdit, error)
	Definition(uri string, line, character uint32) ([]protocol.Or2[protocol.LocationLink, protocol.Location], error)
	WorkspaceDiagnostic(identifier string) (*protocol.WorkspaceDiagnosticReport, error)
	PrepareCallHierarchy(uri string, line, character uint32) ([]protocol.CallHierarchyItem, error)
	IncomingCalls(item protocol.CallHierarchyItem) ([]protocol.CallHierarchyIncomingCall, error)
	OutgoingCalls(item protocol.CallHierarchyItem) ([]protocol.CallHierarchyOutgoingCall, error)
	References(uri string, line, character uint32, includeDeclaration bool) ([]protocol.Location, error)
	Hover(uri string, line, character uint32) (*protocol.Hover, error)
	DocumentSymbols(uri string) ([]protocol.DocumentSymbol, error)
	Implementation(uri string, line, character uint32) ([]protocol.Location, error)
	SignatureHelp(uri string, line, character uint32) (*protocol.SignatureHelp, error)
	SemanticTokens(uri string) (*protocol.SemanticTokens, error)
	SemanticTokensRange(uri string, startLine, startCharacter, endLine, endCharacter uint32) (*protocol.SemanticTokens, error)
	DocumentDiagnostics(uri string, identifier string, previousResultId string) (*protocol.DocumentDiagnosticReport, error)
}

type LSPConnectionInterface interface {
	Call(ctx context.Context, method string, params, result any, opts ...jsonrpc2.CallOption) error
	Notify(ctx context.Context, method string, params any, opts ...jsonrpc2.CallOption) error
	Reply(ctx context.Context, id jsonrpc2.ID, result any) error
	Close() error
	DisconnectNotify() <-chan struct{}
}

type LSPProcessInterface interface {
	Deadline() (time.Time, bool)
	Done() <-chan struct{}
	Err() error
	Value(key any) any
}

type Language string
type LanguageServer string

type GlobalConfig struct {
	LogPath            string `json:"log_file_path"`
	LogLevel           string `json:"log_level"`
	MaxLogFiles        int    `json:"max_log_files"`
	MaxRestartAttempts int    `json:"max_restart_attempts"`
	RestartDelayMs     int    `json:"restart_delay_ms"`
}

type LSPServerConfigProvider interface {
	FindServerConfig(language string) (LanguageServerConfigProvider, error)
	FindAllServerConfigs(language string) ([]LanguageServerConfigProvider, []LanguageServer, error)
	GetGlobalConfig() GlobalConfig
	GetLanguageServers() map[LanguageServer]LanguageServerConfigProvider
	GetServerNameFromLanguage(language Language) LanguageServer

	LanguageDetector
}

type LanguageDetector interface {
	FindExtLanguage(ext string) (*Language, error)
	DetectProjectLanguages(projectPath string) ([]Language, error)
	DetectPrimaryProjectLanguage(projectPath string) (*Language, error)
}
