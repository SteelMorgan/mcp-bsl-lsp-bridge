package tools

// MCP tools for multi-project control (daemon MULTI_PROJECT=1).
//
// File tools auto-register a project on first touch, so these tools are for
// explicit control: warming a project ahead of time, closing one to free
// memory, or inspecting the registry. They operate on container-namespace
// project roots (the directory holding Configuration.xml / src/cf / .git, etc.).

import (
	"context"
	"encoding/json"
	"fmt"

	bridgepkg "rockerboo/mcp-lsp-bridge/bridge"
	"rockerboo/mcp-lsp-bridge/interfaces"
	"rockerboo/mcp-lsp-bridge/logger"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
)

// projectBridge is the subset of the concrete bridge used by the project tools.
// Implemented by *bridgepkg.MCPLSPBridge.
type projectBridge interface {
	MultiProjectEnabled() bool
	ProjectAdd(root string) (map[string]interface{}, error)
	ProjectClose(root string) (map[string]interface{}, error)
	ProjectList() (map[string]interface{}, error)
}

func asProjectBridge(bridge interfaces.BridgeInterface) (projectBridge, error) {
	if b, ok := bridge.(*bridgepkg.MCPLSPBridge); ok {
		return b, nil
	}
	return nil, fmt.Errorf("project tools require the concrete bridge")
}

func formatProjectResult(v map[string]interface{}) (*mcp.CallToolResult, error) {
	raw, err := json.Marshal(v)
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}
	return mcp.NewToolResultText(string(raw)), nil
}

// ProjectAddTool registers/warms a 1C project as a workspace folder.
func ProjectAddTool(bridge interfaces.BridgeInterface) (mcp.Tool, server.ToolHandlerFunc) {
	return mcp.NewTool("project_add",
			mcp.WithDescription("Multi-project mode: register and warm a 1C project (workspace folder) so its files can be analyzed. Indexing starts in the background; poll lsp_status (project_root) for readiness. Projects stay warm until project_close or container shutdown. File tools also auto-add on first use, so this is mainly for pre-warming."),
			mcp.WithDestructiveHintAnnotation(false),
			mcp.WithString("project_root", mcp.Description("Container path to the project root (the directory containing Configuration.xml / src/cf / .git)."), mcp.Required()),
		), func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
			root, err := request.RequireString("project_root")
			if err != nil {
				return mcp.NewToolResultError(err.Error()), nil
			}
			b, err := asProjectBridge(bridge)
			if err != nil {
				return mcp.NewToolResultError(err.Error()), nil
			}
			if !b.MultiProjectEnabled() {
				return mcp.NewToolResultError("multi-project mode is not enabled (set MULTI_PROJECT=1 on the daemon)"), nil
			}
			res, err := b.ProjectAdd(root)
			if err != nil {
				logger.Error("project_add failed", err)
				return mcp.NewToolResultError(fmt.Sprintf("project_add failed: %v", err)), nil
			}
			return formatProjectResult(res)
		}
}

// ProjectCloseTool unregisters a project and frees its memory.
func ProjectCloseTool(bridge interfaces.BridgeInterface) (mcp.Tool, server.ToolHandlerFunc) {
	return mcp.NewTool("project_close",
			mcp.WithDescription("Multi-project mode: unregister a 1C project (workspace folder) and free its memory. Subsequent file calls would auto-add it again."),
			mcp.WithDestructiveHintAnnotation(true),
			mcp.WithString("project_root", mcp.Description("Container path to the project root to close."), mcp.Required()),
		), func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
			root, err := request.RequireString("project_root")
			if err != nil {
				return mcp.NewToolResultError(err.Error()), nil
			}
			b, err := asProjectBridge(bridge)
			if err != nil {
				return mcp.NewToolResultError(err.Error()), nil
			}
			if !b.MultiProjectEnabled() {
				return mcp.NewToolResultError("multi-project mode is not enabled (set MULTI_PROJECT=1 on the daemon)"), nil
			}
			res, err := b.ProjectClose(root)
			if err != nil {
				logger.Error("project_close failed", err)
				return mcp.NewToolResultError(fmt.Sprintf("project_close failed: %v", err)), nil
			}
			return formatProjectResult(res)
		}
}

// ProjectListTool lists all registered projects and their states.
func ProjectListTool(bridge interfaces.BridgeInterface) (mcp.Tool, server.ToolHandlerFunc) {
	return mcp.NewTool("project_list",
			mcp.WithDescription("Multi-project mode: list registered 1C projects with their state (indexing/ready) and which one is last-used."),
			mcp.WithDestructiveHintAnnotation(false),
		), func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
			b, err := asProjectBridge(bridge)
			if err != nil {
				return mcp.NewToolResultError(err.Error()), nil
			}
			if !b.MultiProjectEnabled() {
				return mcp.NewToolResultError("multi-project mode is not enabled (set MULTI_PROJECT=1 on the daemon)"), nil
			}
			res, err := b.ProjectList()
			if err != nil {
				logger.Error("project_list failed", err)
				return mcp.NewToolResultError(fmt.Sprintf("project_list failed: %v", err)), nil
			}
			return formatProjectResult(res)
		}
}

// RegisterProjectTools registers the multi-project control tools.
func RegisterProjectTools(mcpServer ToolServer, bridge interfaces.BridgeInterface) {
	mcpServer.AddTool(ProjectAddTool(bridge))
	mcpServer.AddTool(ProjectCloseTool(bridge))
	mcpServer.AddTool(ProjectListTool(bridge))
}
