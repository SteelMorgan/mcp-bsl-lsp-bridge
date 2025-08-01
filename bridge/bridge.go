package bridge

import (
	"errors"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"rockerboo/mcp-lsp-bridge/logger"
	"rockerboo/mcp-lsp-bridge/lsp"
	"rockerboo/mcp-lsp-bridge/security"
	"rockerboo/mcp-lsp-bridge/types"
	"rockerboo/mcp-lsp-bridge/utils"

	"github.com/mark3labs/mcp-go/server"
	"github.com/myleshyson/lsprotocol-go/protocol"
)

// NewMCPLSPBridge creates a new bridge instance with provided configuration and client factory
func NewMCPLSPBridge(config types.LSPServerConfigProvider, allowedDirectories []string) *MCPLSPBridge {
	bridge := &MCPLSPBridge{
		clients:            make(map[types.LanguageServer]types.LanguageClientInterface),
		config:             config,
		allowedDirectories: allowedDirectories,
	}

	return bridge
}

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

func (b *MCPLSPBridge) IsAllowedDirectory(path string) (string, error) {
	return security.ValidateConfigPath(path, b.allowedDirectories)
}

func (b *MCPLSPBridge) AllowedDirectories() []string {
	return b.allowedDirectories
}

// validateAndConnectClient attempts to validate and establish a language server connection using the injected factory
func (b *MCPLSPBridge) validateAndConnectClient(language string, serverConfig types.LanguageServerConfigProvider, config ConnectionAttemptConfig) (types.LanguageClientInterface, error) {
	// Attempt connection with retry mechanism
	var lastErr error

	startTime := time.Now()

	// Get current working directory
	// dir, err := os.Getwd()
	// if err != nil {
	// 	return nil, fmt.Errorf("failed to get current directory: %w", err)
	// }
	dirs := b.AllowedDirectories()
	dir := dirs[0] // Get first directory (for now)

	logger.Debug(fmt.Sprintf("validateAndConnectClient: Using directory: %s from allowed dirs: %v", dir, dirs))

	absPath, err := b.IsAllowedDirectory(dir)
	if err != nil {
		return nil, fmt.Errorf("file path is not allowed: %s", err)
	}

	for attempt := 0; attempt < config.MaxRetries; attempt++ {
		// Check if total timeout exceeded
		if time.Since(startTime) > config.TotalTimeout {
			break
		}

		// Create language client using the factory
		var client *lsp.LanguageClient

		var err error

		client, err = lsp.NewLanguageClient(serverConfig.GetCommand(), serverConfig.GetArgs()...)
		if err != nil {
			lastErr = fmt.Errorf("failed to create language client on attempt %d: %w", attempt+1, err)
			continue
		}

		_, err = client.Connect()
		if err != nil {
			lastErr = fmt.Errorf("failed to connect to the LSP on attempt %d: %w", attempt+1, err)

			time.Sleep(config.RetryDelay)

			continue
		}

		rootPath := "file://" + absPath
		// root_uri := protocol.DocumentUri(root_path)

		logger.Debug("validateAndConnectClient: Root path for LSP: " + rootPath)
		// Process IDs are typically small positive integers, safe to convert
		// But we'll add bounds checking for completeness
		pid := os.Getpid()
		if pid < 0 || pid > math.MaxInt32 {
			return nil, fmt.Errorf("process ID out of range: %d", pid)
		}
		process_id := int32(pid)

		// Prepare initialization parameters
		workspaceFolders := []protocol.WorkspaceFolder{
			{
				Uri:  protocol.URI(rootPath),
				Name: filepath.Base(absPath),
			},
		}

		client.SetProjectRoots([]string{dir})

		params := protocol.InitializeParams{
			ProcessId: &process_id,
			ClientInfo: &protocol.ClientInfo{
				Name:    "MCP-LSP Bridge",
				Version: "1.0.0",
			},
			// RootUri:          &root_uri,
			WorkspaceFolders: &workspaceFolders,
			// Capabilities: protocol.ClientCapabilities{
			// 	TextDocument: &protocol.TextDocumentClientCapabilities{
			// 		SignatureHelp: &protocol.SignatureHelpClientCapabilities{},
			// 	},
			// },
		}

		// Apply any initialization options from the configuration
		// if serverConfig.GetInitializationOptions() != nil {
		// 	params.InitializationOptions = serverConfig.GetInitializationOptions()
		// }

		// Check connection status before initialize
		metrics := client.GetMetrics()
		logger.Debug(fmt.Sprintf("STATUS: Before Initialize - Client connected: %v, ctx.Err(): %v", metrics.IsConnected(), client.Context().Err()))

		logger.Debug(fmt.Sprintf("STATUS: client %+v", client))

		// Send initialize request
		result, err := client.Initialize(params)
		if err != nil {
			lastErr = fmt.Errorf("initialize request failed on attempt %d: %w", attempt+1, err)
			logger.Error(lastErr)

			err = client.Close()
			if err != nil {
				return nil, err
			}

			time.Sleep(config.RetryDelay)

			continue
		}

		logger.Debug(fmt.Sprintf("STATUS: After Initialize - Client connected: %v, ctx.Err(): %v", metrics.IsConnected(), client.Context().Err()))
		logger.Debug(fmt.Sprintf("STATUS: Initialize result: %+v", result))
		logger.Debug(fmt.Sprintf("STATUS: Setting up semantic tokens provider: %+v", client.ServerCapabilities().SemanticTokensProvider))

		// Set server capabilities
		client.SetServerCapabilities(result.Capabilities)

		semanticTokensProvider := client.ServerCapabilities().SemanticTokensProvider

		var supportsSemanticTokens bool

		if semanticTokensProvider == nil {
			logger.Warn("No semantic tokens provider found")

			supportsSemanticTokens = false
		} else {
			switch semanticTokensProvider.Value.(type) {
			case bool:
				logger.Debug("Semantic tokens supported")

				supportsSemanticTokens = true
			default:
				logger.Warn("Unknown semantic tokens provider")

				supportsSemanticTokens = false
			}

			if supportsSemanticTokens {
				err = client.SetupSemanticTokens()
				if err != nil {
					// Semantic tokens setup failure is non-fatal - many servers don't support this feature
					logger.Warn(fmt.Sprintf("Failed to setup semantic tokens on attempt %d (non-fatal): %v", attempt+1, err))
					// Continue without semantic tokens support
				}
			}
		}

		// Log server info and capabilities
		if result.ServerInfo != nil {
			logger.Debug(fmt.Sprintf("Initialize result - Server Info: %+v", *result.ServerInfo))
		}

		logger.Debug(fmt.Sprintf("Initialize result - Capabilities: %+v", result.Capabilities))

		// Enhanced logging for Workspace and WorkspaceFolders capabilities
		if result.Capabilities.Workspace != nil {
			logger.Debug(fmt.Sprintf("Workspace Capabilities: %+v", *result.Capabilities.Workspace))

			// Specifically log WorkspaceFolders support
			if result.Capabilities.Workspace.WorkspaceFolders != nil {
				logger.Debug(fmt.Sprintf("WorkspaceFolders Support: Supported = %v",
					*result.Capabilities.Workspace.WorkspaceFolders))
			} else {
				logger.Warn("WorkspaceFolders Capability is nil")
			}
		} else {
			logger.Warn("Workspace Capabilities are nil")
		}

		// Send initialized notification
		err = client.Initialized()
		if err != nil {
			lastErr = fmt.Errorf("failed to send initialized notification on attempt %d: %w", attempt+1, err)

			err = client.Close()
			if err != nil {
				return nil, err
			}

			time.Sleep(config.RetryDelay)

			continue
		}

		// Successfully connected
		return client, nil
	}

	return nil, fmt.Errorf("failed to establish language server connection for %s after %d attempts: %w",
		language, config.MaxRetries, lastErr)
}

