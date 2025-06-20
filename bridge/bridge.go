package bridge

import (
	"fmt"
	"os"
	"path/filepath"
	"time"

	"rockerboo/mcp-lsp-bridge/logger"
	"rockerboo/mcp-lsp-bridge/lsp"

	"github.com/mark3labs/mcp-go/server"
	"github.com/myleshyson/lsprotocol-go/protocol"
)

// NewMCPLSPBridge creates a new bridge instance with provided configuration
func NewMCPLSPBridge(config *lsp.LSPServerConfig) *MCPLSPBridge {
	bridge := &MCPLSPBridge{
		clients: make(map[string]*lsp.LanguageClient),
		config:  config,
	}
	return bridge
}

// GetClientForLanguage retrieves or creates a language server client for a specific language
// ConnectionAttemptConfig defines retry parameters for language server connections
type ConnectionAttemptConfig struct {
	MaxRetries   int
	RetryDelay   time.Duration
	TotalTimeout time.Duration
}

// DefaultConnectionConfig provides a default configuration for connection attempts
func DefaultConnectionConfig() ConnectionAttemptConfig {
	return ConnectionAttemptConfig{
		MaxRetries:   3,
		RetryDelay:   2 * time.Second,
		TotalTimeout: 30 * time.Second,
	}
}

// validateAndConnectClient attempts to validate and establish a language server connection
func (b *MCPLSPBridge) validateAndConnectClient(language string, serverConfig lsp.LanguageServerConfig, config ConnectionAttemptConfig) (*lsp.LanguageClient, error) {
	// Attempt connection with retry mechanism
	var lastErr error
	startTime := time.Now()

	for attempt := range config.MaxRetries {
		// Check if total timeout exceeded
		if time.Since(startTime) > config.TotalTimeout {
			break
		}

		// Create language client
		var lc *lsp.LanguageClient
		var err error
		lc, err = lsp.NewLanguageClient(serverConfig.Command, serverConfig.Args...)

		if err != nil {
			lastErr = fmt.Errorf("failed to create language client on attempt %d: %w", attempt+1, err)
			time.Sleep(config.RetryDelay)
			continue
		}

		// Get current working directory
		dir, err := os.Getwd()
		if err != nil {
			lastErr = fmt.Errorf("failed to get current directory: %w", err)
			lc.Close()
			time.Sleep(config.RetryDelay)
			continue
		}

		root_uri := protocol.DocumentUri(fmt.Sprintf("file://%s", dir))
		process_id := int32(os.Getpid())

		// Prepare initialization parameters
		params := protocol.InitializeParams{
			ProcessId: &process_id,
			ClientInfo: &protocol.ClientInfo{
				Name:    "MCP-LSP Bridge",
				Version: "1.0.0",
			},
			RootUri:      &root_uri,
			Capabilities: lc.ClientCapabilities(),
		}

		// Apply any initialization options from the configuration
		if serverConfig.InitializationOptions != nil {
			params.InitializationOptions = serverConfig.InitializationOptions
		}

		// Send initialize request
		var result protocol.InitializeResult
		err = lc.SendRequest("initialize", params, &result, 10*time.Second)
		if err != nil {
			lastErr = fmt.Errorf("initialize request failed on attempt %d: %w", attempt+1, err)
			lc.Close()
			time.Sleep(config.RetryDelay)
			continue
		}

		// Set server capabilities
		lc.SetServerCapabilities(result.Capabilities)

		// Log server info and capabilities
		if result.ServerInfo != nil {
			logger.Info(fmt.Sprintf("Initialize result - Server Info: %+v", *result.ServerInfo))
		}
		logger.Info(fmt.Sprintf("Initialize result - Capabilities: %+v", result.Capabilities))

		// Send initialized notification
		err = lc.SendNotification("initialized", map[string]any{})
		if err != nil {
			lastErr = fmt.Errorf("failed to send initialized notification on attempt %d: %w", attempt+1, err)
			lc.Close()
			time.Sleep(config.RetryDelay)
			continue
		}

		// Successfully connected
		return lc, nil
	}

	return nil, fmt.Errorf("failed to establish language server connection for %s after %d attempts: %w",
		language, config.MaxRetries, lastErr)
}

