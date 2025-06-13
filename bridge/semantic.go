package bridge

import (
	"time"

	"github.com/myleshyson/lsprotocol-go/protocol"
)

// ExtractSemanticInfo retrieves semantic information for a given file
func (b *MCPLSPBridge) ExtractSemanticInfo(language, fileUri string) (*SemanticAnalysisResult, error) {
	client, err := b.GetClientForLanguage(language)
	if err != nil {
		return nil, err
	}

	uri := protocol.DocumentUri(fileUri)

	var symbols []protocol.DocumentSymbol
	// Collect various semantic insights
	sym_err := client.SendRequest("textDocument/documentSymbol",
		protocol.DocumentSymbolParams{
			TextDocument: protocol.TextDocumentIdentifier{Uri: uri},
		},
		&symbols,
		1*time.Second,
	)

	if sym_err != nil {
		return nil, sym_err
	}

	var references []protocol.Location

	ref_err := client.SendRequest("textDocument/references",
		protocol.ReferenceParams{
			TextDocument: protocol.TextDocumentIdentifier{Uri: uri},
			Position:     protocol.Position{Line: 0, Character: 0},
		},
		&references,
		1*time.Second,
	)

	if ref_err != nil {
		return nil, ref_err
	}

	return &SemanticAnalysisResult{
		Symbols:    symbols,
		References: references,
		// other semantic information
	}, nil
}