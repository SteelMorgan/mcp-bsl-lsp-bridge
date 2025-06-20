package mcpserver

import (
	"context"

	"rockerboo/mcp-lsp-bridge/interfaces"
	"rockerboo/mcp-lsp-bridge/logger"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
)

// SetupMCPServer configures the MCP server with AI-powered tools
func SetupMCPServer(bridge interfaces.BridgeInterface) *server.MCPServer {
	hooks := &server.Hooks{}

	hooks.AddBeforeAny(func(ctx context.Context, id any, method mcp.MCPMethod, message any) {
		logger.Debug("beforeAny:", method, id, message)
	})
	hooks.AddOnSuccess(func(ctx context.Context, id any, method mcp.MCPMethod, message any, result any) {
		logger.Info("onSuccess:", method, id, message, result)
	})
	hooks.AddOnError(func(ctx context.Context, id any, method mcp.MCPMethod, message any, err error) {
		logger.Error("onError:", method, id, message, err)
	})
	hooks.AddBeforeInitialize(func(ctx context.Context, id any, message *mcp.InitializeRequest) {
		logger.Debug("beforeInitialize:", id, message)
	})
	hooks.AddOnRequestInitialization(func(ctx context.Context, id any, message any) error {
		logger.Debug("AddOnRequestInitialization:", id, message)
		// authorization verification and other preprocessing tasks are performed.
		return nil
	})
	hooks.AddAfterInitialize(func(ctx context.Context, id any, message *mcp.InitializeRequest, result *mcp.InitializeResult) {
		logger.Info("afterInitialize:", id, message, result)
	})
	hooks.AddAfterCallTool(func(ctx context.Context, id any, message *mcp.CallToolRequest, result *mcp.CallToolResult) {
		logger.Debug("afterCallTool:", id, message, result)
	})
	hooks.AddBeforeCallTool(func(ctx context.Context, id any, message *mcp.CallToolRequest) {
		logger.Debug("beforeCallTool:", id, message)
	})

	mcpServer := server.NewMCPServer(
		"mcp-lsp-bridge",
		"1.0.0",
		server.WithToolCapabilities(false),
		server.WithLogging(),
		server.WithHooks(hooks),
		server.WithInstructions(`This MCP server provides comprehensive Language Server Protocol (LSP) integration for advanced code analysis and manipulation across multiple programming languages.

## Available Tools (15 total)

### Core Analysis & Connection (5 tools)
- mcp__lsp__analyze_code: Get code completion suggestions and insights for specific positions
- mcp__lsp__infer_language: Detect programming language from file paths
- mcp__lsp__detect_project_languages: Analyze project structure to identify all used languages
- mcp__lsp__lsp_connect: Establish connection to appropriate language server
- mcp__lsp__lsp_disconnect: Clean up all active language server connections

### Code Intelligence (4 tools) 
- mcp__lsp__hover: Get symbol documentation, type information, and contextual help
- mcp__lsp__signature_help: Get function parameter assistance with overload information
- mcp__lsp__diagnostics: Retrieve errors, warnings, and diagnostic information for single files
- mcp__lsp__workspace_diagnostics: Get comprehensive diagnostics across entire workspace

### Code Improvement (2 tools)
- mcp__lsp__code_actions: Get quick fixes, refactoring suggestions, and code improvements
- mcp__lsp__format_document: Format code with customizable indentation and style options

### Advanced Navigation (3 tools)
- mcp__lsp__rename: Safely rename symbols across codebase with optional preview
- mcp__lsp__implementation: Find all implementations of interfaces and abstract methods
- mcp__lsp__call_hierarchy: Analyze function call relationships and dependencies

### Project Analysis (1 tool)
- mcp__lsp__project_analysis: Multi-mode analysis (workspace_symbols, references, definitions, text_search)

## Supported Languages
- Go (gopls), Python (pyright), TypeScript (typescript-language-server), Rust (rust-analyzer)

## Recommended Usage Patterns

1. **Start with language detection**: Use mcp__lsp__infer_language or mcp__lsp__detect_project_languages
2. **Establish connections**: Use mcp__lsp__lsp_connect before analysis operations
3. **Leverage intelligence tools**: Use hover for understanding, signature_help for function calls
4. **Use diagnostic tools**: Apply diagnostics for single files, workspace_diagnostics for project-wide analysis
5. **Use improvement tools**: Apply code_actions for fixes, format_document for cleanup
6. **Navigate efficiently**: Use implementation and call_hierarchy for code exploration
7. **Clean up**: Use mcp__lsp__lsp_disconnect when done to free resources

## Multi-Language Support
The bridge automatically detects file types and connects to appropriate language servers. It supports fallback mechanisms and graceful error handling when language servers are unavailable.

## Error Handling
All tools provide comprehensive error messages and logging. Failed operations return actionable error information rather than silent failures.`),
	)

	// Register all MCP tools
	RegisterAllTools(mcpServer, bridge)

	return mcpServer
}
