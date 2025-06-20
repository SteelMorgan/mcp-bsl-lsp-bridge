package mcpserver

import (
	"rockerboo/mcp-lsp-bridge/interfaces"
	"rockerboo/mcp-lsp-bridge/mcpserver/tools"

	"github.com/mark3labs/mcp-go/server"
)

// RegisterAllTools registers all MCP tools with the server
func RegisterAllTools(mcpServer *server.MCPServer, bridge interfaces.BridgeInterface) {
	// Core analysis tools
	tools.RegisterAnalyzeCodeTool(mcpServer, bridge)
	tools.RegisterProjectAnalysisTool(mcpServer, bridge)
	tools.RegisterInferLanguageTool(mcpServer, bridge)
	tools.RegisterProjectLanguageDetectionTool(mcpServer, bridge)

	// LSP connection management
	tools.RegisterLSPConnectTool(mcpServer, bridge)
	tools.RegisterLSPDisconnectTool(mcpServer, bridge)

	// Code intelligence tools
	tools.RegisterHoverTool(mcpServer, bridge)
	tools.RegisterSignatureHelpTool(mcpServer, bridge)
	tools.RegisterDiagnosticsTool(mcpServer, bridge)

	// Code improvement tools
	tools.RegisterCodeActionsTool(mcpServer, bridge)
	tools.RegisterFormatDocumentTool(mcpServer, bridge)

	// Advanced navigation tools
	tools.RegisterRenameTool(mcpServer, bridge)
	tools.RegisterImplementationTool(mcpServer, bridge)
	tools.RegisterCallHierarchyTool(mcpServer, bridge)

	// Workspace analysis
	tools.RegisterWorkspaceDiagnosticsTool(mcpServer, bridge)
}