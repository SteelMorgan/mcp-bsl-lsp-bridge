package tools

import (
	"context"
	"encoding/json"
	"fmt"

	"rockerboo/mcp-lsp-bridge/interfaces"
	"rockerboo/mcp-lsp-bridge/logger"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
	"github.com/myleshyson/lsprotocol-go/protocol"
)

func SelectionRangeTool(bridge interfaces.BridgeInterface) (mcp.Tool, server.ToolHandlerFunc) {
	return mcp.NewTool("selection_range",
			mcp.WithDescription("Get selection ranges for positions (textDocument/selectionRange). Provide positions_json or a single line/character."),
			mcp.WithDestructiveHintAnnotation(false),
			mcp.WithString("uri", mcp.Description("URI to the file"), mcp.Required()),
			mcp.WithString("positions_json", mcp.Description("Optional JSON array of positions: [{\"line\":10,\"character\":5}]")),
			mcp.WithNumber("line", mcp.Description("Line number (0-based) if positions_json not provided"), mcp.Min(0)),
			mcp.WithNumber("character", mcp.Description("Character position (0-based) if positions_json not provided"), mcp.Min(0)),
		), func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
			uri, err := request.RequireString("uri")
			if err != nil {
				logger.Error("selection_range: URI parsing failed", err)
				return mcp.NewToolResultError(err.Error()), nil
			}

			if result, ok := CheckReadyOrReturn(bridge); !ok {
				return result, nil
			}

			positionsJSON := request.GetString("positions_json", "")
			var positions []protocol.Position
			if positionsJSON != "" {
				if err := json.Unmarshal([]byte(positionsJSON), &positions); err != nil {
					return mcp.NewToolResultError(fmt.Sprintf("Invalid positions_json: %v", err)), nil
				}
			} else {
				line, err := request.RequireInt("line")
				if err != nil {
					return mcp.NewToolResultError(fmt.Sprintf("line is required if positions_json is empty: %v", err)), nil
				}
				character, err := request.RequireInt("character")
				if err != nil {
					return mcp.NewToolResultError(fmt.Sprintf("character is required if positions_json is empty: %v", err)), nil
				}
				lineUint32, err := safeUint32(line)
				if err != nil {
					return mcp.NewToolResultError(fmt.Sprintf("Invalid line: %v", err)), nil
				}
				characterUint32, err := safeUint32(character)
				if err != nil {
					return mcp.NewToolResultError(fmt.Sprintf("Invalid character: %v", err)), nil
				}
				positions = []protocol.Position{{Line: lineUint32, Character: characterUint32}}
			}

			ranges, err := bridge.SelectionRange(uri, positions)
			if err != nil {
				logger.Error("selection_range: request failed", err)
				return mcp.NewToolResultError(fmt.Sprintf("Selection range failed: %v", err)), nil
			}

			raw, err := json.Marshal(ranges)
			if err != nil {
				return mcp.NewToolResultError(fmt.Sprintf("Failed to serialize result: %v", err)), nil
			}

			return mcp.NewToolResultText(string(raw)), nil
		}
}

func RegisterSelectionRangeTool(mcpServer ToolServer, bridge interfaces.BridgeInterface) {
	mcpServer.AddTool(SelectionRangeTool(bridge))
}
