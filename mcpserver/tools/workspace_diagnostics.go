package tools

import (
	"context"

	"strings"

	"rockerboo/mcp-lsp-bridge/interfaces"
	"rockerboo/mcp-lsp-bridge/logger"

	"github.com/mark3labs/mcp-go/mcp"
)

// RegisterWorkspaceDiagnosticsTool registers the workspace_diagnostics tool
func RegisterWorkspaceDiagnosticsTool(mcpServer ToolServer, bridge interfaces.BridgeInterface) {
	mcpServer.AddTool(mcp.NewTool("workspace_diagnostics",
		mcp.WithDescription("Get comprehensive diagnostics for entire workspace"),
		mcp.WithDestructiveHintAnnotation(false),
		mcp.WithString("workspace_uri", mcp.Description("URI to the workspace/project root")),
		mcp.WithString("identifier", mcp.Description("Optional identifier for diagnostic session")), // TODO: Add optional when supported
	), func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		// Parameter parsing
		workspaceUri, err := request.RequireString("workspace_uri")
		if err != nil {
			logger.Error("workspace_diagnostics: workspace_uri parsing failed", err)
			return mcp.NewToolResultError(err.Error()), nil
		}

		// Strip file:// prefix if present
		if after, ok := strings.CutPrefix(workspaceUri, "file://"); ok {
			workspaceUri = after
			logger.Info("workspace_diagnostics: stripped file:// prefix",
				"Processed URI: "+workspaceUri)
		}

		// Optional identifier
		identifier := "mcp-lsp-bridge-workspace-diagnostics"
		if id, err := request.RequireString("identifier"); err == nil {
			identifier = id
		}

		// Execute workspace diagnostics
		result, err := bridge.GetWorkspaceDiagnostics(workspaceUri, identifier)
		if err != nil {
			logger.Error("workspace_diagnostics: execution failed", err)
			return mcp.NewToolResultError(err.Error()), nil
		}

		// Format results for user-friendly output
		formattedResult := formatWorkspaceDiagnostics(result)

		return mcp.NewToolResultText(formattedResult), nil
	})
}
