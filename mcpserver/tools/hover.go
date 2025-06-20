package tools

import (
	"context"
	"fmt"

	"rockerboo/mcp-lsp-bridge/logger"
	"rockerboo/mcp-lsp-bridge/interfaces"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
	"github.com/myleshyson/lsprotocol-go/protocol"
)

// RegisterHoverTool registers the hover tool
func RegisterHoverTool(mcpServer *server.MCPServer, bridge interfaces.BridgeInterface) {
	mcpServer.AddTool(mcp.NewTool("hover",
		mcp.WithDescription("Get hover information for symbol at position"),
		mcp.WithString("uri", mcp.Description("URI to the file")),
		mcp.WithNumber("line", mcp.Description("Line number (0-based)")),
		mcp.WithNumber("character", mcp.Description("Character position (0-based)")),
	), func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		// Extensive debug logging
		logger.Info("Hover Tool: Starting hover information request")

		// Parse and validate parameters
		uri, err := request.RequireString("uri")
		if err != nil {
			logger.Error("hover: URI parsing failed", err)
			return mcp.NewToolResultError(err.Error()), nil
		}
		logger.Info(fmt.Sprintf("Hover Tool: URI parsed: %s", uri))

		line, err := request.RequireInt("line")
		if err != nil {
			logger.Error("hover: Line parsing failed", err)
			return mcp.NewToolResultError(err.Error()), nil
		}
		logger.Info(fmt.Sprintf("Hover Tool: Line parsed: %d", line))

		character, err := request.RequireInt("character")
		if err != nil {
			logger.Error("hover: Character parsing failed", err)
			return mcp.NewToolResultError(err.Error()), nil
		}
		logger.Info(fmt.Sprintf("Hover Tool: Character parsed: %d", character))

		// Infer language for debugging
		language, langErr := bridge.InferLanguage(uri)
		if langErr != nil {
			logger.Error("hover: Language inference failed", langErr)
		} else {
			logger.Info(fmt.Sprintf("Hover Tool: Inferred language: %s", language))
		}

		// Execute bridge method with detailed error logging
		result, err := bridge.GetHoverInformation(uri, int32(line), int32(character))
		if err != nil {
			logger.Error("hover: Request failed", fmt.Sprintf("URI: %s, Line: %d, Character: %d, Error: %v", uri, line, character, err))
			return mcp.NewToolResultError(fmt.Sprintf("Failed to get hover information: %v", err)), nil
		}

		// Detailed logging of result
		if result == nil {
			logger.Info("Hover Tool: No hover information available")
			return mcp.NewToolResultText("No hover information available"), nil
		}

		// Enhanced result type logging
		logger.Info(fmt.Sprintf("Hover Tool: Result type: %T", result))

		// Extract contents from the hover response with multiple type checks
		var content string
		switch v := result.(type) {
		case protocol.HoverResponse:
			// Check for result explicitly
			if v.Result == nil {
				content = "=== HOVER INFORMATION ===\nNo hover result available"
			} else {
				content = formatHoverContent(v.Result.Contents)
			}
		case *protocol.Hover:
			// Direct Hover type
			content = formatHoverContent(v.Contents)
		case map[string]any:
			// Fallback for generic map responses
			contents, hasContents := v["contents"]
			if hasContents {
				content = formatHoverContent(contents)
			} else {
				content = fmt.Sprintf("=== HOVER INFORMATION ===\nMap result: %+v", v)
			}
		default:
			// Fallback for any other type
			content = formatHoverContent(result)
		}

		return mcp.NewToolResultText(content), nil
	})
}