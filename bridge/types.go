package bridge

import (
	"rockerboo/mcp-lsp-bridge/lsp"

	"github.com/mark3labs/mcp-go/server"
	"github.com/myleshyson/lsprotocol-go/protocol"
)

// MCPLSPBridge combines MCP server capabilities with multiple LSP clients
type MCPLSPBridge struct {
	server        *server.MCPServer
	clients       map[string]*lsp.LanguageClient
	config        *lsp.LSPServerConfig
	currentClient *lsp.LanguageClient
}

// SemanticAnalysisResult holds the results of semantic analysis
type SemanticAnalysisResult struct {
	Symbols    []protocol.DocumentSymbol
	References []protocol.Location
}