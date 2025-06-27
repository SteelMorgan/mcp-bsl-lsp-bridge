package lsp

import (
	"fmt"
	"time"

	"rockerboo/mcp-lsp-bridge/logger"

	"github.com/myleshyson/lsprotocol-go/protocol"
)

// AnalyzeCodeOptions defines the configuration for code analysis
type AnalyzeCodeOptions struct {
	Uri        string
	Line       int32
	Character  int32
	LanguageId string
}

// AnalyzeCodeResult contains comprehensive code analysis insights
type AnalyzeCodeResult struct {
	Hover         *protocol.HoverResponse
	Completion    *protocol.CompletionResponse
	SignatureHelp *protocol.SignatureHelpResponse
	Diagnostics   []protocol.Diagnostic
	CodeActions   []protocol.CodeAction
}

// AnalyzeCode provides comprehensive code analysis for a given file and position
func AnalyzeCode(client *LanguageClient, opts AnalyzeCodeOptions) (*AnalyzeCodeResult, error) {
	result := &AnalyzeCodeResult{}

	uri := protocol.DocumentUri(opts.Uri)

	// Hover request
	hoverParams := protocol.HoverParams{
		TextDocument: protocol.TextDocumentIdentifier{Uri: uri},
		Position: protocol.Position{
			Line:      uint32(opts.Line),
			Character: uint32(opts.Character),
		},
	}

	var hoverResult protocol.HoverResponse

	err := client.SendRequest("textDocument/hover", hoverParams, &hoverResult, 5*time.Second)
	if err == nil {
		result.Hover = &hoverResult
	} else {
		logger.Error(fmt.Sprintf("Hover request failed: %v", err))
	}

	// Completion request
	completionParams := protocol.CompletionParams{
		TextDocument: protocol.TextDocumentIdentifier{Uri: uri},
		Position: protocol.Position{
			Line:      uint32(opts.Line),
			Character: uint32(opts.Character),
		},
	}

	var completionResult protocol.CompletionResponse

	err = client.SendRequest("textDocument/completion", completionParams, &completionResult, 5*time.Second)
	if err == nil {
		result.Completion = &completionResult
	} else {
		logger.Error(fmt.Sprintf("Completion request failed: %v", err))
	}

	// Signature help request
	signatureParams := protocol.SignatureHelpParams{
		TextDocument: protocol.TextDocumentIdentifier{Uri: uri},
		Position: protocol.Position{
			Line:      uint32(opts.Line),
			Character: uint32(opts.Character),
		},
	}

	var signatureResult protocol.SignatureHelpResponse

	err = client.SendRequest("textDocument/signatureHelp", signatureParams, &signatureResult, 5*time.Second)
	if err == nil {
		result.SignatureHelp = &signatureResult
	} else {
		logger.Error(fmt.Sprintf("Signature help request failed: %v", err))
	}

	// Note: Diagnostics are typically pushed by the server, so this is a simplified approach
	// Actual implementation may vary based on specific language server
	var diagnostics []protocol.Diagnostic
	// Placeholder for diagnostic retrieval logic
	result.Diagnostics = diagnostics

	// Code actions request
	actionParams := protocol.CodeActionParams{
		TextDocument: protocol.TextDocumentIdentifier{Uri: uri},
		Range: protocol.Range{
			Start: protocol.Position{Line: uint32(opts.Line), Character: uint32(opts.Character)},
			End:   protocol.Position{Line: uint32(opts.Line), Character: uint32(opts.Character + 1)},
		},
	}

	var codeActions []protocol.CodeAction

	err = client.SendRequest("textDocument/codeAction", actionParams, &codeActions, 5*time.Second)
	if err == nil {
		result.CodeActions = codeActions
	} else {
		logger.Error(fmt.Sprintf("Code action request failed: %v", err))
	}

	return result, nil
}
