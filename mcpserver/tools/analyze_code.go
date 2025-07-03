package tools

import (
	"context"
	"fmt"
	"reflect"

	"rockerboo/mcp-lsp-bridge/interfaces"
	"rockerboo/mcp-lsp-bridge/logger"
	"rockerboo/mcp-lsp-bridge/lsp"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
)

// RegisterAnalyzeCodeTool registers the analyze_code tool
func RegisterAnalyzeCodeTool(mcpServer ToolServer, bridge interfaces.BridgeInterface) {
	mcpServer.AddTool(AnalyzeCode(bridge))
}

func AnalyzeCode(bridge interfaces.BridgeInterface) (mcp.Tool, server.ToolHandlerFunc) {
	return mcp.NewTool("analyze_code",
			mcp.WithDescription("Analyze code for completion suggestions and insights"),
			mcp.WithString("uri", mcp.Description("URI to the file location to analyze")),
			mcp.WithNumber("line", mcp.Description("Line of the file to analyze")),
			mcp.WithNumber("character", mcp.Description("Character of the line to analyze")),
		), func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
			uri, err := request.RequireString("uri")
			if err != nil {
				logger.Error("analyze_code: URI parsing failed", err)
				return mcp.NewToolResultError(err.Error()), nil
			}

			line, err := request.RequireInt("line")
			if err != nil {
				logger.Error("analyze_code: Line parsing failed", err)
				return mcp.NewToolResultError(err.Error()), nil
			}

			character, err := request.RequireInt("character")
			if err != nil {
				logger.Error("analyze_code: Character parsing failed", err)
				return mcp.NewToolResultError(err.Error()), nil
			}

			// Infer language from the file URI
			language, err := bridge.InferLanguage(uri)
			if err != nil {
				logger.Error("analyze_code: Language inference failed", err)
				return mcp.NewToolResultError("Could not infer language"), nil
			}

			// Get LSP client for the language
			client, err := bridge.GetClientForLanguage(string(*language))
			if err != nil {
				logger.Error("analyze_code: Failed to get LSP client", err)
				return mcp.NewToolResultError("Failed to get LSP client"), nil
			}

			if client == nil {
				logger.Error("analyze_code: Failed to get LSP client", err)
				return mcp.NewToolResultError("Failed to get LSP client"), nil
			}

			// Convert URI and cast client
			lspClient, ok := client.(*lsp.LanguageClient)
			if !ok {
				logger.Error("analyze_code: Invalid LSP client type")
				return mcp.NewToolResultError("Invalid LSP client type"), nil
			}

			// Perform code analysis
			lineInt32, err := safeInt32(line)
			if err != nil {
				return mcp.NewToolResultError(fmt.Sprintf("Invalid line number: %v", err)), nil
			}
			characterInt32, err := safeInt32(character)
			if err != nil {
				return mcp.NewToolResultError(fmt.Sprintf("Invalid character position: %v", err)), nil
			}
			
			analyzeOpts := lsp.AnalyzeCodeOptions{
				Uri:        uri,
				Line:       lineInt32,
				Character:  characterInt32,
				LanguageId: string(*language),
			}

			result, err := lsp.AnalyzeCode(lspClient, analyzeOpts)
			if err != nil {
				logger.Error("analyze_code: Code analysis failed", err)
				return mcp.NewToolResultError("Code analysis failed"), nil
			}

			// Log analysis details
			logger.Info("analyze_code: Successfully analyzed code",
				fmt.Sprintf("URI: %s, Line: %d, Character: %d", uri, line, character),
			)

			// Count completion suggestions by checking the length of the CompletionResponse
			completionCount := 0

			if result.Completion != nil {
				// Use reflection to handle different CompletionResponse types
				completionValue := reflect.ValueOf(result.Completion)
				if completionValue.Kind() == reflect.Ptr {
					completionValue = completionValue.Elem()
				}

				// Try to get the items or suggestions
				itemsField := completionValue.FieldByName("Items")
				if itemsField.IsValid() {
					completionCount = int(itemsField.Len())
				}
			}

			// Prepare result summary
			summary := fmt.Sprintf(
				"Analysis Results:\n"+
					"Hover: %v\n"+
					"Completion Suggestions: %d\n"+
					"Signature Help: %v\n"+
					"Diagnostics: %d\n"+
					"Code Actions: %d",
				result.Hover != nil,
				completionCount,
				result.SignatureHelp != nil,
				len(result.Diagnostics),
				len(result.CodeActions),
			)

			return mcp.NewToolResultText(summary), nil
		}
}
