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

func ColorPresentationTool(bridge interfaces.BridgeInterface) (mcp.Tool, server.ToolHandlerFunc) {
	return mcp.NewTool("color_presentation",
			mcp.WithDescription("Get color presentation suggestions (textDocument/colorPresentation)."),
			mcp.WithDestructiveHintAnnotation(false),
			mcp.WithString("uri", mcp.Description("URI to the file"), mcp.Required()),
			mcp.WithNumber("start_line", mcp.Description("Start line (0-based)"), mcp.Required(), mcp.Min(0)),
			mcp.WithNumber("start_character", mcp.Description("Start character (0-based)"), mcp.Required(), mcp.Min(0)),
			mcp.WithNumber("end_line", mcp.Description("End line (0-based)"), mcp.Required(), mcp.Min(0)),
			mcp.WithNumber("end_character", mcp.Description("End character (0-based)"), mcp.Required(), mcp.Min(0)),
			mcp.WithNumber("red", mcp.Description("Red component (0..1)"), mcp.Required()),
			mcp.WithNumber("green", mcp.Description("Green component (0..1)"), mcp.Required()),
			mcp.WithNumber("blue", mcp.Description("Blue component (0..1)"), mcp.Required()),
			mcp.WithNumber("alpha", mcp.Description("Alpha component (0..1)"), mcp.Required()),
		), func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
			uri, err := request.RequireString("uri")
			if err != nil {
				logger.Error("color_presentation: URI parsing failed", err)
				return mcp.NewToolResultError(err.Error()), nil
			}

			startLine, err := request.RequireInt("start_line")
			if err != nil {
				return mcp.NewToolResultError(fmt.Sprintf("Invalid start_line: %v", err)), nil
			}
			startCharacter, err := request.RequireInt("start_character")
			if err != nil {
				return mcp.NewToolResultError(fmt.Sprintf("Invalid start_character: %v", err)), nil
			}
			endLine, err := request.RequireInt("end_line")
			if err != nil {
				return mcp.NewToolResultError(fmt.Sprintf("Invalid end_line: %v", err)), nil
			}
			endCharacter, err := request.RequireInt("end_character")
			if err != nil {
				return mcp.NewToolResultError(fmt.Sprintf("Invalid end_character: %v", err)), nil
			}

			red, err := request.RequireFloat("red")
			if err != nil {
				return mcp.NewToolResultError(fmt.Sprintf("Invalid red: %v", err)), nil
			}
			green, err := request.RequireFloat("green")
			if err != nil {
				return mcp.NewToolResultError(fmt.Sprintf("Invalid green: %v", err)), nil
			}
			blue, err := request.RequireFloat("blue")
			if err != nil {
				return mcp.NewToolResultError(fmt.Sprintf("Invalid blue: %v", err)), nil
			}
			alpha, err := request.RequireFloat("alpha")
			if err != nil {
				return mcp.NewToolResultError(fmt.Sprintf("Invalid alpha: %v", err)), nil
			}

			if result, ok := CheckReadyOrReturn(bridge); !ok {
				return result, nil
			}

			startLineUint32, err := safeUint32(startLine)
			if err != nil {
				return mcp.NewToolResultError(fmt.Sprintf("Invalid start_line: %v", err)), nil
			}
			startCharacterUint32, err := safeUint32(startCharacter)
			if err != nil {
				return mcp.NewToolResultError(fmt.Sprintf("Invalid start_character: %v", err)), nil
			}
			endLineUint32, err := safeUint32(endLine)
			if err != nil {
				return mcp.NewToolResultError(fmt.Sprintf("Invalid end_line: %v", err)), nil
			}
			endCharacterUint32, err := safeUint32(endCharacter)
			if err != nil {
				return mcp.NewToolResultError(fmt.Sprintf("Invalid end_character: %v", err)), nil
			}

			rng := protocol.Range{
				Start: protocol.Position{Line: startLineUint32, Character: startCharacterUint32},
				End:   protocol.Position{Line: endLineUint32, Character: endCharacterUint32},
			}
			color := protocol.Color{
				Red:   red,
				Green: green,
				Blue:  blue,
				Alpha: alpha,
			}

			presentations, err := bridge.ColorPresentation(uri, color, rng)
			if err != nil {
				logger.Error("color_presentation: request failed", err)
				return mcp.NewToolResultError(fmt.Sprintf("Color presentation failed: %v", err)), nil
			}

			raw, err := json.Marshal(presentations)
			if err != nil {
				return mcp.NewToolResultError(fmt.Sprintf("Failed to serialize result: %v", err)), nil
			}

			return mcp.NewToolResultText(string(raw)), nil
		}
}

func RegisterColorPresentationTool(mcpServer ToolServer, bridge interfaces.BridgeInterface) {
	mcpServer.AddTool(ColorPresentationTool(bridge))
}
