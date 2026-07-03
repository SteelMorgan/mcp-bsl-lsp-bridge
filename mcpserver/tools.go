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

	// Completion / API discovery (platform-aware with BSL LS type-system v2 + bsl-context)
	tools.RegisterCompletionTool(mcpServer, bridge)
	tools.RegisterAnalyzeCodeTool(mcpServer, bridge)
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
	tools.RegisterDefinitionTool(mcpServer, bridge)
	tools.RegisterSelectionRangeTool(mcpServer, bridge)
	// Platform-aware features enabled with BSL LS type-system v2 + bsl-context:
	tools.RegisterSignatureHelpTool(mcpServer, bridge)
	tools.RegisterSemanticTokensTool(mcpServer, bridge)
	tools.RegisterInlayHintsTool(mcpServer, bridge)
	// tools.RegisterDiagnosticsTool(mcpServer, bridge)
	// Hide IDE/UI-oriented tools that don't help an AI agent much:
	// - folding_range
	// - document_link
	// - document_color
	// - color_presentation

	// Code improvement tools
	tools.RegisterCodeActionsTool(mcpServer, bridge)
	// tools.RegisterFormatDocumentTool(mcpServer, bridge) // BSL LS formatting подвисает/неполезно для агента
	// Hide IDE/UI-oriented tool:
	// - range_formatting
	tools.RegisterPrepareRenameTool(mcpServer, bridge)
	tools.RegisterRangeTools(mcpServer, bridge)

	// Advanced navigation tools
	tools.RegisterRenameTool(mcpServer, bridge)
	// tools.RegisterImplementationTool(mcpServer, bridge)  // BSL LS не поддерживает implementation

	// Call hierarchy tools
	tools.RegisterCallHierarchyTool(mcpServer, bridge)
	tools.RegisterCallGraphTool(mcpServer, bridge)
	// Combo: incoming callers + references + per-caller module-type classification (change-impact)
	tools.RegisterSymbolImpactTool(mcpServer, bridge)

	// Workspace analysis
	// tools.RegisterWorkspaceDiagnosticsTool(mcpServer, bridge) // Too heavy/noisy for AI agent workflows

	// Document diagnostics
	tools.RegisterDocumentDiagnosticsTool(mcpServer, bridge)
	// Targeted security/performance/SQL diagnostics (offline classification table)
	tools.RegisterQualityDiagnosticsTool(mcpServer, bridge)
	// Per-method cyclomatic + cognitive complexity (BSL LS complexity CodeLens)
	tools.RegisterComplexityTool(mcpServer, bridge)
	// Combo: complexity + quality_diagnostics merged per method, ranked by refactor priority
	tools.RegisterModuleHealthTool(mcpServer, bridge)

	// Workspace notifications and commands
	// did_change_watched_files - needed for notifying LSP about new files (essential for call_graph)
	tools.RegisterDidChangeWatchedFilesTool(mcpServer, bridge)
	// Hide other editor plumbing tools:
	// - execute_command
	// - did_change_configuration

	// Diagnostic tools
	// Hide bridge diagnostic tool from the agent tool list:
	// - mcp_lsp_diagnostics

	// Server/client status (includes LSP $/progress)
	tools.RegisterLSPStatusTool(mcpServer, bridge)

	// Multi-project control (active only when daemon MULTI_PROJECT=1)
	tools.RegisterProjectTools(mcpServer, bridge)

	// Upstream rlm-tools-bsl static search/navigation tools (thin local proxy).
	tools.RegisterRLMTools(mcpServer, bridge)
}
