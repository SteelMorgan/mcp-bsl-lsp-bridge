package mcpserver

import (
	"rockerboo/mcp-lsp-bridge/interfaces"
	"rockerboo/mcp-lsp-bridge/mcpserver/tools"
)

// Registers all MCP tools with the server
func RegisterAllTools(mcpServer tools.ToolServer, bridge interfaces.BridgeInterface) {
	// Core analysis tools

	// New unified symbol exploration tool
	tools.RegisterSymbolExploreTool(mcpServer, bridge)

	// Disabling lesser used tools
	// tools.RegisterAnalyzeCodeTool(mcpServer, bridge)
	tools.RegisterProjectAnalysisTool(mcpServer, bridge)

	// Language detection tools
	// NOTE: BSL projects are single-language in our usage, and MCP is connected manually.
	// Hide language detection tools from the MCP tool list:
	// - infer_language
	// - detect_project_languages

	// LSP connection management
	// Disabling lesser used tools
	// Auto-connect is performed at MCP initialize; hide connection management tools:
	// - lsp_connect
	// - lsp_disconnect

	// Code intelligence tools
	tools.RegisterHoverTool(mcpServer, bridge)
	tools.RegisterSignatureHelpTool(mcpServer, bridge)
	// tools.RegisterDiagnosticsTool(mcpServer, bridge)
	// Hide IDE/UI-oriented tools that don't help an AI agent much:
	// - semantic_tokens
	// - folding_range
	// - selection_range
	// - document_link
	// - document_color
	// - color_presentation

	// Code improvement tools
	tools.RegisterCodeActionsTool(mcpServer, bridge)
	tools.RegisterFormatDocumentTool(mcpServer, bridge)
	// Hide IDE/UI-oriented tool:
	// - range_formatting
	tools.RegisterPrepareRenameTool(mcpServer, bridge)
	tools.RegisterRangeTools(mcpServer, bridge)

	// Advanced navigation tools
	tools.RegisterRenameTool(mcpServer, bridge)
	tools.RegisterImplementationTool(mcpServer, bridge)

	// Call hierarchy tool
	tools.RegisterCallHierarchyTool(mcpServer, bridge)

	// Workspace analysis
	tools.RegisterWorkspaceDiagnosticsTool(mcpServer, bridge)

	// Document diagnostics
	tools.RegisterDocumentDiagnosticsTool(mcpServer, bridge)

	// Workspace notifications and commands
	// Hide tools that mostly serve editor plumbing:
	// - execute_command
	// - did_change_watched_files
	// - did_change_configuration

	// Diagnostic tools
	// Hide bridge diagnostic tool from the agent tool list:
	// - mcp_lsp_diagnostics

	// Server/client status (includes LSP $/progress)
	tools.RegisterLSPStatusTool(mcpServer, bridge)
}
