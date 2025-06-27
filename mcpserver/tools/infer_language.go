package tools

import (
	"context"
	"fmt"
	"path/filepath"

	"rockerboo/mcp-lsp-bridge/interfaces"
	"rockerboo/mcp-lsp-bridge/logger"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
)

// RegisterInferLanguageTool registers the infer_language tool
func RegisterInferLanguageTool(mcpServer ToolServer, bridge interfaces.BridgeInterface) {
	mcpServer.AddTool(InferLanguageTool(bridge))
}

func InferLanguageTool(bridge interfaces.BridgeInterface) (mcp.Tool, server.ToolHandlerFunc) {
	return mcp.NewTool("infer_language",
			mcp.WithDescription("Infer the programming language for a file"),
			mcp.WithString("file_path", mcp.Description("Path to the file to infer language")),
		), func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
			filePath, err := request.RequireString("file_path")
			if err != nil {
				logger.Error("infer_language: File path parsing failed", err)
				return mcp.NewToolResultError(err.Error()), nil
			}

			ext := filepath.Ext(filePath)
			language, err := bridge.InferLanguage(filePath)

			if err != nil {
				return mcp.NewToolResultError("No language found for extension " + ext), nil
			}

			logger.Info("infer_language: Successfully inferred language",
				fmt.Sprintf("File: %s, Language: %s", filePath, language),
			)

			return mcp.NewToolResultText(string(language)), nil
		}
}
