// Session Adapter - Adapts SessionClient to LanguageClientInterface
//
// This adapter wraps SessionClient to implement the LanguageClientInterface,
// allowing mcp-lsp-bridge to use Session Manager transparently.

package lsp

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"rockerboo/mcp-lsp-bridge/logger"
	"rockerboo/mcp-lsp-bridge/types"

	"github.com/myleshyson/lsprotocol-go/protocol"
)

// SessionAdapter adapts SessionClient to LanguageClientInterface
type SessionAdapter struct {
	client       *SessionClient
	projectRoots []string
	connected    bool
	lastError    string
}

// NewSessionAdapter creates a new session adapter
func NewSessionAdapter(host string, port int) (*SessionAdapter, error) {
	client := NewSessionClient(host, port)

	return &SessionAdapter{
		client: client,
	}, nil
}

// Connect connects to Session Manager
func (sa *SessionAdapter) Connect() (types.LanguageClientInterface, error) {
	if err := sa.client.Connect(); err != nil {
		sa.lastError = err.Error()
		return nil, err
	}
	sa.connected = true
	return sa, nil
}

// Close closes the connection
func (sa *SessionAdapter) Close() error {
	sa.connected = false
	return sa.client.Close()
}

// IsConnected returns connection status
func (sa *SessionAdapter) IsConnected() bool {
	return sa.connected && sa.client.IsConnected()
}

// Context returns a background context (Session Manager manages its own context)
func (sa *SessionAdapter) Context() context.Context {
	return context.Background()
}

// SetProjectRoots sets the project roots
func (sa *SessionAdapter) SetProjectRoots(roots []string) {
	sa.projectRoots = roots
}

// GetProjectRoots returns the project roots
func (sa *SessionAdapter) GetProjectRoots() []string {
	return sa.projectRoots
}

// Initialize - Session Manager is already initialized, just return success
func (sa *SessionAdapter) Initialize(params protocol.InitializeParams) (*protocol.InitializeResult, error) {
	logger.Debug("SessionAdapter: Initialize called - Session Manager already initialized")

	// Get capabilities from Session Manager
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	status, err := sa.client.GetStatus(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to get session status: %w", err)
	}

	initialized, ok := status["initialized"].(bool)
	if !ok || !initialized {
		return nil, fmt.Errorf("Session Manager not initialized")
	}

	// Return minimal result - actual capabilities are in Session Manager
	// We return an empty capabilities struct - the bridge doesn't really use this
	return &protocol.InitializeResult{
		Capabilities: protocol.ServerCapabilities{},
	}, nil
}

// Initialized - no-op for Session Manager
func (sa *SessionAdapter) Initialized() error {
	logger.Debug("SessionAdapter: Initialized notification - no-op for Session Manager")
	return nil
}

// Shutdown - no-op for Session Manager (it keeps running)
func (sa *SessionAdapter) Shutdown() error {
	logger.Debug("SessionAdapter: Shutdown - no-op for Session Manager")
	return nil
}

// Exit - no-op for Session Manager
func (sa *SessionAdapter) Exit() error {
	logger.Debug("SessionAdapter: Exit - no-op for Session Manager")
	return nil
}

// DidOpen opens a document
func (sa *SessionAdapter) DidOpen(uri string, languageId protocol.LanguageKind, text string, version int32) error {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	return sa.client.DidOpen(ctx, uri, string(languageId), text)
}

// DidChange - not implemented yet
func (sa *SessionAdapter) DidChange(uri string, version int32, changes []protocol.TextDocumentContentChangeEvent) error {
	// TODO: implement if needed
	return nil
}

// DidSave - not implemented yet
func (sa *SessionAdapter) DidSave(uri string, text *string) error {
	// TODO: implement if needed
	return nil
}

// DidClose closes a document
func (sa *SessionAdapter) DidClose(uri string) error {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	return sa.client.DidClose(ctx, uri)
}

// Hover gets hover information
func (sa *SessionAdapter) Hover(uri string, line, character uint32) (*protocol.Hover, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	result, err := sa.client.Hover(ctx, uri, line, character)
	if err != nil {
		return nil, err
	}

	if result == nil || string(result) == "null" {
		return nil, nil
	}

	var hover protocol.Hover
	if err := json.Unmarshal(result, &hover); err != nil {
		return nil, fmt.Errorf("failed to unmarshal hover: %w", err)
	}

	return &hover, nil
}

