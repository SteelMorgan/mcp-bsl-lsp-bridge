package bridge

import (
	"sync"

	"rockerboo/mcp-lsp-bridge/types"
	"rockerboo/mcp-lsp-bridge/utils"

	"github.com/mark3labs/mcp-go/server"
)

// MCPLSPBridge combines MCP server capabilities with multiple LSP clients
type MCPLSPBridge struct {
	server             *server.MCPServer
	clients            map[types.LanguageServer]types.LanguageClientInterface
	config             types.LSPServerConfigProvider
	allowedDirectories []string
	pathMapper         *utils.DockerPathMapper
	mu                 sync.RWMutex
}