func (b *MCPLSPBridge) GetClientForLanguage(language string) (*lsp.LanguageClient, error) {
	// Check if client already exists and is connected
	if existingClient, exists := b.clients[language]; exists {
		// Additional check to ensure client is still valid
		metrics := existingClient.GetMetrics()
		if metrics != nil && metrics["is_connected"] == true {
			return existingClient, nil
		}
	}

	// Find the server configuration
	serverConfig, exists := b.config.LanguageServers[language]
	if !exists {
		return nil, fmt.Errorf("no server configuration found for language %s", language)
	}

	// Attempt to connect with default configuration
	lc, err := b.validateAndConnectClient(language, serverConfig, DefaultConnectionConfig())
	if err != nil {
		return nil, err
	}

	// Store the new client
	b.clients[language] = lc

	return lc, nil
}

// CloseAllClients closes all active language server clients
func (b *MCPLSPBridge) CloseAllClients() {
	for _, client := range b.clients {
		client.Close()
	}
	b.clients = make(map[string]*lsp.LanguageClient)
}

// InferLanguage infers the programming language from a file path
func (b *MCPLSPBridge) InferLanguage(filePath string) (string, error) {
	ext := filepath.Ext(filePath)
	language, exists := b.config.ExtensionLanguageMap[ext]
	if !exists {
		return "", fmt.Errorf("no language found for extension %s", ext)
	}
	return language, nil
}

// GetConfig returns the bridge's configuration
func (b *MCPLSPBridge) GetConfig() *lsp.LSPServerConfig {
	return b.config
}

// GetServer returns the bridge's MCP server
func (b *MCPLSPBridge) GetServer() *server.MCPServer {
	return b.server
}

// SetServer sets the bridge's MCP server
func (b *MCPLSPBridge) SetServer(mcpServer *server.MCPServer) {
	b.server = mcpServer
}

// GetClientForLanguageInterface returns a client as interface{} for tool compatibility
func (b *MCPLSPBridge) GetClientForLanguageInterface(language string) (any, error) {
	return b.GetClientForLanguage(language)
}

// DetectProjectLanguages detects all languages used in a project directory
func (b *MCPLSPBridge) DetectProjectLanguages(projectPath string) ([]string, error) {
	if b.config == nil {
		return nil, fmt.Errorf("no LSP configuration available")
	}
	return b.config.DetectProjectLanguages(projectPath)
}

// DetectPrimaryProjectLanguage detects the primary language of a project
func (b *MCPLSPBridge) DetectPrimaryProjectLanguage(projectPath string) (string, error) {
	if b.config == nil {
		return "", fmt.Errorf("no LSP configuration available")
	}
	return b.config.DetectPrimaryProjectLanguage(projectPath)
}

// FindSymbolReferences finds all references to a symbol at a given position
func (b *MCPLSPBridge) FindSymbolReferences(language, uri string, line, character int32, includeDeclaration bool) ([]any, error) {
	client, err := b.GetClientForLanguage(language)
	if err != nil {
		return nil, fmt.Errorf("failed to get client for language %s: %w", language, err)
	}

	references, err := client.References(uri, line, character, includeDeclaration)
	if err != nil {
		return nil, fmt.Errorf("failed to find references: %w", err)
	}

	// Convert to []any for interface compatibility
	result := make([]any, len(references))
	for i, ref := range references {
		result[i] = ref
	}
	return result, nil
}

// FindSymbolDefinitions finds all definitions for a symbol at a given position
func (b *MCPLSPBridge) FindSymbolDefinitions(language, uri string, line, character int32) ([]any, error) {
	client, err := b.GetClientForLanguage(language)
	if err != nil {
		return nil, fmt.Errorf("failed to get client for language %s: %w", language, err)
	}

	definitions, err := client.Definition(uri, line, character)
	if err != nil {
		// Log the error but return empty results instead of failing
		logger.Warn(fmt.Sprintf("Failed to find definitions for %s at %s:%d:%d: %v", language, uri, line, character, err))
		return []any{}, nil
	}

	// Convert to []any for interface compatibility
	result := make([]any, len(definitions))
	for i, def := range definitions {
		result[i] = def
	}
	return result, nil
}

// SearchTextInWorkspace performs a text search across the workspace
func (b *MCPLSPBridge) SearchTextInWorkspace(language, query string) ([]any, error) {
	client, err := b.GetClientForLanguage(language)
	if err != nil {
		return nil, fmt.Errorf("failed to get client for language %s: %w", language, err)
	}

	symbols, err := client.WorkspaceSymbols(query)
	if err != nil {
		return nil, fmt.Errorf("failed to search workspace symbols: %w", err)
	}

	// Convert to []any for interface compatibility
	result := make([]any, len(symbols))
	for i, symbol := range symbols {
		result[i] = symbol
	}
	return result, nil
}

