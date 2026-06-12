package tools

import (
	"context"
	"path/filepath"

	"rockerboo/mcp-lsp-bridge/interfaces"
	"rockerboo/mcp-lsp-bridge/logger"
	"rockerboo/mcp-lsp-bridge/lsp"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
)

// LSPStatusTool reports current LSP client status, including server-sent $/progress streams.
func LSPStatusTool(bridge interfaces.BridgeInterface) (mcp.Tool, server.ToolHandlerFunc) {
	return mcp.NewTool("lsp_status",
			mcp.WithDescription("Show current LSP connection status and server progress ($/progress). Useful for detecting whether a language server is still indexing or ready. In multi-project mode the response includes a per-project list (root + state ready/indexing); pass project_root to narrow it to one project."),
			mcp.WithDestructiveHintAnnotation(false),
			mcp.WithBoolean("include_progress", mcp.Description("Include server progress details in status output."), mcp.DefaultBool(true)),
			mcp.WithString("project_root", mcp.Description("Multi-project mode only: filter the per-project status to this project root (container path).")),
		), func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
			status, err := BuildLSPStatus(bridge)
			if err != nil {
				return mcp.NewToolResultError(err.Error()), nil
			}

			if projectRoot := request.GetString("project_root", ""); projectRoot != "" {
				status.Projects = filterProjects(status.Projects, projectRoot)
			}

			payload, err := FormatLSPStatus(status)
			if err != nil {
				return mcp.NewToolResultError(err.Error()), nil
			}
			logger.Debug("lsp_status: reported status for clients")
			return mcp.NewToolResultText(payload), nil
		}
}

// filterProjects returns only the project whose root matches (after cleaning).
func filterProjects(projects []lsp.ProjectStatus, root string) []lsp.ProjectStatus {
	want := filepath.Clean(root)
	out := make([]lsp.ProjectStatus, 0, 1)
	for _, p := range projects {
		if filepath.Clean(p.Root) == want {
			out = append(out, p)
		}
	}
	return out
}

func RegisterLSPStatusTool(mcpServer ToolServer, bridge interfaces.BridgeInterface) {
	tool, handler := LSPStatusTool(bridge)
	mcpServer.AddTool(tool, handler)
}
