package lsp

import (
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/myleshyson/lsprotocol-go/protocol"
	"rockerboo/mcp-lsp-bridge/logger"
)

// LSP Protocol Method Implementations

// Initialize sends an initialize request to the language server
func (lc *LanguageClient) Initialize(params protocol.InitializeParams) (*protocol.InitializeResult, error) {
	// Check connection status before sending request
	logger.Debug(fmt.Sprintf("STATUS: Initialize - About to call SendRequest, ctx.Err()=%v", lc.ctx.Err()))
	select {
	case <-lc.conn.DisconnectNotify():
		logger.Error("STATUS: Initialize - Connection already disconnected!")
		return nil, errors.New("connection already disconnected")
	default:
		logger.Debug("STATUS: Initialize - Connection appears healthy")
	}

	var result protocol.InitializeResult

	err := lc.SendRequest("initialize", params, &result, 5*time.Second)
	if err != nil {
		return nil, err
	}

	return &result, nil
}

// Initialized sends the initialized notification
func (lc *LanguageClient) Initialized() error {
	return lc.SendNotification("initialized", protocol.InitializedParams{})
}

// Shutdown sends a shutdown request
func (lc *LanguageClient) Shutdown() error {
	var result protocol.ShutdownResponse
	return lc.SendRequest("shutdown", nil, &result, 5*time.Second)
}

// Exit sends an exit notification
func (lc *LanguageClient) Exit() error {
	return lc.SendNotification("exit", nil)
}

// DidOpen sends a textDocument/didOpen notification
func (lc *LanguageClient) DidOpen(uri string, languageId protocol.LanguageKind, text string, version int32) error {
	params := protocol.DidOpenTextDocumentParams{
		TextDocument: protocol.TextDocumentItem{
			Uri:        protocol.DocumentUri(uri),
			LanguageId: languageId,
			Version:    version,
			Text:       text,
		},
	}

	return lc.SendNotification("textDocument/didOpen", params)
}

// DidChange sends a textDocument/didChange notification
func (lc *LanguageClient) DidChange(uri string, version int32, changes []protocol.TextDocumentContentChangeEvent) error {
	params := protocol.DidChangeTextDocumentParams{
		TextDocument: protocol.VersionedTextDocumentIdentifier{
			Uri:     protocol.DocumentUri(uri),
			Version: version,
		},
		ContentChanges: changes,
	}

	return lc.SendNotification("textDocument/didChange", params)
}

// DidSave sends a textDocument/didSave notification
func (lc *LanguageClient) DidSave(uri string, text *string) error {
	params := map[string]interface{}{
		"textDocument": map[string]interface{}{
			"uri": uri,
		},
	}
	if text != nil {
		params["text"] = *text
	}

	return lc.SendNotification("textDocument/didSave", params)
}

// DidClose sends a textDocument/didClose notification
func (lc *LanguageClient) DidClose(uri string) error {
	params := protocol.DidCloseTextDocumentParams{
		TextDocument: protocol.TextDocumentIdentifier{
			Uri: protocol.DocumentUri(uri),
		},
	}

	return lc.SendNotification("textDocument/didClose", params)
}

func (lc *LanguageClient) WorkspaceSymbols(query string) ([]protocol.WorkspaceSymbol, error) {
	var result []protocol.WorkspaceSymbol

	err := lc.SendRequest("workspace/symbol", protocol.WorkspaceSymbolParams{
		Query: query,
	}, &result, 5*time.Second)
	if err != nil {
		return nil, err
	}

	return result, nil
}

// Definition requests definition locations for a symbol at a given position
// Returns LocationLink[] or converts Location[] to LocationLink[]
func (lc *LanguageClient) Definition(uri string, line, character uint32) ([]protocol.Or2[protocol.LocationLink, protocol.Location], error) {
	// Use raw JSON response to handle both Location[] and LocationLink[] formats
	var rawResult json.RawMessage

	err := lc.SendRequest("textDocument/definition", protocol.DefinitionParams{
		TextDocument: protocol.TextDocumentIdentifier{
			Uri: protocol.DocumentUri(uri),
		},
		Position: protocol.Position{
			Line:      line,
			Character: character,
		},
	}, &rawResult, 5*time.Second)
	if err != nil {
		return nil, err
	}

	// First try to unmarshal as LocationLink[]
	var links []protocol.Or2[protocol.LocationLink, protocol.Location]

	err = json.Unmarshal(rawResult, &links)
	if err != nil {
		return nil, fmt.Errorf("failed to unmarshal definition response: %w", err)
	}

	return links, nil
}

