package tools

import (
	"context"
	"fmt"

	"rockerboo/mcp-lsp-bridge/interfaces"
	"rockerboo/mcp-lsp-bridge/logger"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
)

func HoverTool(bridge interfaces.BridgeInterface) (mcp.Tool, server.ToolHandlerFunc) {
	return mcp.NewTool("hover",
			mcp.WithDescription("Get detailed symbol information (signatures, documentation, types)."),
			mcp.WithString("uri", mcp.Description("URI to the file"), mcp.Required()),
			mcp.WithNumber("line", mcp.Description("Line number (0-based)"), mcp.Required(), mcp.Min(0)),
			mcp.WithNumber("character", mcp.Description("Character position (0-based)"), mcp.Required(), mcp.Min(0)),
		), func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
			// Parse and validate parameters
			uri, err := request.RequireString("uri")
			if err != nil {
				logger.Error("hover: URI parsing failed", err)
				return mcp.NewToolResultError(err.Error()), nil
			}

			line, err := request.RequireInt("line")
			if err != nil {
				logger.Error("hover: Line parsing failed", err)
				return mcp.NewToolResultError(err.Error()), nil
			}

			character, err := request.RequireInt("character")
			if err != nil {
				logger.Error("hover: Character parsing failed", err)
				return mcp.NewToolResultError(err.Error()), nil
			}

			// Infer language for debugging
			language, langErr := bridge.InferLanguage(uri)
			if langErr != nil {
				logger.Error("hover: Language inference failed", langErr)
			} else {
				logger.Info(fmt.Sprintf("Hover Tool: Inferred language: %s", *language))
			}

			// Execute bridge method with detailed error logging
			lineUint32, err := safeUint32(line)
			if err != nil {
				return mcp.NewToolResultError(fmt.Sprintf("Invalid line number: %v", err)), nil
			}
			characterUint32, err := safeUint32(character)
			if err != nil {
				return mcp.NewToolResultError(fmt.Sprintf("Invalid character position: %v", err)), nil
			}
			
			result, err := bridge.GetHoverInformation(uri, lineUint32, characterUint32)
			if err != nil {
				logger.Error("hover: Request failed", fmt.Sprintf("URI: %s, Line: %d, Character: %d, Error: %v", uri, line, character, err))
				return mcp.NewToolResultError(fmt.Sprintf("Failed to get hover information: %v", err)), nil
			}

			// Detailed logging of result
			if result == nil {
				logger.Info("Hover Tool: No hover information available")
				return mcp.NewToolResultText("No hover information available"), nil
			}

			content := formatHoverContent(result.Contents)

			return mcp.NewToolResultText(content), nil
		}
}

// RegisterHoverTool registers the hover tool with the MCP server.
func RegisterHoverTool(mcpServer ToolServer, bridge interfaces.BridgeInterface) {
	mcpServer.AddTool(HoverTool(bridge))
}