// GetMultiLanguageClients gets language clients for multiple languages with fallback
func (b *MCPLSPBridge) GetMultiLanguageClients(languages []string) (map[string]any, error) {
	clients := make(map[string]any)
	var lastErr error

	for _, language := range languages {
		client, err := b.GetClientForLanguage(language)
		if err != nil {
			logger.Error(fmt.Sprintf("Failed to get client for language %s: %v", language, err))
			lastErr = err
			continue
		}
		clients[language] = client
	}

	if len(clients) == 0 && lastErr != nil {
		return nil, fmt.Errorf("failed to get any language clients: %w", lastErr)
	}

	return clients, nil
}

// GetHoverInformation gets hover information for a symbol at a specific position
func (b *MCPLSPBridge) GetHoverInformation(uri string, line, character int32) (any, error) {
	// Extensive debug logging
	logger.Info(fmt.Sprintf("GetHoverInformation: Starting hover request for URI: %s, Line: %d, Character: %d", uri, line, character))

	// Infer language from URI
	language, err := b.InferLanguage(uri)
	if err != nil {
		logger.Error("GetHoverInformation: Failed to infer language", fmt.Sprintf("URI: %s, Error: %v", uri, err))
		return nil, fmt.Errorf("failed to infer language: %w", err)
	}
	logger.Info(fmt.Sprintf("GetHoverInformation: Inferred language: %s", language))

	// Get language client
	client, err := b.GetClientForLanguage(language)
	if err != nil {
		logger.Error("GetHoverInformation: Failed to get language client", fmt.Sprintf("Language: %s, Error: %v", language, err))
		return nil, fmt.Errorf("failed to get client for language %s: %w", language, err)
	}

	// Execute hover request directly to get HoverResponse
	hoverParams := protocol.HoverParams{
		TextDocument: protocol.TextDocumentIdentifier{Uri: protocol.DocumentUri(uri)},
		Position: protocol.Position{
			Line:      uint32(line),
			Character: uint32(character),
		},
	}

	logger.Info("GetHoverInformation: Sending hover request to language server")

	var result protocol.HoverResponse
	err = client.SendRequest("textDocument/hover", hoverParams, &result, 5*time.Second)
	if err != nil {
		logger.Error("GetHoverInformation: Hover request failed", fmt.Sprintf("Language: %s, Error: %v", language, err))
		return nil, fmt.Errorf("hover request failed: %w", err)
	}

	// Log hover response details
	logger.Info(fmt.Sprintf("GetHoverInformation: Received hover response. Type: %T, Contents: %+v", result, result))

	return result, nil
}

// GetDiagnostics gets diagnostics for a document
func (b *MCPLSPBridge) GetDiagnostics(uri string) ([]any, error) {
	// For now, return empty diagnostics since LSP diagnostics are typically pushed
	// and we haven't implemented a storage mechanism yet
	// TODO: Implement diagnostic storage in the client handler
	return []any{}, nil
}

// GetSignatureHelp gets signature help for a function at a specific position
func (b *MCPLSPBridge) GetSignatureHelp(uri string, line, character int32) (any, error) {
	// Infer language from URI
	language, err := b.InferLanguage(uri)
	if err != nil {
		return nil, fmt.Errorf("failed to infer language: %w", err)
	}

	// Get language client
	client, err := b.GetClientForLanguage(language)
	if err != nil {
		return nil, fmt.Errorf("failed to get client for language %s: %w", language, err)
	}

	// Execute signature help request
	signatureParams := protocol.SignatureHelpParams{
		TextDocument: protocol.TextDocumentIdentifier{Uri: protocol.DocumentUri(uri)},
		Position: protocol.Position{
			Line:      uint32(line),
			Character: uint32(character),
		},
	}

	var result protocol.SignatureHelpResponse
	err = client.SendRequest("textDocument/signatureHelp", signatureParams, &result, 5*time.Second)
	if err != nil {
		return nil, fmt.Errorf("signature help request failed: %w", err)
	}

	return result, nil
}