// References finds all references to a symbol at a given position
func (lc *LanguageClient) References(uri string, line, character uint32, includeDeclaration bool) ([]protocol.Location, error) {
	var result []protocol.Location

	err := lc.SendRequest("textDocument/references", protocol.ReferenceParams{
		TextDocument: protocol.TextDocumentIdentifier{
			Uri: protocol.DocumentUri(uri),
		},
		Position: protocol.Position{
			Line:      line,
			Character: character,
		},
		Context: protocol.ReferenceContext{
			IncludeDeclaration: includeDeclaration,
		},
	}, &result, 5*time.Second)
	if err != nil {
		return nil, err
	}

	return result, nil
}

// Hover provides hover information at a given position
func (lc *LanguageClient) Hover(uri string, line, character uint32) (*protocol.Hover, error) {
	params := protocol.HoverParams{
		TextDocument: protocol.TextDocumentIdentifier{
			Uri: protocol.DocumentUri(uri),
		},
		Position: protocol.Position{
			Line:      line,
			Character: character,
		},
	}

	var rawResponse json.RawMessage

	err := lc.SendRequest("textDocument/hover", params, &rawResponse, 5*time.Second)
	if err != nil {
		return nil, err
	}

	// Handle null response - server has no hover information
	if len(rawResponse) == 4 && string(rawResponse) == "null" {
		return nil, nil
	}

	var result protocol.Hover

	err = json.Unmarshal(rawResponse, &result)
	if err != nil {
		return nil, fmt.Errorf("failed to unmarshal hover response: %w", err)
	}

	return &result, nil
}

// DocumentSymbols returns all symbols in a document
func (lc *LanguageClient) DocumentSymbols(uri string) ([]protocol.DocumentSymbol, error) {
	// Try DocumentSymbol[] first (newer format)
	var symbolResult []protocol.DocumentSymbol
	err := lc.SendRequest("textDocument/documentSymbol", protocol.DocumentSymbolParams{
		TextDocument: protocol.TextDocumentIdentifier{
			Uri: protocol.DocumentUri(uri),
		},
	}, &symbolResult, 5*time.Second)

	if err == nil && len(symbolResult) > 0 {
		return symbolResult, nil
	}

	// Fallback to SymbolInformation[] (older format)
	var infoResult []protocol.SymbolInformation
	err = lc.SendRequest("textDocument/documentSymbol", protocol.DocumentSymbolParams{
		TextDocument: protocol.TextDocumentIdentifier{
			Uri: protocol.DocumentUri(uri),
		},
	}, &infoResult, 5*time.Second)

	if err != nil {
		return nil, err
	}

	// Convert SymbolInformation[] to DocumentSymbol[]
	result := make([]protocol.DocumentSymbol, len(infoResult))
	for i, info := range infoResult {
		result[i] = protocol.DocumentSymbol{
			Name:           info.Name,
			Kind:           info.Kind,
			Range:          info.Location.Range,
			SelectionRange: info.Location.Range, // For SymbolInformation, this is the best we can do
			// Note: Children will be empty since SymbolInformation is flat
		}
	}

	return result, nil
}

// Implementation finds implementations of a symbol at a given position
func (lc *LanguageClient) Implementation(uri string, line, character uint32) ([]protocol.Location, error) {
	var result []protocol.Location

	err := lc.SendRequest("textDocument/implementation", protocol.ImplementationParams{
		TextDocument: protocol.TextDocumentIdentifier{
			Uri: protocol.DocumentUri(uri),
		},
		Position: protocol.Position{
			Line:      line,
			Character: character,
		},
	}, &result, 5*time.Second)
	if err != nil {
		return nil, err
	}

	return result, nil
}