// GetClientForLanguage retrieves or creates a language server client for a specific language
func (b *MCPLSPBridge) GetClientForLanguage(language string) (types.LanguageClientInterface, error) {
	// Look up the server name for the given language
	server := b.config.GetServerNameFromLanguage(types.Language(language))
	if server == "" {
		return nil, fmt.Errorf("no server found for language %s", language)
	}

	// Check if client already exists
	if existingClient, exists := b.clients[server]; exists {
		// Check if client context is still valid (not cancelled)
		if existingClient.Context().Err() == nil {
			// Reset status to connected if it was in error state but context is still valid
			metrics := existingClient.GetMetrics()
			logger.Debug(fmt.Sprintf("GetClientForLanguage: Existing client for %s, metrics: %+v", language, metrics))

			return existingClient, nil
		}
		// Client context is cancelled, remove it and create a new one
		logger.Warn("Removing client with cancelled context for language " + language)

		err := existingClient.Close()
		if err != nil {
			return nil, err
		}

		delete(b.clients, server)
	}

	// Find the server configuration
	serverConfig, err := b.GetConfig().FindServerConfig(language)
	if err != nil {
		return nil, fmt.Errorf("no server configuration found for language %s", language)
	}

	// Attempt to connect with default configuration
	client, err := b.validateAndConnectClient(language, serverConfig, DefaultConnectionConfig())
	if err != nil {
		return nil, err
	}

	// Store the new client
	b.clients[server] = client

	return client, nil
}

// GetAllClientsForLanguage retrieves or creates all language server clients for a specific language
func (b *MCPLSPBridge) GetAllClientsForLanguage(language string) ([]types.LanguageClientInterface, []types.LanguageServer, error) {
	// Find all server configurations for this language
	serverConfigs, serverNames, err := b.config.FindAllServerConfigs(language)
	if err != nil {
		return nil, nil, fmt.Errorf("no server configurations found for language %s: %w", language, err)
	}

	var clients []types.LanguageClientInterface
	var validServerNames []types.LanguageServer

	for i, serverName := range serverNames {
		serverConfig := serverConfigs[i]

		// Check if client already exists
		if existingClient, exists := b.clients[serverName]; exists {
			// Check if client context is still valid (not cancelled)
			if existingClient.Context().Err() == nil {
				clients = append(clients, existingClient)
				validServerNames = append(validServerNames, serverName)
				continue
			}
			// Client context is cancelled, remove it and create a new one
			logger.Warn("Removing client with cancelled context for language " + language + " server " + string(serverName))

			err := existingClient.Close()
			if err != nil {
				logger.Error(fmt.Sprintf("Error closing cancelled client for %s: %v", serverName, err))
			}

			delete(b.clients, serverName)
		}

		// Attempt to connect with default configuration
		client, err := b.validateAndConnectClient(language, serverConfig, DefaultConnectionConfig())
		if err != nil {
			logger.Warn(fmt.Sprintf("Failed to connect to server %s for language %s: %v", serverName, language, err))
			continue // Skip this server, try others
		}

		// Store the new client
		b.clients[serverName] = client
		clients = append(clients, client)
		validServerNames = append(validServerNames, serverName)
	}

	if len(clients) == 0 {
		return nil, nil, fmt.Errorf("failed to connect to any language servers for language %s", language)
	}

	return clients, validServerNames, nil
}

