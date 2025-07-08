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

// safeUint32 safely converts an int32 to uint32, checking for overflow
func safeUint32FromInt32(val int32) (uint32, error) {
	if val < 0 {
		return 0, fmt.Errorf("value cannot be negative: %d", val)
	}
	return uint32(val), nil
}

// AnalyzeCode provides comprehensive code analysis for a given file and position
func AnalyzeCode(client *LanguageClient, opts AnalyzeCodeOptions) (*AnalyzeCodeResult, error) {
	result := &AnalyzeCodeResult{}

	uri := protocol.DocumentUri(opts.Uri)

	// Safe conversions for opts.Line and opts.Character
	lineUint32, err := safeUint32FromInt32(opts.Line)
	if err != nil {
		return nil, fmt.Errorf("invalid line number: %v", err)
	}
	characterUint32, err := safeUint32FromInt32(opts.Character)
	if err != nil {
		return nil, fmt.Errorf("invalid character position: %v", err)
	}

	// Hover request
	hoverParams := protocol.HoverParams{
		TextDocument: protocol.TextDocumentIdentifier{Uri: uri},
		Position: protocol.Position{
			Line:      lineUint32,
			Character: characterUint32,
		},
	}

	var hoverResult protocol.HoverResponse

	err = client.SendRequest("textDocument/hover", hoverParams, &hoverResult, 5*time.Second)
	if err == nil {
		result.Hover = &hoverResult
	} else {
		logger.Error(fmt.Sprintf("Hover request failed: %v", err))
	}

	// Completion request
	completionParams := protocol.CompletionParams{
		TextDocument: protocol.TextDocumentIdentifier{Uri: uri},
		Position: protocol.Position{
			Line:      lineUint32,
			Character: characterUint32,
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
			Line:      lineUint32,
			Character: characterUint32,
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

	// Code actions request with safe bounds checking
	endCharacterInt32 := max(0, opts.Character+1)
	endCharacterUint32, err := safeUint32FromInt32(endCharacterInt32)
	if err != nil {
		return nil, fmt.Errorf("invalid end character position: %v", err)
	}

	actionParams := protocol.CodeActionParams{
		TextDocument: protocol.TextDocumentIdentifier{Uri: uri},
		Range: protocol.Range{
			Start: protocol.Position{
				Line:      lineUint32,
				Character: characterUint32,
			},
			End: protocol.Position{
				Line:      lineUint32,
				Character: endCharacterUint32,
			},
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
