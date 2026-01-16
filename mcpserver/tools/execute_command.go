package tools

import (
	"context"
	"encoding/json"
	"fmt"

	"rockerboo/mcp-lsp-bridge/interfaces"
	"rockerboo/mcp-lsp-bridge/logger"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
)

func ExecuteCommandTool(bridge interfaces.BridgeInterface) (mcp.Tool, server.ToolHandlerFunc) {
	return mcp.NewTool("execute_command",
			mcp.WithDescription("Execute workspace commands exposed by the language server (workspace/executeCommand). Useful for server-specific actions like refactors or code generation."),
			mcp.WithDestructiveHintAnnotation(false),
			mcp.WithString("command", mcp.Description("LSP command identifier (server-specific)."), mcp.Required()),
			mcp.WithString("arguments_json", mcp.Description("Optional JSON array of arguments for the command.")),
			mcp.WithString("language", mcp.Description("Language server ID (e.g., 'bsl'). Required if uri is not provided.")),
			mcp.WithString("uri", mcp.Description("Optional file URI to infer language when language is not provided.")),
		), func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
			command, err := request.RequireString("command")
			if err != nil {
				logger.Error("execute_command: command parsing failed", err)
				return mcp.NewToolResultError(err.Error()), nil
			}

			if result, ok := CheckReadyOrReturn(bridge); !ok {
				return result, nil
			}

			argsJSON := request.GetString("arguments_json", "")
			var args []any
			if argsJSON != "" {
				if err := json.Unmarshal([]byte(argsJSON), &args); err != nil {
					return mcp.NewToolResultError(fmt.Sprintf("Invalid arguments_json: %v", err)), nil
				}
			}

			language := request.GetString("language", "")
			if language == "" {
				uri := request.GetString("uri", "")
				if uri == "" {
					return mcp.NewToolResultError("language or uri is required"), nil
				}
				lang, err := bridge.InferLanguage(uri)
				if err != nil {
					return mcp.NewToolResultError(fmt.Sprintf("failed to infer language from uri: %v", err)), nil
				}
				language = string(*lang)
			}

			result, err := bridge.ExecuteCommand(language, command, args)
			if err != nil {
				logger.Error("execute_command: request failed", err)
				return mcp.NewToolResultError(fmt.Sprintf("execute command failed: %v", err)), nil
			}

			if len(result) == 0 {
				return mcp.NewToolResultText("null"), nil
			}

			return mcp.NewToolResultText(string(result)), nil
		}
}

func RegisterExecuteCommandTool(mcpServer ToolServer, bridge interfaces.BridgeInterface) {
	mcpServer.AddTool(ExecuteCommandTool(bridge))
}