// CloseAllClients closes all active language server clients
func (b *MCPLSPBridge) CloseAllClients() {
	for serverName, client := range b.clients {
		err := client.Close()
		if err != nil {
			logger.Error(fmt.Errorf("failed to close client for %s: %w", serverName, err))
		}
	}

	b.clients = make(map[types.LanguageServer]types.LanguageClientInterface)
}

// InferLanguage infers the programming language from a file path
func (b *MCPLSPBridge) InferLanguage(filePath string) (*types.Language, error) {
	ext := filepath.Ext(filePath)
	language, err := b.GetConfig().FindExtLanguage(ext)

	if err != nil {
		return nil, err
	}

	return language, nil
}

// GetConfig returns the bridge's configuration
func (b *MCPLSPBridge) GetConfig() types.LSPServerConfigProvider {
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

// Detects all languages used in a project directory
func (b *MCPLSPBridge) DetectProjectLanguages(projectPath string) ([]types.Language, error) {
	if b.config == nil {
		return nil, errors.New("no LSP configuration available")
	}

	return b.GetConfig().DetectProjectLanguages(projectPath)
}

// Detects the primary language of a project
func (b *MCPLSPBridge) DetectPrimaryProjectLanguage(projectPath string) (*types.Language, error) {
	if b.config == nil {
		return nil, errors.New("no LSP configuration available")
	}

	return b.GetConfig().DetectPrimaryProjectLanguage(projectPath)
}

// Finds all references to a symbol at a given position
func (b *MCPLSPBridge) FindSymbolReferences(language, uri string, line, character uint32, includeDeclaration bool) ([]protocol.Location, error) {
	client, err := b.GetClientForLanguage(language)
	if err != nil {
		return nil, fmt.Errorf("failed to get client for language %s: %w", language, err)
	}

	references, err := client.References(uri, line, character, includeDeclaration)
	if err != nil {
		return nil, fmt.Errorf("failed to find references: %w", err)
	}

	return references, nil
}

// FindSymbolDefinitions finds all definitions for a symbol at a given position
func (b *MCPLSPBridge) FindSymbolDefinitions(language, uri string, line, character uint32) ([]protocol.Or2[protocol.LocationLink, protocol.Location], error) {
	client, err := b.GetClientForLanguage(language)
	if err != nil {
		return nil, fmt.Errorf("failed to get client for language %s: %w", language, err)
	}

	definitions, err := client.Definition(uri, line, character)
	if err != nil {
		// Log the error but return empty results instead of failing
		logger.Warn(fmt.Sprintf("Failed to find definitions for %s at %s:%d:%d: %v", language, uri, line, character, err))
		return []protocol.Or2[protocol.LocationLink, protocol.Location]{}, nil
	}

	return definitions, nil
}

// SearchTextInWorkspace performs a text search across the workspace
func (b *MCPLSPBridge) SearchTextInWorkspace(language, query string) ([]protocol.WorkspaceSymbol, error) {
	client, err := b.GetClientForLanguage(language)
	if err != nil {
		return nil, fmt.Errorf("failed to get client for language %s: %w", language, err)
	}

	symbols, err := client.WorkspaceSymbols(query)
	if err != nil {
		return nil, fmt.Errorf("failed to search workspace symbols: %w", err)
	}

	return symbols, nil
}

// GetMultiLanguageClients gets language clients for multiple languages with fallback
func (b *MCPLSPBridge) GetMultiLanguageClients(languages []string) (map[types.Language]types.LanguageClientInterface, error) {
	clients := make(map[types.Language]types.LanguageClientInterface)

	var mu sync.Mutex

	var wg sync.WaitGroup

	var lastErr error

	var errMu sync.Mutex

	for _, language := range languages {
		wg.Add(1)

		go func(lang string) {
			defer wg.Done()

			client, err := b.GetClientForLanguage(lang)
			if err != nil {
				logger.Error(fmt.Sprintf("Failed to get client for language %s: %v", lang, err))
				errMu.Lock()
				lastErr = err
				errMu.Unlock()

				return
			}

			mu.Lock()
			clients[types.Language(lang)] = client
			mu.Unlock()
		}(language)
	}

	wg.Wait()

	if len(clients) == 0 && lastErr != nil {
		return nil, fmt.Errorf("failed to get any language clients: %w", lastErr)
	}

	return clients, nil
}

// GetHoverInformation gets hover information for a symbol at a specific position
func (b *MCPLSPBridge) GetHoverInformation(uri string, line, character uint32) (*protocol.Hover, error) {
	// Extensive debug logging
	logger.Debug(fmt.Sprintf("GetHoverInformation: Starting hover request for URI: %s, Line: %d, Character: %d", uri, line, character))

	// Normalize URI to ensure proper file:// scheme
	normalizedURI := utils.NormalizeURI(uri)

	// Infer language from URI (use original URI for file extension detection)
	language, err := b.InferLanguage(uri)
	if err != nil {
		logger.Error("GetHoverInformation: Failed to infer language", fmt.Sprintf("URI: %s, Error: %v", uri, err))
		return nil, fmt.Errorf("failed to infer language: %w", err)
	}

	// Get language client
	client, err := b.GetClientForLanguage(string(*language))
	if err != nil {
		logger.Error("GetHoverInformation: Failed to get language client", fmt.Sprintf("Language: %s, Error: %v", string(*language), err))
		return nil, fmt.Errorf("failed to get client for language %s: %w", string(*language), err)
	}

	// Ensure the document is opened in the language server
	err = b.ensureDocumentOpen(client, normalizedURI, string(*language))
	if err != nil {
		// Continue anyway, as some servers might still work without explicit didOpen
		logger.Error("GetHoverInformation: Failed to open document", fmt.Sprintf("URI: %s, Error: %v", normalizedURI, err))
	}

	result, err := client.Hover(normalizedURI, line, character)

	if err != nil {
		logger.Error("GetHoverInformation: Request failed", fmt.Sprintf("Language: %s, Error: %v", string(*language), err))
		return nil, fmt.Errorf("hover request failed: %w", err)
	}

	return result, nil
}

// ensureDocumentOpen sends a textDocument/didOpen notification to the language server
// This is often required before other document operations can be performed
func (b *MCPLSPBridge) ensureDocumentOpen(client types.LanguageClientInterface, uri, language string) error {
	// Read the file content
	// Remove file:// prefix to get the actual file path
	filePath := strings.TrimPrefix(uri, "file://")

	projectRoots := client.ProjectRoots()

	// Clean the path to resolve .. and . elements
	cleanPath := filepath.Clean(filePath)

	// Convert to absolute path
	absPath, err := filepath.Abs(cleanPath)
	if err != nil {
		return fmt.Errorf("invalid file path: %w", err)
	}

	for _, allowedBaseDir := range projectRoots {
		// Validate against allowed base directory
		if !security.IsWithinAllowedDirectory(absPath, allowedBaseDir) {
			return errors.New("access denied: path outside allowed directory")
		}
	}

	content, err := os.ReadFile(filePath)
	if err != nil {
		return fmt.Errorf("failed to read file %s: %w", filePath, err)
	}

	// Send textDocument/didOpen notification
	didOpenParams := protocol.DidOpenTextDocumentParams{
		TextDocument: protocol.TextDocumentItem{
			Uri:        protocol.DocumentUri(uri),
			LanguageId: protocol.LanguageKind(language),
			Version:    1,
			Text:       string(content),
		},
	}

	err = client.SendNotification("textDocument/didOpen", didOpenParams)
	if err != nil {
		return fmt.Errorf("failed to send didOpen notification: %w", err)
	}

	logger.Debug(fmt.Sprintf("Document opened in LSP server: %s (language: %s)", uri, language))

	return nil
}

// GetDocumentSymbols gets all symbols in a document
func (b *MCPLSPBridge) GetDocumentSymbols(uri string) ([]protocol.DocumentSymbol, error) {
	// Normalize URI to ensure proper file:// scheme
	normalizedURI := utils.NormalizeURI(uri)
	logger.Debug(fmt.Sprintf("GetDocumentSymbols: Starting request for URI: %s -> %s", uri, normalizedURI))

	// Infer language from URI
	language, err := b.InferLanguage(uri)
	if err != nil {
		logger.Error("GetDocumentSymbols: Failed to infer language", fmt.Sprintf("URI: %s, Error: %v", uri, err))
		return nil, fmt.Errorf("failed to infer language: %w", err)
	}

	logger.Debug(fmt.Sprintf("GetDocumentSymbols: Inferred language: %s", *language))

	// Get language client
	client, err := b.GetClientForLanguage(string(*language))
	if err != nil {
		logger.Error("GetDocumentSymbols: Failed to get language client", fmt.Sprintf("Language: %s, Error: %v", string(*language), err))
		return nil, fmt.Errorf("failed to get client for language %s: %w", string(*language), err)
	}

	// Additional debugging: check client status
	metrics := client.GetMetrics()
	logger.Debug(fmt.Sprintf("GetDocumentSymbols: Client metrics: %+v", metrics))

	// Ensure the document is opened in the language server
	err = b.ensureDocumentOpen(client, normalizedURI, string(*language))
	if err != nil {
		// Continue anyway, as some servers might still work without explicit didOpen
		logger.Error("GetDocumentSymbols: Failed to open document", fmt.Sprintf("URI: %s, Error: %v", normalizedURI, err))
	}

	// Get document symbols
	symbols, err := client.DocumentSymbols(normalizedURI)
	if err != nil {
		logger.Error("GetDocumentSymbols: Request failed", fmt.Sprintf("Language: %s, Error: %v", string(*language), err))
		return nil, fmt.Errorf("document symbols request failed: %w", err)
	}

	logger.Debug(fmt.Sprintf("GetDocumentSymbols: Found %d symbols", len(symbols)))

	return symbols, nil
}

func (b *MCPLSPBridge) GetServerConfig(language string) (types.LanguageServerConfigProvider, error) {
	languageServer, err := b.GetConfig().FindServerConfig(language)
	if err != nil {
		return nil, err
	}

	return languageServer, nil
}

// GetSignatureHelp gets signature help for a function at a specific position
func (b *MCPLSPBridge) GetSignatureHelp(uri string, line, character uint32) (*protocol.SignatureHelp, error) {
	// Normalize URI
	normalizedURI := utils.NormalizeURI(uri)

	// Infer language from URI
	language, err := b.InferLanguage(normalizedURI)
	if err != nil {
		logger.Error("GetSignatureHelp: Language inference failed", fmt.Sprintf("URI: %s, Error: %v", normalizedURI, err))
		return nil, fmt.Errorf("failed to infer language: %w", err)
	}

	// Get language client
	client, err := b.GetClientForLanguage(string(*language))
	if err != nil {
		logger.Error("GetSignatureHelp: Client creation failed", fmt.Sprintf("Language: %s, Error: %v", string(*language), err))
		return nil, fmt.Errorf("failed to get client for language %s: %w", string(*language), err)
	}

	// Ensure document is open in the language server
	err = b.ensureDocumentOpen(client, normalizedURI, string(*language))
	if err != nil {
		// Continue anyway, as some servers might still work without explicit didOpen
		logger.Error("GetSignatureHelp: Failed to open document", fmt.Sprintf("URI: %s, Error: %v", normalizedURI, err))
	}

	// Execute signature help request using the LSP client method
	var signatureHelp *protocol.SignatureHelp

	signatureHelp, err = client.SignatureHelp(normalizedURI, line, character)
	if err != nil {
		logger.Error("GetSignatureHelp: Request failed", fmt.Sprintf("Language: %s, Error: %v", string(*language), err))
		return nil, fmt.Errorf("signature help request failed: %w", err)
	}

	// Check if the signatureHelp response is valid and contains signatures
	if signatureHelp == nil || len(signatureHelp.Signatures) == 0 {
		logger.Warn(fmt.Sprintf("GetSignatureHelp: No signatures found for position %d:%d in %s. Response was: %+v", line, character, normalizedURI, signatureHelp))
		// Return an empty result or a specific error indicating no signatures were found
		return nil, nil // Return empty, or you could return a specific error
	}

	logger.Debug(fmt.Sprintf("GetSignatureHelp: Found signature help for position %d:%d", line, character))

	return signatureHelp, nil
}

// GetCodeActions gets code actions for a specific range
func (b *MCPLSPBridge) GetCodeActions(uri string, line, character, endLine, endCharacter uint32) ([]protocol.CodeAction, error) {
	// Infer language from URI
	language, err := b.InferLanguage(uri)
	if err != nil {
		return nil, fmt.Errorf("failed to infer language: %w", err)
	}

	// Get language client
	client, err := b.GetClientForLanguage(string(*language))
	if err != nil {
		return nil, fmt.Errorf("failed to get client for language %s: %w", string(*language), err)
	}

	codeActions, err := client.CodeActions(uri, line, character, endLine, endCharacter)
	if err != nil {
		return nil, fmt.Errorf("failed to get code actions: %w", err)
	}

	return codeActions, nil
}

// FormatDocument formats a document
func (b *MCPLSPBridge) FormatDocument(uri string, tabSize uint32, insertSpaces bool) ([]protocol.TextEdit, error) {
	// Infer language from URI
	language, err := b.InferLanguage(uri)
	if err != nil {
		return nil, fmt.Errorf("failed to infer language: %w", err)
	}

	// Get language client
	client, err := b.GetClientForLanguage(string(*language))
	if err != nil {
		return nil, fmt.Errorf("failed to get client for language %s: %w", string(*language), err)
	}

	// Execute document formatting request
	params := protocol.DocumentFormattingParams{
		TextDocument: protocol.TextDocumentIdentifier{Uri: protocol.DocumentUri(uri)},
		Options: protocol.FormattingOptions{
			TabSize:      tabSize,
			InsertSpaces: insertSpaces,
		},
	}

	var result []protocol.TextEdit

	err = client.SendRequest("textDocument/formatting", params, &result, 10*time.Second)
	if err != nil {
		return nil, fmt.Errorf("document formatting request failed: %w", err)
	}

	return result, nil
}

// ApplyTextEdits applies text edits to a file
func (b *MCPLSPBridge) ApplyTextEdits(uri string, edits []protocol.TextEdit) error {
	// Convert URI to file path
	filePath := strings.TrimPrefix(uri, "file://")
	filePath, err := b.IsAllowedDirectory(filePath)
	// Check if file path is allowed
	if err != nil {
		return fmt.Errorf("file path is not allowed: %s", filePath)
	}

	// Read current file content
	content, err := os.ReadFile(filePath) // #nosec G304
	if err != nil {
		return fmt.Errorf("failed to read file %s: %w", filePath, err)
	}

	stat, err := os.Stat(filePath)
	if err != nil {
		return fmt.Errorf("failed to get file stats for %s: %w", filePath, err)
	}

	// Apply edits to content
	modifiedContent, err := applyTextEditsToContent(string(content), edits)
	if err != nil {
		return fmt.Errorf("failed to apply text edits: %w", err)
	}

	// Write modified content back to file
	err = os.WriteFile(filePath, []byte(modifiedContent), stat.Mode())
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

	// Split content into lines for easier manipulation. Use "\n" directly in the string
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

	// Use "\n" directly in the string
	return strings.Join(lines, "\n"), nil
}

// RenameSymbol renames a symbol with optional preview
func (b *MCPLSPBridge) RenameSymbol(uri string, line, character uint32, newName string, preview bool) (*protocol.WorkspaceEdit, error) {
	// Normalize URI to ensure proper file:// scheme
	normalizedURI := utils.NormalizeURI(uri)
	logger.Debug(fmt.Sprintf("RenameSymbol: Starting rename request for URI: %s -> %s, Line: %d, Character: %d, NewName: %s", uri, normalizedURI, line, character, newName))

	// Infer language from URI
	language, err := b.InferLanguage(uri)
	if err != nil {
		logger.Error("RenameSymbol: Failed to infer language", fmt.Sprintf("URI: %s, Error: %v", uri, err))
		return nil, fmt.Errorf("failed to infer language: %w", err)
	}

	// Get language client
	client, err := b.GetClientForLanguage(string(*language))
	if err != nil {
		logger.Error("RenameSymbol: Failed to get language client", fmt.Sprintf("Language: %s, Error: %v", string(*language), err))
		return nil, fmt.Errorf("failed to get client for language %s: %w", string(*language), err)
	}

	// Ensure the document is opened in the language server
	err = b.ensureDocumentOpen(client, normalizedURI, string(*language))
	if err != nil {
		// Continue anyway, as some servers might still work without explicit didOpen
		logger.Error("RenameSymbol: Failed to open document", fmt.Sprintf("URI: %s, Error: %v", normalizedURI, err))
	}

	result, err := client.Rename(normalizedURI, line, character, newName)
	if err != nil {
		logger.Error("RenameSymbol: Failed to rename symbol", fmt.Sprintf("URI: %s, Line: %d, Character: %d, NewName: %s, Error: %v", normalizedURI, line, character, newName, err))
		return nil, fmt.Errorf("failed to rename symbol: %w", err)
	}

	return result, nil
}

// ApplyWorkspaceEdit applies a workspace edit to multiple files
func (b *MCPLSPBridge) ApplyWorkspaceEdit(workspaceEdit *protocol.WorkspaceEdit) error {
	logger.Debug(fmt.Sprintf("ApplyWorkspaceEdit: Processing workspace edit. Changes: %+v, DocumentChanges: %+v", workspaceEdit.Changes, workspaceEdit.DocumentChanges))

	// Handle DocumentChanges format (preferred by most language servers)
	if workspaceEdit.DocumentChanges != nil {
		for _, docChange := range workspaceEdit.DocumentChanges {
			logger.Debug(fmt.Sprintf("ApplyWorkspaceEdit: Processing document change of type: %T", docChange.Value))

			// DocumentChanges is []Or4[TextDocumentEdit, CreateFile, RenameFile, DeleteFile]
			// We only handle TextDocumentEdit for now
			if textDocEdit, ok := docChange.Value.(protocol.TextDocumentEdit); ok {
				logger.Debug(fmt.Sprintf("ApplyWorkspaceEdit: Found TextDocumentEdit for URI: %s", textDocEdit.TextDocument.Uri))

				// Convert protocol.TextEdit to []any for ApplyTextEdits
				textEdits := make([]protocol.TextEdit, len(textDocEdit.Edits))

				for i, edit := range textDocEdit.Edits {
					// Edits might also be a union type, extract the actual TextEdit
					if actualEdit, ok := edit.Value.(protocol.TextEdit); ok {
						textEdits[i] = actualEdit
						logger.Debug(fmt.Sprintf("ApplyWorkspaceEdit: Edit %d - Line %d:%d-%d:%d, NewText: '%s'",
							i+1, actualEdit.Range.Start.Line, actualEdit.Range.Start.Character,
							actualEdit.Range.End.Line, actualEdit.Range.End.Character, actualEdit.NewText))
					} else {
						logger.Error("ApplyWorkspaceEdit: Edit is not a TextEdit", fmt.Sprintf("Type: %T", edit.Value))
						continue
					}
				}

				// Apply the edits
				if len(textEdits) > 0 {
					logger.Debug(fmt.Sprintf("ApplyWorkspaceEdit: Applying %d text edits to %s", len(textEdits), textDocEdit.TextDocument.Uri))

					err := b.ApplyTextEdits(string(textDocEdit.TextDocument.Uri), textEdits)
					if err != nil {
						return fmt.Errorf("failed to apply document changes to %s: %w", textDocEdit.TextDocument.Uri, err)
					}
				}
			} else if createFile, ok := docChange.Value.(protocol.CreateFile); ok {
				logger.Debug(fmt.Sprintf("ApplyWorkspaceEdit: Found CreateFile for URI: %s", createFile.Uri))
				filePath := strings.TrimPrefix(string(createFile.Uri), "file://")
				filePath, err := b.IsAllowedDirectory(filePath)
				if err != nil {
					return fmt.Errorf("failed to create file %s: %w", filePath, err)
				}
				// Create the file with default permissions (e.g., 0600)
				err = os.WriteFile(filePath, []byte{}, 0600)
				if err != nil {
					return fmt.Errorf("failed to create file %s: %w", filePath, err)
				}
				logger.Debug("ApplyWorkspaceEdit: Created file " + filePath)
			} else if renameFile, ok := docChange.Value.(protocol.RenameFile); ok {
				logger.Debug(fmt.Sprintf("ApplyWorkspaceEdit: Found RenameFile from %s to %s", renameFile.OldUri, renameFile.NewUri))
				oldPath := strings.TrimPrefix(string(renameFile.OldUri), "file://")
				newPath := strings.TrimPrefix(string(renameFile.NewUri), "file://")

				oldPath, err := b.IsAllowedDirectory(oldPath)
				if err != nil {
					return fmt.Errorf("failed to rename file (old path not allowed) %s: %w", oldPath, err)
				}
				newPath, err = b.IsAllowedDirectory(newPath)
				if err != nil {
					return fmt.Errorf("failed to rename file (new path not allowed) %s: %w", newPath, err)
				}

				err = os.Rename(oldPath, newPath)
				if err != nil {
					return fmt.Errorf("failed to rename file from %s to %s: %w", oldPath, newPath, err)
				}
				logger.Debug(fmt.Sprintf("ApplyWorkspaceEdit: Renamed file from %s to %s", oldPath, newPath))
			} else if deleteFile, ok := docChange.Value.(protocol.DeleteFile); ok {
				logger.Debug(fmt.Sprintf("ApplyWorkspaceEdit: Found DeleteFile for URI: %s", deleteFile.Uri))
				filePath := strings.TrimPrefix(string(deleteFile.Uri), "file://")
				filePath, err := b.IsAllowedDirectory(filePath)
				if err != nil {
					return fmt.Errorf("failed to delete file %s: %w", filePath, err)
				}
				err = os.Remove(filePath)
				if err != nil {
					return fmt.Errorf("failed to delete file %s: %w", filePath, err)
				}
				logger.Debug("ApplyWorkspaceEdit: Deleted file " + filePath)
			} else {
				logger.Debug(fmt.Sprintf("ApplyWorkspaceEdit: Skipping unknown document change type: %T", docChange.Value))
			}
		}
	}

	// Apply changes map (alternative format)
	if workspaceEdit.Changes != nil {
		for uri, edits := range workspaceEdit.Changes {
			err := b.ApplyTextEdits(string(uri), edits)
			if err != nil {
				return fmt.Errorf("failed to apply edits to %s: %w", uri, err)
			}
		}
	}

	return nil
}

// FindImplementations finds implementations of a symbol
func (b *MCPLSPBridge) FindImplementations(uri string, line, character uint32) ([]protocol.Location, error) {
	// Normalize URI
	normalizedURI := utils.NormalizeURI(uri)

	// Infer language from URI
	language, err := b.InferLanguage(normalizedURI)
	if err != nil {
		logger.Error("FindImplementations: Language inference failed", fmt.Sprintf("URI: %s, Error: %v", normalizedURI, err))
		return nil, fmt.Errorf("failed to infer language: %w", err)
	}

	// Get language client
	client, err := b.GetClientForLanguage(string(*language))
	if err != nil {
		logger.Error("FindImplementations: Client creation failed", fmt.Sprintf("Language: %s, Error: %v", string(*language), err))
		return nil, fmt.Errorf("failed to get client for language %s: %w", string(*language), err)
	}

	// Ensure document is open in the language server
	err = b.ensureDocumentOpen(client, normalizedURI, string(*language))
	if err != nil {
		// Continue anyway, as some servers might still work without explicit didOpen
		logger.Error("FindImplementations: Failed to open document", fmt.Sprintf("URI: %s, Error: %v", normalizedURI, err))
	}

	// Execute implementation request using the LSP client method
	implementations, err := client.Implementation(normalizedURI, line, character)
	if err != nil {
		logger.Error("FindImplementations: Request failed", fmt.Sprintf("Language: %s, Error: %v", string(*language), err))
		return nil, fmt.Errorf("implementation request failed: %w", err)
	}

	logger.Debug(fmt.Sprintf("FindImplementations: Found %d implementations", len(implementations)))

	return implementations, nil
}

func (b *MCPLSPBridge) SemanticTokens(uri string, targetTypes []string, startLine, startCharacter, endLine, endCharacter uint32) ([]types.TokenPosition, error) {
	language, err := b.InferLanguage(uri)
	if err != nil {
		return nil, fmt.Errorf("failed to infer language: %w", err)
	}

	client, err := b.GetClientForLanguage(string(*language))
	if err != nil {
		return nil, fmt.Errorf("failed to get client for language %s: %w", *language, err)
	}

	err = b.ensureDocumentOpen(client, uri, string(*language))
	if err != nil {
		// Continue anyway, as some servers might still work without explicit didOpen
		logger.Error("SemanticTokens: Failed to open document", fmt.Sprintf("URI: %s, Error: %v", uri, err))
	}

	tokens, err := client.SemanticTokensRange(uri, startLine, startCharacter, endLine, endCharacter)
	if err != nil {
		logger.Error(fmt.Sprintf("SemanticTokens: Failed to get raw semantic tokens from client: %v", err))
		serverCommand := client.GetMetrics().GetCommand()
		return nil, fmt.Errorf("semantic tokens not supported by %s language server for %s files: %w", serverCommand, *language, err)
	}

	// Handle nil tokens response (server returned null)
	if tokens == nil {
		logger.Debug("SemanticTokens: Server returned null/no semantic tokens for this range")
		return []types.TokenPosition{}, nil
	}

	logger.Debug(fmt.Sprintf("SemanticTokens: Raw tokens from LSP client: %+v", tokens))

	logger.Debug("SemanticTokens: About to get token parser")
	parser := client.TokenParser()
	logger.Debug(fmt.Sprintf("SemanticTokens: Got token parser: %v", parser != nil))

	if parser == nil {
		// If no token parser exists but the LSP request succeeded,
		// the server supports semantic tokens but didn't advertise capabilities properly.
		// This is common with some language servers like gopls.
		serverCommand := client.GetMetrics().GetCommand()
		logger.Debug(fmt.Sprintf("SemanticTokens: %s server for %s files supports semantic tokens but didn't advertise capabilities - creating fallback parser", serverCommand, *language))

		// Create a fallback parser with common token types
		fallbackTokenTypes := []string{
			"keyword", "class", "interface", "enum", "function", "method", "macro", "variable",
			"parameter", "property", "label", "comment", "string", "number", "regexp",
			"operator", "decorator", "type", "typeParameter", "namespace", "struct",
			"event", "operator", "modifier", "punctuation", "bracket", "delimiter",
		}
		fallbackTokenModifiers := []string{
			"declaration", "definition", "readonly", "static", "deprecated", "abstract",
			"async", "modification", "documentation", "defaultLibrary",
		}

		// Use the semantic token parser constructor directly
		parser = lsp.NewSemanticTokenParser(fallbackTokenTypes, fallbackTokenModifiers)

		if parser == nil {
			return nil, errors.New("failed to create fallback token parser")
		}

		logger.Debug("SemanticTokens: Created fallback token parser successfully")
	}

	tokenRange := protocol.Range{
		Start: protocol.Position{
			Line:      startLine,
			Character: startCharacter,
		},
		End: protocol.Position{
			Line:      endLine,
			Character: endCharacter,
		},
	}

	// []string{"type", "class", "interface", "struct"}
	positions, err := parser.FindTokensByType(tokens, targetTypes, tokenRange)

	if err != nil {
		return nil, fmt.Errorf("failed to find tokens by types (%v): %w", targetTypes, err)
	}

	return positions, nil
}

// PrepareCallHierarchy prepares call hierarchy items
func (b *MCPLSPBridge) PrepareCallHierarchy(uri string, line, character uint32) ([]protocol.CallHierarchyItem, error) {
	// Infer language from URI
	language, err := b.InferLanguage(uri)
	if err != nil {
		return nil, err
	}

	// Get language client
	client, err := b.GetClientForLanguage(string(*language))
	if err != nil {
		return nil, err
	}

	result, err := client.PrepareCallHierarchy(uri, line, character)
	if err != nil {
		return nil, err
	}

	return result, nil
}

// IncomingCalls gets incoming calls for a call hierarchy item
func (b *MCPLSPBridge) IncomingCalls(item protocol.CallHierarchyItem) ([]protocol.CallHierarchyIncomingCall, error) {
	// Infer language from the URI in the call hierarchy item
	language, err := b.InferLanguage(string(item.Uri))
	if err != nil {
		return nil, fmt.Errorf("failed to infer language from URI %s: %w", item.Uri, err)
	}

	// Get the language client for the inferred language
	client, err := b.GetClientForLanguage(string(*language))
	if err != nil {
		return nil, fmt.Errorf("failed to get language client for %s: %w", *language, err)
	}

	// Call the language client's IncomingCalls method
	return client.IncomingCalls(item)
}

// OutgoingCalls gets outgoing calls for a call hierarchy item
func (b *MCPLSPBridge) OutgoingCalls(item protocol.CallHierarchyItem) ([]protocol.CallHierarchyOutgoingCall, error) {
	// Infer language from the URI in the call hierarchy item
	language, err := b.InferLanguage(string(item.Uri))
	if err != nil {
		return nil, fmt.Errorf("failed to infer language from URI %s: %w", item.Uri, err)
	}

	// Get the language client for the inferred language
	client, err := b.GetClientForLanguage(string(*language))
	if err != nil {
		return nil, fmt.Errorf("failed to get language client for %s: %w", *language, err)
	}

	// Call the language client's OutgoingCalls method
	return client.OutgoingCalls(item)
}

// GetWorkspaceDiagnostics gets diagnostics for entire workspace
func (b *MCPLSPBridge) GetWorkspaceDiagnostics(workspaceUri string, identifier string) ([]protocol.WorkspaceDiagnosticReport, error) {
	// 1. Detect project languages or use multi-language approach
	languages, err := b.DetectProjectLanguages(workspaceUri)
	if err != nil {
		return []protocol.WorkspaceDiagnosticReport{}, err
	}

	if len(languages) == 0 {
		return []protocol.WorkspaceDiagnosticReport{}, nil // No languages detected, return empty result
	}

	var languageStrings []string
	for _, lang := range languages {
		languageStrings = append(languageStrings, string(lang))
	}

	// 2. Get language clients for detected languages
	clients, err := b.GetMultiLanguageClients(languageStrings)
	if err != nil {
		return nil, fmt.Errorf("failed to get language clients: %w", err)
	}

	// 3. Execute workspace diagnostic requests
	var allReports []protocol.WorkspaceDiagnosticReport

	for language, clientInterface := range clients {
		client := clientInterface

		report, err := client.WorkspaceDiagnostic(identifier)
		if err != nil {
			logger.Warn(fmt.Sprintf("Workspace diagnostics failed for %s: %v", language, err))
			continue
		}

		allReports = append(allReports, *report)
	}

	return allReports, nil
}

// GetDocumentDiagnostics gets diagnostics for a single document using LSP 3.17+ textDocument/diagnostic method
func (b *MCPLSPBridge) GetDocumentDiagnostics(uri string, identifier string, previousResultId string) (*protocol.DocumentDiagnosticReport, error) {
	// Normalize URI
	normalizedURI := utils.NormalizeURI(uri)

	// Determine language from file extension
	language, err := b.InferLanguage(normalizedURI)
	if err != nil {
		return nil, fmt.Errorf("failed to determine language for URI %s: %w", normalizedURI, err)
	}

	// Get client for the language
	client, err := b.GetClientForLanguage(string(*language))
	if err != nil {
		return nil, fmt.Errorf("failed to get client for language %s: %w", string(*language), err)
	}

	// Ensure document is open in the language server
	err = b.ensureDocumentOpen(client, normalizedURI, string(*language))
	if err != nil {
		// Continue anyway, as some servers might still work without explicit didOpen
		logger.Warn(fmt.Sprintf("GetDocumentDiagnostics: Failed to open document %s: %v", normalizedURI, err))
	}

	// Request document diagnostics
	report, err := client.DocumentDiagnostics(normalizedURI, identifier, previousResultId)
	if err != nil {
		return nil, fmt.Errorf("document diagnostics request failed: %w", err)
	}

	return report, nil
}