// Definition gets definition locations
func (sa *SessionAdapter) Definition(uri string, line, character uint32) ([]protocol.Or2[protocol.LocationLink, protocol.Location], error) {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	result, err := sa.client.Definition(ctx, uri, line, character)
	if err != nil {
		return nil, err
	}

	if result == nil || string(result) == "null" {
		return nil, nil
	}

	var locations []protocol.Or2[protocol.LocationLink, protocol.Location]
	if err := json.Unmarshal(result, &locations); err != nil {
		return nil, fmt.Errorf("failed to unmarshal definition: %w", err)
	}

	return locations, nil
}

// References finds all references
func (sa *SessionAdapter) References(uri string, line, character uint32, includeDeclaration bool) ([]protocol.Location, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	result, err := sa.client.References(ctx, uri, line, character, includeDeclaration)
	if err != nil {
		return nil, err
	}

	if result == nil || string(result) == "null" {
		return nil, nil
	}

	var locations []protocol.Location
	if err := json.Unmarshal(result, &locations); err != nil {
		return nil, fmt.Errorf("failed to unmarshal references: %w", err)
	}

	return locations, nil
}

// DocumentSymbols gets document symbols
func (sa *SessionAdapter) DocumentSymbols(uri string) ([]protocol.DocumentSymbol, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	result, err := sa.client.DocumentSymbols(ctx, uri)
	if err != nil {
		return nil, err
	}

	if result == nil || string(result) == "null" {
		return nil, nil
	}

	var symbols []protocol.DocumentSymbol
	if err := json.Unmarshal(result, &symbols); err != nil {
		return nil, fmt.Errorf("failed to unmarshal document symbols: %w", err)
	}

	return symbols, nil
}

// WorkspaceSymbols searches for symbols
func (sa *SessionAdapter) WorkspaceSymbols(query string) ([]protocol.WorkspaceSymbol, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	result, err := sa.client.WorkspaceSymbol(ctx, query)
	if err != nil {
		return nil, err
	}

	if result == nil || string(result) == "null" {
		return nil, nil
	}

	var symbols []protocol.WorkspaceSymbol
	if err := json.Unmarshal(result, &symbols); err != nil {
		return nil, fmt.Errorf("failed to unmarshal workspace symbols: %w", err)
	}

	return symbols, nil
}

// PrepareCallHierarchy prepares call hierarchy
func (sa *SessionAdapter) PrepareCallHierarchy(uri string, line, character uint32) ([]protocol.CallHierarchyItem, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	result, err := sa.client.PrepareCallHierarchy(ctx, uri, line, character)
	if err != nil {
		return nil, err
	}

	if result == nil || string(result) == "null" {
		return nil, nil
	}

	var items []protocol.CallHierarchyItem
	if err := json.Unmarshal(result, &items); err != nil {
		return nil, fmt.Errorf("failed to unmarshal call hierarchy items: %w", err)
	}

	return items, nil
}

// IncomingCalls gets incoming calls
func (sa *SessionAdapter) IncomingCalls(item protocol.CallHierarchyItem) ([]protocol.CallHierarchyIncomingCall, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 120*time.Second)
	defer cancel()

	itemJSON, err := json.Marshal(item)
	if err != nil {
		return nil, err
	}

	result, err := sa.client.IncomingCalls(ctx, itemJSON)
	if err != nil {
		return nil, err
	}

	if result == nil || string(result) == "null" {
		return nil, nil
	}

	var calls []protocol.CallHierarchyIncomingCall
	if err := json.Unmarshal(result, &calls); err != nil {
		return nil, fmt.Errorf("failed to unmarshal incoming calls: %w", err)
	}

	return calls, nil
}

