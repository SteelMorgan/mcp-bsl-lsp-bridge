package bridge

import (
	"rockerboo/mcp-lsp-bridge/lsp"

	"github.com/mark3labs/mcp-go/server"
)

// MCPLSPBridge combines MCP server capabilities with multiple LSP clients
type MCPLSPBridge struct {
	server  *server.MCPServer
	clients map[lsp.Language]lsp.LanguageClientInterface
	config  lsp.LSPServerConfigProvider
	allowedDirectories []string
}