// SignatureHelp provides signature help at a given position
func (lc *LanguageClient) SignatureHelp(uri string, line, character uint32) (*protocol.SignatureHelp, error) {
	params := protocol.SignatureHelpParams{
		TextDocument: protocol.TextDocumentIdentifier{
			Uri: protocol.DocumentUri(uri),
		},
		Position: protocol.Position{
			Line:      line,
			Character: character,
		},
	}

	var rawResponse json.RawMessage

	err := lc.SendRequest("textDocument/signatureHelp", params, &rawResponse, 5*time.Second)
	if err != nil {
		return nil, err
	}

	// Handle null response - server has no signature help available
	if len(rawResponse) == 4 && string(rawResponse) == "null" {
		return nil, nil
	}

	var result protocol.SignatureHelp

	err = json.Unmarshal(rawResponse, &result)
	if err != nil {
		return nil, err
	}

	return &result, nil
}

func (lc *LanguageClient) CodeActions(uri string, line, character, endLine, endCharacter uint32) ([]protocol.CodeAction, error) {

	params := protocol.CodeActionParams{
		TextDocument: protocol.TextDocumentIdentifier{Uri: protocol.DocumentUri(uri)},
		Range: protocol.Range{
			Start: protocol.Position{Line: line, Character: character},
			End:   protocol.Position{Line: endLine, Character: endCharacter},
		},
		Context: protocol.CodeActionContext{
			// Context can be empty for general code actions
		},
	}

	var result []protocol.CodeAction

	err := lc.SendRequest("textDocument/codeAction", params, &result, 5*time.Second)
	if err != nil {
		return nil, fmt.Errorf("code action request failed: %w", err)
	}

	return result, nil
}

func (lc *LanguageClient) Rename(uri string, line, character uint32, newName string) (*protocol.WorkspaceEdit, error) {
	params := protocol.RenameParams{
		TextDocument: protocol.TextDocumentIdentifier{Uri: protocol.DocumentUri(uri)},
		Position: protocol.Position{
			Line:      line,
			Character: character,
		},
		NewName: newName,
	}

	var result protocol.WorkspaceEdit

	err := lc.SendRequest("textDocument/rename", params, &result, 10*time.Second)
	if err != nil {
		return nil, fmt.Errorf("rename request failed: %w", err)
	}

	return &result, nil
}

func (lc *LanguageClient) WorkspaceDiagnostic(identifier string) (*protocol.WorkspaceDiagnosticReport, error) {
	params := protocol.WorkspaceDiagnosticParams{
		Identifier:        identifier,
		PreviousResultIds: []protocol.PreviousResultId{}, // Empty for first request
	}

	var result protocol.WorkspaceDiagnosticReport

	err := lc.SendRequest("workspace/diagnostic", params, &result, 30*time.Second) // Longer timeout for workspace operations
	if err != nil {
		return nil, fmt.Errorf("workspace diagnostic request failed: %w", err)
	}

	return &result, nil
}

func (lc *LanguageClient) Formatting(uri string, tabSize uint32, insertSpaces bool) ([]protocol.TextEdit, error) {
	params := protocol.DocumentFormattingParams{
		TextDocument: protocol.TextDocumentIdentifier{Uri: protocol.DocumentUri(uri)},
		Options: protocol.FormattingOptions{
			TabSize:      tabSize,
			InsertSpaces: insertSpaces,
		},
	}

	var result []protocol.TextEdit

	err := lc.SendRequest("textDocument/formatting", params, &result, 30*time.Second)
	if err != nil {
		return nil, fmt.Errorf("workspace diagnostic request failed: %w", err)
	}

	return result, nil
}

func (lc *LanguageClient) PrepareCallHierarchy(uri string, line, character uint32) ([]protocol.CallHierarchyItem, error) {
	params := protocol.CallHierarchyPrepareParams{
		TextDocument: protocol.TextDocumentIdentifier{Uri: protocol.DocumentUri(uri)},
		Position: protocol.Position{
			Line:      line,
			Character: character,
		},
	}

	var result []protocol.CallHierarchyItem

	err := lc.SendRequest("textDocument/prepareCallHierarchy", params, &result, 5*time.Second)
	if err != nil {
		return nil, fmt.Errorf("prepare call hierarchy request failed: %w", err)
	}

	return result, nil
}