// OutgoingCalls gets outgoing calls
func (sa *SessionAdapter) OutgoingCalls(item protocol.CallHierarchyItem) ([]protocol.CallHierarchyOutgoingCall, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 120*time.Second)
	defer cancel()

	itemJSON, err := json.Marshal(item)
	if err != nil {
		return nil, err
	}

	result, err := sa.client.OutgoingCalls(ctx, itemJSON)
	if err != nil {
		return nil, err
	}

	if result == nil || string(result) == "null" {
		return nil, nil
	}

	var calls []protocol.CallHierarchyOutgoingCall
	if err := json.Unmarshal(result, &calls); err != nil {
		return nil, fmt.Errorf("failed to unmarshal outgoing calls: %w", err)
	}

	return calls, nil
}

// Implementation finds implementations
func (sa *SessionAdapter) Implementation(uri string, line, character uint32) ([]protocol.Location, error) {
	// Forward as definition for now - BSL doesn't really have interfaces
	return sa.References(uri, line, character, true)
}

// SignatureHelp - not implemented yet
func (sa *SessionAdapter) SignatureHelp(uri string, line, character uint32) (*protocol.SignatureHelp, error) {
	return nil, nil
}

// CodeActions - not implemented yet
func (sa *SessionAdapter) CodeActions(uri string, line, character, endLine, endCharacter uint32) ([]protocol.CodeAction, error) {
	return nil, nil
}

// Rename - not implemented yet
func (sa *SessionAdapter) Rename(uri string, line, character uint32, newName string) (*protocol.WorkspaceEdit, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()

	result, err := sa.client.Rename(ctx, uri, line, character, newName)
	if err != nil {
		return nil, err
	}
	if result == nil || string(result) == "null" {
		return nil, nil
	}

	var edit protocol.WorkspaceEdit
	if err := json.Unmarshal(result, &edit); err != nil {
		return nil, fmt.Errorf("failed to unmarshal rename result: %w", err)
	}
	return &edit, nil
}

// Formatting - not implemented yet
func (sa *SessionAdapter) Formatting(uri string, tabSize uint32, insertSpaces bool) ([]protocol.TextEdit, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()

	result, err := sa.client.Formatting(ctx, uri, tabSize, insertSpaces)
	if err != nil {
		return nil, err
	}
	if result == nil || string(result) == "null" {
		return nil, nil
	}

	var edits []protocol.TextEdit
	if err := json.Unmarshal(result, &edits); err != nil {
		return nil, fmt.Errorf("failed to unmarshal formatting edits: %w", err)
	}
	return edits, nil
}

// RangeFormatting - not implemented yet
func (sa *SessionAdapter) RangeFormatting(uri string, startLine, startCharacter, endLine, endCharacter uint32, tabSize uint32, insertSpaces bool) ([]protocol.TextEdit, error) {
	return nil, fmt.Errorf("range formatting not implemented in session mode")
}

// WorkspaceDiagnostic - not implemented yet
func (sa *SessionAdapter) WorkspaceDiagnostic(identifier string) (*protocol.WorkspaceDiagnosticReport, error) {
	// Workspace diagnostics can be extremely heavy on BSL projects (10k LOC modules, 20k+ files).
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Minute)
	defer cancel()

	result, err := sa.client.WorkspaceDiagnostic(ctx, identifier)
	if err != nil {
		return nil, err
	}
	if result == nil || string(result) == "null" {
		return nil, nil
	}

	var report protocol.WorkspaceDiagnosticReport
	if err := json.Unmarshal(result, &report); err != nil {
		return nil, fmt.Errorf("failed to unmarshal workspace diagnostic report: %w", err)
	}
	return &report, nil
}

// DocumentDiagnostics gets diagnostics for a document
func (sa *SessionAdapter) DocumentDiagnostics(uri string, identifier string, previousResultId string) (*protocol.DocumentDiagnosticReport, error) {
	// Document diagnostics can be slow on large BSL workspaces.
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()

	result, err := sa.client.Diagnostic(ctx, uri, identifier, previousResultId)
	if err != nil {
		return nil, err
	}

	if result == nil || string(result) == "null" {
		return nil, nil
	}

	var report protocol.DocumentDiagnosticReport
	if err := json.Unmarshal(result, &report); err != nil {
		return nil, fmt.Errorf("failed to unmarshal diagnostic: %w", err)
	}

	return &report, nil
}

// SemanticTokens - not implemented
func (sa *SessionAdapter) SemanticTokens(uri string) (*protocol.SemanticTokens, error) {
	return nil, nil
}

