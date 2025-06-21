package bridge

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
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
	// Check if client already exists
	if existingClient, exists := b.clients[language]; exists {
		// Check if client context is still valid (not cancelled)
		if existingClient.Context().Err() == nil {
			// Reset status to connected if it was in error state but context is still valid
			metrics := existingClient.GetMetrics()
			logger.Info(fmt.Sprintf("GetClientForLanguage: Existing client for %s, metrics: %+v", language, metrics))
			return existingClient, nil
		}
		// Client context is cancelled, remove it and create a new one
		logger.Warn(fmt.Sprintf("Removing client with cancelled context for language %s", language))
		existingClient.Close()
		delete(b.clients, language)
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

// normalizeURI ensures the URI has the proper file:// scheme
func normalizeURI(uri string) string {
	// If it already has a file scheme, return as-is
	if strings.HasPrefix(uri, "file://") {
		return uri
	}

	// If it has any other scheme (http://, https://, etc.), return as-is
	if strings.Contains(uri, "://") {
		return uri
	}

	// If it's an absolute path, convert to file URI
	if strings.HasPrefix(uri, "/") {
		return "file://" + uri
	}

	// If it's a relative path, convert to absolute path first, then to file URI
	if absPath, err := filepath.Abs(uri); err == nil {
		return "file://" + absPath
	}

	// Fallback: assume it's a file path and add file:// prefix
	return "file://" + uri
}

// GetHoverInformation gets hover information for a symbol at a specific position
func (b *MCPLSPBridge) GetHoverInformation(uri string, line, character int32) (any, error) {
	// Extensive debug logging
	logger.Info(fmt.Sprintf("GetHoverInformation: Starting hover request for URI: %s, Line: %d, Character: %d", uri, line, character))

	// Normalize URI to ensure proper file:// scheme
	normalizedURI := normalizeURI(uri)
	logger.Info(fmt.Sprintf("GetHoverInformation: Normalized URI: %s -> %s", uri, normalizedURI))

	// Infer language from URI (use original URI for file extension detection)
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

	// Ensure the document is opened in the language server
	err = b.ensureDocumentOpen(client, normalizedURI, language)
	if err != nil {
		logger.Error("GetHoverInformation: Failed to open document", fmt.Sprintf("URI: %s, Error: %v", normalizedURI, err))
		// Continue anyway, as some servers might still work without explicit didOpen
	}

	// Execute hover request to get Hover result using normalized URI
	hoverParams := protocol.HoverParams{
		TextDocument: protocol.TextDocumentIdentifier{Uri: protocol.DocumentUri(normalizedURI)},
		Position: protocol.Position{
			Line:      uint32(line),
			Character: uint32(character),
		},
	}

	logger.Info("GetHoverInformation: Sending hover request to language server")

	// LSP hover response is either a Hover object or null, not wrapped in HoverResponse
	var result *protocol.Hover
	err = client.SendRequest("textDocument/hover", hoverParams, &result, 5*time.Second)
	if err != nil {
		logger.Error("GetHoverInformation: Hover request failed", fmt.Sprintf("Language: %s, Error: %v", language, err))
		return nil, fmt.Errorf("hover request failed: %w", err)
	}

	// Log hover response details
	logger.Info(fmt.Sprintf("GetHoverInformation: Received hover response. Type: %T, Contents: %+v", result, result))

	return result, nil
}

// ensureDocumentOpen sends a textDocument/didOpen notification to the language server
// This is often required before other document operations can be performed
func (b *MCPLSPBridge) ensureDocumentOpen(client *lsp.LanguageClient, uri, language string) error {
	// Read the file content
	// Remove file:// prefix to get the actual file path
	filePath := strings.TrimPrefix(uri, "file://")

	content, err := os.ReadFile(filePath)
	if err != nil {
		return fmt.Errorf("failed to read file %s: %w", filePath, err)
	}

	// Determine the language ID based on the language parameter
	languageId := language
	if language == "typescript" {
		languageId = "typescript"
	} else if language == "javascript" {
		languageId = "javascript"
	}

	// Send textDocument/didOpen notification
	didOpenParams := protocol.DidOpenTextDocumentParams{
		TextDocument: protocol.TextDocumentItem{
			Uri:        protocol.DocumentUri(uri),
			LanguageId: protocol.LanguageKind(languageId),
			Version:    1,
			Text:       string(content),
		},
	}

	err = client.SendNotification("textDocument/didOpen", didOpenParams)
	if err != nil {
		return fmt.Errorf("failed to send didOpen notification: %w", err)
	}

	logger.Info(fmt.Sprintf("Document opened in LSP server: %s (language: %s)", uri, languageId))
	return nil
}

// GetDocumentSymbols gets all symbols in a document
func (b *MCPLSPBridge) GetDocumentSymbols(uri string) ([]any, error) {
	// Normalize URI to ensure proper file:// scheme
	normalizedURI := normalizeURI(uri)
	logger.Info(fmt.Sprintf("GetDocumentSymbols: Starting request for URI: %s -> %s", uri, normalizedURI))

	// Infer language from URI
	language, err := b.InferLanguage(uri)
	if err != nil {
		logger.Error("GetDocumentSymbols: Failed to infer language", fmt.Sprintf("URI: %s, Error: %v", uri, err))
		return nil, fmt.Errorf("failed to infer language: %w", err)
	}
	logger.Info(fmt.Sprintf("GetDocumentSymbols: Inferred language: %s", language))

	// Get language client
	client, err := b.GetClientForLanguage(language)
	if err != nil {
		logger.Error("GetDocumentSymbols: Failed to get language client", fmt.Sprintf("Language: %s, Error: %v", language, err))
		return nil, fmt.Errorf("failed to get client for language %s: %w", language, err)
	}

	// Additional debugging: check client status
	metrics := client.GetMetrics()
	logger.Info(fmt.Sprintf("GetDocumentSymbols: Client metrics: %+v", metrics))

	// Ensure the document is opened in the language server
	err = b.ensureDocumentOpen(client, normalizedURI, language)
	if err != nil {
		logger.Error("GetDocumentSymbols: Failed to open document", fmt.Sprintf("URI: %s, Error: %v", normalizedURI, err))
		// Continue anyway, as some servers might still work without explicit didOpen
	}

	// Get document symbols
	symbols, err := client.DocumentSymbols(normalizedURI)
	if err != nil {
		logger.Error("GetDocumentSymbols: Request failed", fmt.Sprintf("Language: %s, Error: %v", language, err))
		return nil, fmt.Errorf("document symbols request failed: %w", err)
	}

	// Convert to []any for interface compatibility
	result := make([]any, len(symbols))
	for i, symbol := range symbols {
		result[i] = symbol
	}

	logger.Info(fmt.Sprintf("GetDocumentSymbols: Found %d symbols", len(result)))
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
	// Normalize URI
	normalizedURI := normalizeURI(uri)

	// Infer language from URI
	language, err := b.InferLanguage(normalizedURI)
	if err != nil {
		logger.Error("GetSignatureHelp: Language inference failed", fmt.Sprintf("URI: %s, Error: %v", normalizedURI, err))
		return nil, fmt.Errorf("failed to infer language: %w", err)
	}

	// Get language client
	client, err := b.GetClientForLanguage(language)
	if err != nil {
		logger.Error("GetSignatureHelp: Client creation failed", fmt.Sprintf("Language: %s, Error: %v", language, err))
		return nil, fmt.Errorf("failed to get client for language %s: %w", language, err)
	}

	// Ensure document is open in the language server
	err = b.ensureDocumentOpen(client, normalizedURI, language)
	if err != nil {
		logger.Error("GetSignatureHelp: Failed to open document", fmt.Sprintf("URI: %s, Error: %v", normalizedURI, err))
		// Continue anyway, as some servers might still work without explicit didOpen
	}

	// Execute signature help request using the LSP client method
	signatureHelp, err := client.SignatureHelp(normalizedURI, line, character)
	if err != nil {
		logger.Error("GetSignatureHelp: Request failed", fmt.Sprintf("Language: %s, Error: %v", language, err))
		return nil, fmt.Errorf("signature help request failed: %w", err)
	}

	logger.Info(fmt.Sprintf("GetSignatureHelp: Found signature help for position %d:%d", line, character))
	return signatureHelp, nil
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

// ApplyTextEdits applies text edits to a file
func (b *MCPLSPBridge) ApplyTextEdits(uri string, edits []any) error {
	// Convert URI to file path
	filePath := strings.TrimPrefix(uri, "file://")

	// Read current file content
	content, err := os.ReadFile(filePath)
	if err != nil {
		return fmt.Errorf("failed to read file %s: %w", filePath, err)
	}

	// Convert edits to protocol.TextEdit
	textEdits := make([]protocol.TextEdit, 0, len(edits))
	for _, edit := range edits {
		if textEdit, ok := edit.(protocol.TextEdit); ok {
			textEdits = append(textEdits, textEdit)
		}
	}

	// Apply edits to content
	modifiedContent, err := applyTextEditsToContent(string(content), textEdits)
	if err != nil {
		return fmt.Errorf("failed to apply text edits: %w", err)
	}

	// Write modified content back to file
	err = os.WriteFile(filePath, []byte(modifiedContent), 0o644)
	if err != nil {
		return fmt.Errorf("failed to write file %s: %w", filePath, err)
	}

	return nil
}

// applyTextEditsToContent applies text edits to string content
func applyTextEditsToContent(content string, edits []protocol.TextEdit) (string, error) {
	if len(edits) == 0 {
		return content, nil
	}

	// Split content into lines for easier manipulation
	lines := strings.Split(content, "\n")

	// Sort edits by position (reverse order to apply from end to beginning)
	// This prevents position shifts from affecting subsequent edits
	for i := 0; i < len(edits)-1; i++ {
		for j := i + 1; j < len(edits); j++ {
			edit1 := edits[i]
			edit2 := edits[j]

			// Compare positions (later positions first)
			if edit1.Range.Start.Line < edit2.Range.Start.Line ||
				(edit1.Range.Start.Line == edit2.Range.Start.Line &&
					edit1.Range.Start.Character < edit2.Range.Start.Character) {
				edits[i], edits[j] = edits[j], edits[i]
			}
		}
	}

	// Apply edits in reverse order
	for _, edit := range edits {
		startLine := int(edit.Range.Start.Line)
		startChar := int(edit.Range.Start.Character)
		endLine := int(edit.Range.End.Line)
		endChar := int(edit.Range.End.Character)

		// Validate line indices
		if startLine >= len(lines) || endLine >= len(lines) {
			continue // Skip invalid edits
		}

		if startLine == endLine {
			// Single line edit
			line := lines[startLine]
			if startChar > len(line) || endChar > len(line) {
				continue // Skip invalid character positions
			}

			// Replace text within the line
			newLine := line[:startChar] + edit.NewText + line[endChar:]
			lines[startLine] = newLine
		} else {
			// Multi-line edit
			if startChar > len(lines[startLine]) || endChar > len(lines[endLine]) {
				continue // Skip invalid character positions
			}

			// Create new line combining start of first line + new text + end of last line
			newLine := lines[startLine][:startChar] + edit.NewText + lines[endLine][endChar:]

			// Remove the lines that were replaced
			newLines := make([]string, 0, len(lines)-(endLine-startLine))
			newLines = append(newLines, lines[:startLine]...)
			newLines = append(newLines, newLine)
			if endLine+1 < len(lines) {
				newLines = append(newLines, lines[endLine+1:]...)
			}
			lines = newLines
		}
	}

	return strings.Join(lines, "\n"), nil
}

// RenameSymbol renames a symbol with optional preview
func (b *MCPLSPBridge) RenameSymbol(uri string, line, character int32, newName string, preview bool) (any, error) {
	// Normalize URI to ensure proper file:// scheme
	normalizedURI := normalizeURI(uri)
	logger.Info(fmt.Sprintf("RenameSymbol: Starting rename request for URI: %s -> %s, Line: %d, Character: %d, NewName: %s", uri, normalizedURI, line, character, newName))

	// Infer language from URI
	language, err := b.InferLanguage(uri)
	if err != nil {
		logger.Error("RenameSymbol: Failed to infer language", fmt.Sprintf("URI: %s, Error: %v", uri, err))
		return nil, fmt.Errorf("failed to infer language: %w", err)
	}
	logger.Info(fmt.Sprintf("RenameSymbol: Inferred language: %s", language))

	// Get language client
	client, err := b.GetClientForLanguage(language)
	if err != nil {
		logger.Error("RenameSymbol: Failed to get language client", fmt.Sprintf("Language: %s, Error: %v", language, err))
		return nil, fmt.Errorf("failed to get client for language %s: %w", language, err)
	}

	// Ensure the document is opened in the language server
	err = b.ensureDocumentOpen(client, normalizedURI, language)
	if err != nil {
		logger.Error("RenameSymbol: Failed to open document", fmt.Sprintf("URI: %s, Error: %v", normalizedURI, err))
		// Continue anyway, as some servers might still work without explicit didOpen
	}

	// Execute rename request using normalized URI
	params := protocol.RenameParams{
		TextDocument: protocol.TextDocumentIdentifier{Uri: protocol.DocumentUri(normalizedURI)},
		Position: protocol.Position{
			Line:      uint32(line),
			Character: uint32(character),
		},
		NewName: newName,
	}

	logger.Info("RenameSymbol: Sending rename request to language server")

	var result protocol.WorkspaceEdit
	err = client.SendRequest("textDocument/rename", params, &result, 10*time.Second)
	if err != nil {
		logger.Error("RenameSymbol: Rename request failed", fmt.Sprintf("Language: %s, Error: %v", language, err))
		return nil, fmt.Errorf("rename request failed: %w", err)
	}

	// Log the response details
	logger.Info(fmt.Sprintf("RenameSymbol: Received rename response. Changes: %+v, DocumentChanges: %+v", result.Changes, result.DocumentChanges))

	return result, nil
}

// ApplyWorkspaceEdit applies a workspace edit to multiple files
func (b *MCPLSPBridge) ApplyWorkspaceEdit(edit any) error {
	workspaceEdit, ok := edit.(protocol.WorkspaceEdit)
	if !ok {
		return fmt.Errorf("invalid workspace edit type: %T", edit)
	}

	logger.Info(fmt.Sprintf("ApplyWorkspaceEdit: Processing workspace edit. Changes: %+v, DocumentChanges: %+v", workspaceEdit.Changes, workspaceEdit.DocumentChanges))

	// Handle DocumentChanges format (preferred by most language servers)
	if workspaceEdit.DocumentChanges != nil {
		for _, docChange := range workspaceEdit.DocumentChanges {
			logger.Info(fmt.Sprintf("ApplyWorkspaceEdit: Processing document change of type: %T", docChange.Value))
			
			// DocumentChanges is []Or4[TextDocumentEdit, CreateFile, RenameFile, DeleteFile]
			// We only handle TextDocumentEdit for now
			if textDocEdit, ok := docChange.Value.(protocol.TextDocumentEdit); ok {
				logger.Info(fmt.Sprintf("ApplyWorkspaceEdit: Found TextDocumentEdit for URI: %s", textDocEdit.TextDocument.Uri))
				
				// Convert protocol.TextEdit to []any for ApplyTextEdits
				textEdits := make([]any, len(textDocEdit.Edits))
				for i, edit := range textDocEdit.Edits {
					// Edits might also be a union type, extract the actual TextEdit
					if actualEdit, ok := edit.Value.(protocol.TextEdit); ok {
						textEdits[i] = actualEdit
						logger.Info(fmt.Sprintf("ApplyWorkspaceEdit: Edit %d - Line %d:%d-%d:%d, NewText: '%s'", 
							i+1, actualEdit.Range.Start.Line, actualEdit.Range.Start.Character,
							actualEdit.Range.End.Line, actualEdit.Range.End.Character, actualEdit.NewText))
					} else {
						logger.Error("ApplyWorkspaceEdit: Edit is not a TextEdit", fmt.Sprintf("Type: %T", edit.Value))
						continue
					}
				}
				
				// Apply the edits
				if len(textEdits) > 0 {
					logger.Info(fmt.Sprintf("ApplyWorkspaceEdit: Applying %d text edits to %s", len(textEdits), textDocEdit.TextDocument.Uri))
					err := b.ApplyTextEdits(string(textDocEdit.TextDocument.Uri), textEdits)
					if err != nil {
						return fmt.Errorf("failed to apply document changes to %s: %w", textDocEdit.TextDocument.Uri, err)
					}
				}
			} else {
				// Handle other types (CreateFile, RenameFile, DeleteFile) if needed
				logger.Info(fmt.Sprintf("ApplyWorkspaceEdit: Skipping non-TextDocumentEdit change: %T", docChange.Value))
			}
		}
	}

	// Apply changes map (alternative format)
	if workspaceEdit.Changes != nil {
		for uri, edits := range workspaceEdit.Changes {
			// Convert []protocol.TextEdit to []any
			anyEdits := make([]any, len(edits))
			for i, edit := range edits {
				anyEdits[i] = edit
			}
			
			err := b.ApplyTextEdits(string(uri), anyEdits)
			if err != nil {
				return fmt.Errorf("failed to apply edits to %s: %w", uri, err)
			}
		}
	}

	return nil
}

// FindImplementations finds implementations of a symbol
func (b *MCPLSPBridge) FindImplementations(uri string, line, character int32) ([]any, error) {
	// Normalize URI
	normalizedURI := normalizeURI(uri)

	// Infer language from URI
	language, err := b.InferLanguage(normalizedURI)
	if err != nil {
		logger.Error("FindImplementations: Language inference failed", fmt.Sprintf("URI: %s, Error: %v", normalizedURI, err))
		return nil, fmt.Errorf("failed to infer language: %w", err)
	}

	// Get language client
	client, err := b.GetClientForLanguage(language)
	if err != nil {
		logger.Error("FindImplementations: Client creation failed", fmt.Sprintf("Language: %s, Error: %v", language, err))
		return nil, fmt.Errorf("failed to get client for language %s: %w", language, err)
	}

	// Ensure document is open in the language server
	err = b.ensureDocumentOpen(client, normalizedURI, language)
	if err != nil {
		logger.Error("FindImplementations: Failed to open document", fmt.Sprintf("URI: %s, Error: %v", normalizedURI, err))
		// Continue anyway, as some servers might still work without explicit didOpen
	}

	// Execute implementation request using the LSP client method
	implementations, err := client.Implementation(normalizedURI, line, character)
	if err != nil {
		logger.Error("FindImplementations: Request failed", fmt.Sprintf("Language: %s, Error: %v", language, err))
		return nil, fmt.Errorf("implementation request failed: %w", err)
	}

	// Convert to []any for interface compatibility
	result := make([]any, len(implementations))
	for i, impl := range implementations {
		result[i] = impl
	}

	logger.Info(fmt.Sprintf("FindImplementations: Found %d implementations", len(result)))
	return result, nil
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