// GetCodeActions gets code actions for a specific range
func (b *MCPLSPBridge) GetCodeActions(uri string, line, character, endLine, endCharacter int32) ([]any, error) {
	// Infer language from URI
	language, err := b.InferLanguage(uri)
	if err != nil {
		return nil, fmt.Errorf("failed to infer language: %w", err)
	}

	// Get language client
	client, err := b.GetClientForLanguage(language)
	if err != nil {
		return nil, fmt.Errorf("failed to get client for language %s: %w", language, err)
	}

	// Execute code action request
	params := protocol.CodeActionParams{
		TextDocument: protocol.TextDocumentIdentifier{Uri: protocol.DocumentUri(uri)},
		Range: protocol.Range{
			Start: protocol.Position{Line: uint32(line), Character: uint32(character)},
			End:   protocol.Position{Line: uint32(endLine), Character: uint32(endCharacter)},
		},
		Context: protocol.CodeActionContext{
			// Context can be empty for general code actions
		},
	}

	var result []protocol.CodeAction
	err = client.SendRequest("textDocument/codeAction", params, &result, 5*time.Second)
	if err != nil {
		return nil, fmt.Errorf("code action request failed: %w", err)
	}

	// Convert to []any for interface compatibility
	actions := make([]any, len(result))
	for i, action := range result {
		actions[i] = action
	}

	return actions, nil
}

// FormatDocument formats a document
func (b *MCPLSPBridge) FormatDocument(uri string, tabSize int32, insertSpaces bool) ([]any, error) {
	// Infer language from URI
	language, err := b.InferLanguage(uri)
	if err != nil {
		return nil, fmt.Errorf("failed to infer language: %w", err)
	}

	// Get language client
	client, err := b.GetClientForLanguage(language)
	if err != nil {
		return nil, fmt.Errorf("failed to get client for language %s: %w", language, err)
	}

	// Execute document formatting request
	params := protocol.DocumentFormattingParams{
		TextDocument: protocol.TextDocumentIdentifier{Uri: protocol.DocumentUri(uri)},
		Options: protocol.FormattingOptions{
			TabSize:      uint32(tabSize),
			InsertSpaces: insertSpaces,
		},
	}

	var result []protocol.TextEdit
	err = client.SendRequest("textDocument/formatting", params, &result, 10*time.Second)
	if err != nil {
		return nil, fmt.Errorf("document formatting request failed: %w", err)
	}

	// Convert to []any for interface compatibility
	edits := make([]any, len(result))
	for i, edit := range result {
		edits[i] = edit
	}

	return edits, nil
}

// RenameSymbol renames a symbol with optional preview
func (b *MCPLSPBridge) RenameSymbol(uri string, line, character int32, newName string, preview bool) (any, error) {
	// Infer language from URI
	language, err := b.InferLanguage(uri)
	if err != nil {
		return nil, fmt.Errorf("failed to infer language: %w", err)
	}

	// Get language client
	client, err := b.GetClientForLanguage(language)
	if err != nil {
		return nil, fmt.Errorf("failed to get client for language %s: %w", language, err)
	}

	// Execute rename request
	params := protocol.RenameParams{
		TextDocument: protocol.TextDocumentIdentifier{Uri: protocol.DocumentUri(uri)},
		Position: protocol.Position{
			Line:      uint32(line),
			Character: uint32(character),
		},
		NewName: newName,
	}

	var result protocol.WorkspaceEdit
	err = client.SendRequest("textDocument/rename", params, &result, 10*time.Second)
	if err != nil {
		return nil, fmt.Errorf("rename request failed: %w", err)
	}

	return result, nil
}

// FindImplementations finds implementations of a symbol
func (b *MCPLSPBridge) FindImplementations(uri string, line, character int32) ([]any, error) {
	// Infer language from URI
	language, err := b.InferLanguage(uri)
	if err != nil {
		return nil, fmt.Errorf("failed to infer language: %w", err)
	}

	// Get language client
	client, err := b.GetClientForLanguage(language)
	if err != nil {
		return nil, fmt.Errorf("failed to get client for language %s: %w", language, err)
	}

	// Execute implementation request
	params := protocol.ImplementationParams{
		TextDocument: protocol.TextDocumentIdentifier{Uri: protocol.DocumentUri(uri)},
		Position: protocol.Position{
			Line:      uint32(line),
			Character: uint32(character),
		},
	}

	var result []protocol.Location
	err = client.SendRequest("textDocument/implementation", params, &result, 5*time.Second)
	if err != nil {
		return nil, fmt.Errorf("implementation request failed: %w", err)
	}

	// Convert to []any for interface compatibility
	implementations := make([]any, len(result))
	for i, impl := range result {
		implementations[i] = impl
	}

	return implementations, nil
}

