package tools

import (
	"context"
	"fmt"
	"strings"

	"rockerboo/mcp-lsp-bridge/logger"
	"rockerboo/mcp-lsp-bridge/interfaces"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
)

// RegisterWorkspaceDiagnosticsTool registers the workspace_diagnostics tool
func RegisterWorkspaceDiagnosticsTool(mcpServer *server.MCPServer, bridge interfaces.BridgeInterface) {
	mcpServer.AddTool(mcp.NewTool("workspace_diagnostics",
		mcp.WithDescription("Get comprehensive diagnostics for entire workspace"),
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
		if strings.HasPrefix(workspaceUri, "file://") {
			workspaceUri = strings.TrimPrefix(workspaceUri, "file://")
			logger.Info("workspace_diagnostics: stripped file:// prefix", 
				fmt.Sprintf("Processed URI: %s", workspaceUri))
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