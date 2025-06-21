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

// RegisterSignatureHelpTool registers the signature help tool
func RegisterSignatureHelpTool(mcpServer *server.MCPServer, bridge interfaces.BridgeInterface) {
	mcpServer.AddTool(mcp.NewTool("signature_help",
		mcp.WithDescription("Get function parameter information at call sites. Use when positioned inside function calls (between parentheses) to see parameter details and overloads."),
		mcp.WithString("uri", mcp.Description("URI to the file")),
		mcp.WithNumber("line", mcp.Description("Line number (0-based) - position at function call site")),
		mcp.WithNumber("character", mcp.Description("Character position (0-based) - position within function call parentheses")),
	), func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
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

		// Execute bridge method
		result, err := bridge.GetSignatureHelp(uri, int32(line), int32(character))
		if err != nil {
			logger.Error("signature_help: Request failed", err)
			return mcp.NewToolResultError("Failed to get signature help"), nil
		}

		// Format and return result
		if result == nil {
			return mcp.NewToolResultText("No signature help available"), nil
		}

		// Extract signature help from the response
		if sigHelp, ok := result.(protocol.SignatureHelpResponse); ok {
			content := formatSignatureHelp(sigHelp)
			return mcp.NewToolResultText(content), nil
		}

		return mcp.NewToolResultText(fmt.Sprintf("Signature help result: %v", result)), nil
	})
}