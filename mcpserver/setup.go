package mcpserver

import (
	"fmt"
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
		logger.Debug("onSuccess:", method, id, message, result)
	})
	hooks.AddOnError(func(ctx context.Context, id any, method mcp.MCPMethod, message any, err error) {
		logger.Error("onError:", method, id, message, err)
	})
	hooks.AddBeforeInitialize(func(ctx context.Context, id any, message *mcp.InitializeRequest) {
		logger.Debug("beforeInitialize:", id, message)
	})
	hooks.AddOnRequestInitialization(func(ctx context.Context, id any, message any) error {
		logger.Debug(fmt.Sprintf("AddOnRequestInitialization: %s %+v", id, message))
		// authorization verification and other preprocessing tasks are performed.
		return nil
	})
	hooks.AddAfterInitialize(func(ctx context.Context, id any, message *mcp.InitializeRequest, result *mcp.InitializeResult) {
		logger.Debug("afterInitialize:", id, message, result)
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

## Key Capabilities & Usage Flow

The bridge offers robust tools for:

1.  **Project Initialization**: Detect project languages and connect to relevant language servers.
2.  **Code Discovery & Understanding**:
    *   **Symbol Search**: Locate definitions, references, and usage of symbols project-wide.
    *   **Contextual Help**: Get documentation and type information for code elements.
    *   **Code Content**: Extract specific code blocks by range.
    *   **Diagnostics**: Identify errors, warnings, and overall project health.
3.  **Code Improvement & Navigation**:
    *   **Refactoring**: Apply quick fixes, suggestions, and safely rename symbols (preview changes first).
    *   **Formatting**: Standardize code style.
    *   **Navigation**: Trace implementations and function call hierarchies.
4.  **Resource Management**: Disconnect language servers when analysis is complete.

## Multi-Language Support
The bridge automatically detects file types and connects to appropriate language servers. It supports fallback mechanisms and provides actionable error messages.`),
	)

	// Register all MCP tools
	RegisterAllTools(mcpServer, bridge)

	return mcpServer
}