// SemanticTokensRange - not implemented
func (sa *SessionAdapter) SemanticTokensRange(uri string, startLine, startCharacter, endLine, endCharacter uint32) (*protocol.SemanticTokens, error) {
	return nil, nil
}

// PrepareRename - not implemented yet
func (sa *SessionAdapter) PrepareRename(uri string, line, character uint32) (*protocol.PrepareRenameResult, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()

	result, err := sa.client.PrepareRename(ctx, uri, line, character)
	if err != nil {
		return nil, err
	}
	if result == nil || string(result) == "null" {
		return nil, nil
	}

	var pr protocol.PrepareRenameResult
	if err := json.Unmarshal(result, &pr); err != nil {
		return nil, fmt.Errorf("failed to unmarshal prepareRename result: %w", err)
	}
	return &pr, nil
}

// FoldingRange - not implemented yet
func (sa *SessionAdapter) FoldingRange(uri string) ([]protocol.FoldingRange, error) {
	return nil, nil
}

// SelectionRange - not implemented yet
func (sa *SessionAdapter) SelectionRange(uri string, positions []protocol.Position) ([]protocol.SelectionRange, error) {
	return nil, nil
}

// DocumentLink - not implemented yet
func (sa *SessionAdapter) DocumentLink(uri string) ([]protocol.DocumentLink, error) {
	return nil, nil
}

// DocumentColor - not implemented yet
func (sa *SessionAdapter) DocumentColor(uri string) ([]protocol.ColorInformation, error) {
	return nil, nil
}

// ColorPresentation - not implemented yet
func (sa *SessionAdapter) ColorPresentation(uri string, color protocol.Color, rng protocol.Range) ([]protocol.ColorPresentation, error) {
	return nil, nil
}

// ExecuteCommand - not implemented yet
func (sa *SessionAdapter) ExecuteCommand(command string, args []any) (json.RawMessage, error) {
	return nil, fmt.Errorf("execute command not implemented in session mode")
}

// SendRequest sends a raw request (for compatibility)
func (sa *SessionAdapter) SendRequest(method string, params any, result any, timeout time.Duration) error {
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()
	return sa.client.Call(ctx, method, params, result)
}

// SendNotification sends a notification (for compatibility)
func (sa *SessionAdapter) SendNotification(method string, params any) error {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	var result interface{}
	return sa.client.Call(ctx, method, params, &result)
}

// GetStatus returns connection status
func (sa *SessionAdapter) GetStatus() (connected bool, status string, lastError string) {
	if sa.connected && sa.client.IsConnected() {
		return true, "connected", ""
	}
	return false, "disconnected", sa.lastError
}

// Connect method for LanguageClientInterface compatibility
func (sa *SessionAdapter) ConnectInterface() (interface{}, error) {
	return sa.Connect()
}

// GetMetrics returns client metrics (stub for now)
func (sa *SessionAdapter) GetMetrics() types.ClientMetricsProvider {
	status := int(StatusUninitialized)
	if sa.connected && sa.client.IsConnected() {
		status = int(StatusConnected)
	} else if !sa.connected {
		status = int(StatusDisconnected)
	}
	return &sessionMetrics{connected: sa.connected, status: status}
}

// Status returns connection status as int
func (sa *SessionAdapter) Status() int {
	if sa.connected {
		return 1
	}
	return 0
}

// ProjectRoots returns project roots
func (sa *SessionAdapter) ProjectRoots() []string {
	return sa.projectRoots
}

// ClientCapabilities returns client capabilities
func (sa *SessionAdapter) ClientCapabilities() protocol.ClientCapabilities {
	return protocol.ClientCapabilities{}
}

// ServerCapabilities returns server capabilities
func (sa *SessionAdapter) ServerCapabilities() protocol.ServerCapabilities {
	return protocol.ServerCapabilities{}
}

// SetServerCapabilities sets server capabilities (no-op for session mode)
func (sa *SessionAdapter) SetServerCapabilities(capabilities protocol.ServerCapabilities) {
	// No-op - Session Manager handles this
}

// SetupSemanticTokens sets up semantic tokens (no-op)
func (sa *SessionAdapter) SetupSemanticTokens() error {
	return nil
}

