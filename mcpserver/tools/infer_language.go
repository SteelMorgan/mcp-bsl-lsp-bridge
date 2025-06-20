package tools

import (
	"context"
	"fmt"
	"path/filepath"

	"rockerboo/mcp-lsp-bridge/logger"
	"rockerboo/mcp-lsp-bridge/interfaces"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
)

// RegisterInferLanguageTool registers the infer_language tool
func RegisterInferLanguageTool(mcpServer *server.MCPServer, bridge interfaces.BridgeInterface) {
	mcpServer.AddTool(mcp.NewTool("infer_language",
		mcp.WithDescription("Infer the programming language for a file"),
		mcp.WithString("file_path", mcp.Description("Path to the file to infer language")),
	), func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		filePath, err := request.RequireString("file_path")
		if err != nil {
			logger.Error("infer_language: File path parsing failed", err)
			return mcp.NewToolResultError(err.Error()), nil
		}

		// Infer language from file extension
		config := bridge.GetConfig()
		if config == nil {
			logger.Error("infer_language: No configuration available")
			return mcp.NewToolResultError("No LSP configuration found"), nil
		}

		ext := filepath.Ext(filePath)
		language, found := config.ExtensionLanguageMap[ext]
		if !found {
			logger.Error("infer_language: Language inference failed",
				fmt.Sprintf("Extension: %s", ext),
			)
			return mcp.NewToolResultError(fmt.Sprintf("No language found for extension %s", ext)), nil
		}

		logger.Info("infer_language: Successfully inferred language",
			fmt.Sprintf("File: %s, Language: %s", filePath, language),
		)

		return mcp.NewToolResultText(language), nil
	})
}