package tools

import (
	"context"
	"encoding/json"
	"fmt"

	"rockerboo/mcp-lsp-bridge/interfaces"
	"rockerboo/mcp-lsp-bridge/logger"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
)

func PrepareRenameTool(bridge interfaces.BridgeInterface) (mcp.Tool, server.ToolHandlerFunc) {
	return mcp.NewTool("prepare_rename",
			mcp.WithDescription("Check rename availability and obtain the rename range (textDocument/prepareRename)."),
			mcp.WithDestructiveHintAnnotation(false),
			mcp.WithString("uri", mcp.Description("URI to the file"), mcp.Required()),
			mcp.WithNumber("line", mcp.Description("Line number (0-based)"), mcp.Required(), mcp.Min(0)),
			mcp.WithNumber("character", mcp.Description("Character position (0-based)"), mcp.Required(), mcp.Min(0)),
		), func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
			uri, err := request.RequireString("uri")
			if err != nil {
				logger.Error("prepare_rename: URI parsing failed", err)
				return mcp.NewToolResultError(err.Error()), nil
			}
			line, err := request.RequireInt("line")
			if err != nil {
				return mcp.NewToolResultError(fmt.Sprintf("Invalid line: %v", err)), nil
			}
			character, err := request.RequireInt("character")
			if err != nil {
				return mcp.NewToolResultError(fmt.Sprintf("Invalid character: %v", err)), nil
			}

			if result, ok := CheckReadyOrReturn(bridge); !ok {
				return result, nil
			}

			lineUint32, err := safeUint32(line)
			if err != nil {
				return mcp.NewToolResultError(fmt.Sprintf("Invalid line: %v", err)), nil
			}
			characterUint32, err := safeUint32(character)
			if err != nil {
				return mcp.NewToolResultError(fmt.Sprintf("Invalid character: %v", err)), nil
			}

			result, err := bridge.PrepareRename(uri, lineUint32, characterUint32)
			if err != nil {
				logger.Error("prepare_rename: request failed", err)
				return mcp.NewToolResultError(fmt.Sprintf("Prepare rename failed: %v", err)), nil
			}

			raw, err := json.Marshal(result)
			if err != nil {
				return mcp.NewToolResultError(fmt.Sprintf("Failed to serialize result: %v", err)), nil
			}

			return mcp.NewToolResultText(string(raw)), nil
		}
}

func RegisterPrepareRenameTool(mcpServer ToolServer, bridge interfaces.BridgeInterface) {
	mcpServer.AddTool(PrepareRenameTool(bridge))
}
