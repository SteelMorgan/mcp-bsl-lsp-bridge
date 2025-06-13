package lsp

import (
	"time"

	"github.com/myleshyson/lsprotocol-go/protocol"
)

// LSP Protocol Method Implementations

// Initialize sends an initialize request to the language server
func (lc *LanguageClient) Initialize(params protocol.InitializeParams) (*protocol.InitializeResult, error) {
	var result protocol.InitializeResult
	err := lc.SendRequest("initialize", params, &result, 10*time.Second)
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
	params := map[string]interface{}{
		"textDocument": map[string]interface{}{
			"uri": uri,
		},
	}
	return lc.SendNotification("textDocument/didClose", params)
}