// PrepareCallHierarchy prepares call hierarchy items
func (b *MCPLSPBridge) PrepareCallHierarchy(uri string, line, character int32) ([]any, error) {
	// Infer language from URI
	language, err := b.InferLanguage(uri)
	if err != nil {
		return nil, fmt.Errorf("failed to infer language: %w", err)
	}

	// Get language client
	client, err := b.GetClientForLanguage(language)
	if err != nil {
		return nil, fmt.Errorf("failed to get client for language %s: %w", language, err)
	}

	// Execute prepare call hierarchy request
	params := protocol.CallHierarchyPrepareParams{
		TextDocument: protocol.TextDocumentIdentifier{Uri: protocol.DocumentUri(uri)},
		Position: protocol.Position{
			Line:      uint32(line),
			Character: uint32(character),
		},
	}

	var result []protocol.CallHierarchyItem
	err = client.SendRequest("textDocument/prepareCallHierarchy", params, &result, 5*time.Second)
	if err != nil {
		return nil, fmt.Errorf("prepare call hierarchy request failed: %w", err)
	}

	// Convert to []any for interface compatibility
	items := make([]any, len(result))
	for i, item := range result {
		items[i] = item
	}

	return items, nil
}

// GetIncomingCalls gets incoming calls for a call hierarchy item
func (b *MCPLSPBridge) GetIncomingCalls(item any) ([]any, error) {
	// For now, return empty since we need to handle the protocol.CallHierarchyItem properly
	// TODO: Implement proper call hierarchy item handling
	return []any{}, nil
}

// GetOutgoingCalls gets outgoing calls for a call hierarchy item
func (b *MCPLSPBridge) GetOutgoingCalls(item any) ([]any, error) {
	// For now, return empty since we need to handle the protocol.CallHierarchyItem properly
	// TODO: Implement proper call hierarchy item handling
	return []any{}, nil
}

// GetWorkspaceDiagnostics gets diagnostics for entire workspace
func (b *MCPLSPBridge) GetWorkspaceDiagnostics(workspaceUri string, identifier string) (any, error) {
	// 1. Detect project languages or use multi-language approach
	languages, err := b.DetectProjectLanguages(workspaceUri)
	if err != nil {
		return nil, fmt.Errorf("failed to detect project languages: %w", err)
	}

	if len(languages) == 0 {
		return []any{}, nil // No languages detected, return empty result
	}

	// 2. Get language clients for detected languages
	clients, err := b.GetMultiLanguageClients(languages)
	if err != nil {
		return nil, fmt.Errorf("failed to get language clients: %w", err)
	}

	// 3. Execute workspace diagnostic requests
	var allReports []any
	for language, clientInterface := range clients {
		client, ok := clientInterface.(*lsp.LanguageClient)
		if !ok {
			logger.Warn(fmt.Sprintf("Invalid client type for language %s", language))
			continue
		}

		report, err := b.executeWorkspaceDiagnosticRequest(client, workspaceUri, identifier)
		if err != nil {
			logger.Warn(fmt.Sprintf("Workspace diagnostics failed for %s: %v", language, err))
			continue
		}
		allReports = append(allReports, report)
	}

	return allReports, nil
}

// executeWorkspaceDiagnosticRequest executes LSP workspace/diagnostic request
func (b *MCPLSPBridge) executeWorkspaceDiagnosticRequest(client *lsp.LanguageClient, workspaceUri, identifier string) (protocol.WorkspaceDiagnosticReport, error) {
	params := protocol.WorkspaceDiagnosticParams{
		Identifier:        identifier,
		PreviousResultIds: []protocol.PreviousResultId{}, // Empty for first request
	}

	var result protocol.WorkspaceDiagnosticReport
	err := client.SendRequest("workspace/diagnostic", params, &result, 30*time.Second) // Longer timeout for workspace operations
	if err != nil {
		return protocol.WorkspaceDiagnosticReport{}, fmt.Errorf("workspace diagnostic request failed: %w", err)
	}

	return result, nil
}
