package tools

import (
	"context"
	"fmt"
	"strings"

	"rockerboo/mcp-lsp-bridge/interfaces"
	"rockerboo/mcp-lsp-bridge/logger"
	"rockerboo/mcp-lsp-bridge/lsp"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
)

func SemanticTokensTool(bridge interfaces.BridgeInterface) (mcp.Tool, server.ToolHandlerFunc) {
	return mcp.NewTool("semantic_tokens",
			mcp.WithDescription("Get semantic tokens for a specific range of a file."),
			mcp.WithString("uri", mcp.Description("URI to the file")),
			mcp.WithNumber("start_line", mcp.Description("Start Line number (0-based)")),
			mcp.WithNumber("start_character", mcp.Description("Start Character position (0-based)")),
			mcp.WithNumber("end_line", mcp.Description("End Line number (0-based)")),
			mcp.WithNumber("end_character", mcp.Description("End Character position (0-based)")),
			mcp.WithString("type", mcp.Description("function, parameter, variable")),
		), func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
			// Parse and validate parameters
			uri, err := request.RequireString("uri")
			if err != nil {
				logger.Error("hover: URI parsing failed", err)
				return mcp.NewToolResultError(err.Error()), nil
			}

			startLine, err := request.RequireInt("start_line")
			if err != nil {
				logger.Error("semantic_tokens: Start Line parsing failed", err)
				return mcp.NewToolResultError(err.Error()), nil
			}

			startCharacter, err := request.RequireInt("start_character")
			if err != nil {
				logger.Error("semantic_tokens: Start Character parsing failed", err)
				return mcp.NewToolResultError(err.Error()), nil
			}

			endLine, err := request.RequireInt("end_line")
			if err != nil {
				logger.Error("semantic_tokens: End Line parsing failed", err)
				return mcp.NewToolResultError(err.Error()), nil
			}

			endCharacter, err := request.RequireInt("end_character")
			if err != nil {
				logger.Error("semantic_tokens: End Character parsing failed", err)
				return mcp.NewToolResultError(err.Error()), nil
			}

			targetTypes := []string{"function"}

			positions, err := bridge.SemanticTokens(uri, targetTypes, uint32(startLine), uint32(startCharacter), uint32(endLine), uint32(endCharacter))

			if err != nil {
				logger.Error("semantic_tokens: failed to get token positions", err)
				return mcp.NewToolResultError(err.Error()), nil
			}

			return mcp.NewToolResultText(formatTokensByType(positions)), nil
		}
}

func formatTokensByType(positions []lsp.TokenPosition) string {
	var response strings.Builder
	tokensByType := make(map[string][]lsp.TokenPosition)

	// Group tokens by type
	for _, pos := range positions {
		tokensByType[pos.TokenType] = append(tokensByType[pos.TokenType], pos)
	}

	// Format output
	for tokenType, tokens := range tokensByType {
		fmt.Fprintf(&response, "%s tokens:\n", tokenType)
		for _, token := range tokens {
			fmt.Fprintf(&response, "  - '%s' at %d:%d\n",
				token.Text,
				token.Range.Start.Line,
				token.Range.Start.Character,
			)
		}
		response.WriteString("\n")
	}

	return response.String()
}

// RegisterSemanticTokensTool registers the semantic token tool with the MCP server.
func RegisterSemanticTokensTool(mcpServer ToolServer, bridge interfaces.BridgeInterface) {
	mcpServer.AddTool(SemanticTokensTool(bridge))
}