// TokenParser returns semantic token parser (nil for now)
func (sa *SessionAdapter) TokenParser() types.SemanticTokensParserProvider {
	return nil
}

// sessionMetrics implements ClientMetricsProvider for SessionAdapter
type sessionMetrics struct {
	connected bool
	command   string
	status    int
}

func (m *sessionMetrics) GetCommand() string                     { return m.command }
func (m *sessionMetrics) SetCommand(command string)              { m.command = command }
func (m *sessionMetrics) GetStatus() int                         { return m.status }
func (m *sessionMetrics) SetStatus(status int)                   { m.status = status }
func (m *sessionMetrics) GetTotalRequests() int64                { return 0 }
func (m *sessionMetrics) SetTotalRequests(total int64)           {}
func (m *sessionMetrics) IncrementTotalRequests()                {}
func (m *sessionMetrics) GetSuccessfulRequests() int64           { return 0 }
func (m *sessionMetrics) SetSuccessfulRequests(successful int64) {}
func (m *sessionMetrics) IncrementSuccessfulRequests()           {}
func (m *sessionMetrics) GetFailedRequests() int64               { return 0 }
func (m *sessionMetrics) SetFailedRequests(failed int64)         {}
func (m *sessionMetrics) IncrementFailedRequests()               {}
func (m *sessionMetrics) GetLastInitialized() time.Time          { return time.Time{} }
func (m *sessionMetrics) SetLastInitialized(t time.Time)         {}
func (m *sessionMetrics) GetLastErrorTime() time.Time            { return time.Time{} }
func (m *sessionMetrics) SetLastErrorTime(t time.Time)           {}
func (m *sessionMetrics) GetLastError() string                   { return "" }
func (m *sessionMetrics) SetLastError(err string)                {}
func (m *sessionMetrics) IsConnected() bool                      { return m.connected }
func (m *sessionMetrics) SetConnected(connected bool)            { m.connected = connected }
func (m *sessionMetrics) GetProcessID() int32                    { return 0 }
func (m *sessionMetrics) SetProcessID(pid int32)                 {}

// DidChangeWatchedFiles notifies about file changes
func (sa *SessionAdapter) DidChangeWatchedFiles(changes []protocol.FileEvent) error {
	params := protocol.DidChangeWatchedFilesParams{
		Changes: changes,
	}
	return sa.SendNotification("workspace/didChangeWatchedFiles", params)
}

// DidChangeConfiguration notifies about config changes (no-op)
func (sa *SessionAdapter) DidChangeConfiguration(settings any) error {
	return nil
}

// IndexingStatus represents the current indexing progress from session manager (minimal)
type IndexingStatus struct {
	State          string `json:"state"` // "idle" | "indexing" | "complete"
	Current        int    `json:"current"`
	Total          int    `json:"total"`
	ETASeconds     int    `json:"eta_seconds,omitempty"`
	ElapsedSeconds int    `json:"elapsed_seconds,omitempty"`
	Message        string `json:"message,omitempty"`
}

// GetSessionStatus returns the full session status including indexing progress
func (sa *SessionAdapter) GetSessionStatus(ctx context.Context) (map[string]interface{}, error) {
	return sa.client.GetStatus(ctx)
}

// GetIndexingStatus returns the current indexing progress
func (sa *SessionAdapter) GetIndexingStatus() *IndexingStatus {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	status, err := sa.client.GetStatus(ctx)
	if err != nil {
		return nil
	}

	indexingData, ok := status["indexing"].(map[string]interface{})
	if !ok {
		return nil
	}

	result := &IndexingStatus{State: "idle"}

	if v, ok := indexingData["state"].(string); ok {
		result.State = v
	}
	if v, ok := indexingData["current"].(float64); ok {
		result.Current = int(v)
	}
	if v, ok := indexingData["total"].(float64); ok {
		result.Total = int(v)
	}
	if v, ok := indexingData["eta_seconds"].(float64); ok {
		result.ETASeconds = int(v)
	}
	if v, ok := indexingData["elapsed_seconds"].(float64); ok {
		result.ElapsedSeconds = int(v)
	}
	if v, ok := indexingData["message"].(string); ok {
		result.Message = v
	}

	return result
}