func (lc *LanguageClient) SemanticTokens(uri string) (*protocol.SemanticTokens, error) {
	var rawResponse json.RawMessage

	err := lc.SendRequest("textDocument/semanticTokens", protocol.SemanticTokensParams{
		TextDocument: protocol.TextDocumentIdentifier{
			Uri: protocol.DocumentUri(uri),
		},
	}, &rawResponse, 5*time.Second)
	if err != nil {
		return nil, err
	}

	// Handle null response - server has no semantic tokens for this document
	if len(rawResponse) == 4 && string(rawResponse) == "null" {
		return nil, nil
	}

	var result protocol.SemanticTokens
	err = json.Unmarshal(rawResponse, &result)
	if err != nil {
		return nil, fmt.Errorf("failed to unmarshal semantic tokens response: %w", err)
	}

	return &result, nil
}

func (lc *LanguageClient) SemanticTokensRange(uri string, startLine, startCharacter, endLine, endCharacter uint32) (*protocol.SemanticTokens, error) {
	var rawResponse json.RawMessage

	err := lc.SendRequest("textDocument/semanticTokens/range", protocol.SemanticTokensRangeParams{
		TextDocument: protocol.TextDocumentIdentifier{
			Uri: protocol.DocumentUri(uri),
		},
		Range: protocol.Range{
			Start: protocol.Position{
				Line:      startLine,
				Character: startCharacter,
			},
			End: protocol.Position{
				Line:      endLine,
				Character: endCharacter,
			},
		},
	}, &rawResponse, 5*time.Second)
	if err != nil {
		return nil, err
	}

	// Handle null response - server has no semantic tokens for this range
	if len(rawResponse) == 4 && string(rawResponse) == "null" {
		logger.Debug("SemanticTokensRange: Server returned null response")
		return nil, nil
	}

	logger.Debug("SemanticTokensRange: Raw response: " + string(rawResponse))

	var result protocol.SemanticTokens
	err = json.Unmarshal(rawResponse, &result)
	if err != nil {
		logger.Error(fmt.Sprintf("SemanticTokensRange: Failed to unmarshal response: %v", err))
		return nil, fmt.Errorf("failed to unmarshal semantic tokens response: %w", err)
	}

	logger.Debug(fmt.Sprintf("SemanticTokensRange: Parsed result: %+v", result))
	return &result, nil
}

// IncomingCalls retrieves incoming calls for a given Call Hierarchy Item
func (lc *LanguageClient) IncomingCalls(item protocol.CallHierarchyItem) ([]protocol.CallHierarchyIncomingCall, error) {
	params := protocol.CallHierarchyIncomingCallsParams{
		Item: item,
	}

	var result []protocol.CallHierarchyIncomingCall

	err := lc.SendRequest("callHierarchy/incomingCalls", params, &result, 5*time.Second)
	if err != nil {
		return nil, fmt.Errorf("incoming calls request failed: %w", err)
	}

	return result, nil
}

// OutgoingCalls retrieves outgoing calls for a given Call Hierarchy Item
func (lc *LanguageClient) OutgoingCalls(item protocol.CallHierarchyItem) ([]protocol.CallHierarchyOutgoingCall, error) {
	params := protocol.CallHierarchyOutgoingCallsParams{
		Item: item,
	}

	var result []protocol.CallHierarchyOutgoingCall

	err := lc.SendRequest("callHierarchy/outgoingCalls", params, &result, 5*time.Second)
	if err != nil {
		return nil, fmt.Errorf("outgoing calls request failed: %w", err)
	}

	return result, nil
}

// DocumentDiagnostics gets diagnostics for a specific document using LSP 3.17+ textDocument/diagnostic method
func (lc *LanguageClient) DocumentDiagnostics(uri string, identifier string, previousResultId string) (*protocol.DocumentDiagnosticReport, error) {
	params := protocol.DocumentDiagnosticParams{
		TextDocument: protocol.TextDocumentIdentifier{
			Uri: protocol.DocumentUri(uri),
		},
	}

	// Add optional parameters if provided
	if identifier != "" {
		params.Identifier = identifier
	}
	if previousResultId != "" {
		params.PreviousResultId = previousResultId
	}

	var result protocol.DocumentDiagnosticReport

	err := lc.SendRequest("textDocument/diagnostic", params, &result, 10*time.Second)
	if err != nil {
		return nil, fmt.Errorf("document diagnostic request failed: %w", err)
	}

	return &result, nil
}
