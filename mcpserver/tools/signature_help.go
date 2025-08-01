package tools

import (
	"context"
	"fmt"

	"rockerboo/mcp-lsp-bridge/interfaces"
	"rockerboo/mcp-lsp-bridge/logger"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
)

// RegisterSignatureHelpTool registers the signature help tool
func RegisterSignatureHelpTool(mcpServer ToolServer, bridge interfaces.BridgeInterface) {
	mcpServer.AddTool(SignatureHelpTool(bridge))
}

func SignatureHelpTool(bridge interfaces.BridgeInterface) (mcp.Tool, server.ToolHandlerFunc) {
	return mcp.NewTool("signature_help",
			mcp.WithDescription("Get function parameter information at call sites. ESSENTIAL when writing function calls - provides real-time parameter hints and overload information that prevents syntax errors."),
			mcp.WithDestructiveHintAnnotation(false),
			mcp.WithString("uri", mcp.Description("URI to the file")),
			mcp.WithNumber("line", mcp.Description("Line number (0-based) - position at function call site")),
			mcp.WithNumber("character", mcp.Description("Character position (0-based) - position within function call parentheses")),
		),
		func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
			// Parse and validate parameters
			uri, err := request.RequireString("uri")
			if err != nil {
				logger.Error("signature_help: URI parsing failed", err)
				return mcp.NewToolResultError(err.Error()), nil
			}

			line, err := request.RequireInt("line")
			if err != nil {
				logger.Error("signature_help: Line parsing failed", err)
				return mcp.NewToolResultError(err.Error()), nil
			}

			character, err := request.RequireInt("character")
			if err != nil {
				logger.Error("signature_help: Character parsing failed", err)
				return mcp.NewToolResultError(err.Error()), nil
			}

			// Safe conversions for line and character
			lineUint32, err := safeUint32(line)
			if err != nil {
				return mcp.NewToolResultError(fmt.Sprintf("Invalid line number: %v", err)), nil
			}
			characterUint32, err := safeUint32(character)
			if err != nil {
				return mcp.NewToolResultError(fmt.Sprintf("Invalid character position: %v", err)), nil
			}

			// Execute bridge method
			result, err := bridge.GetSignatureHelp(uri, lineUint32, characterUint32)
			if err != nil {
				logger.Error("signature_help: Request failed", err)
				return mcp.NewToolResultError(fmt.Sprintf("Failed to get signature help: %+v", err)), nil
			}

			// Format and return result
			if result == nil {
				return mcp.NewToolResultText("No signature help available"), nil
			}

			return mcp.NewToolResultText(fmt.Sprintf("Signature help result: %v", result)), nil
		}
}
