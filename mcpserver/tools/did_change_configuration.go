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

func DidChangeConfigurationTool(bridge interfaces.BridgeInterface) (mcp.Tool, server.ToolHandlerFunc) {
	return mcp.NewTool("did_change_configuration",
			mcp.WithDescription("Notify the language server that configuration settings changed (workspace/didChangeConfiguration)."),
			mcp.WithDestructiveHintAnnotation(false),
			mcp.WithString("language", mcp.Description("Language server ID (e.g., 'bsl')."), mcp.Required()),
			mcp.WithString("settings_json", mcp.Description("JSON object with configuration settings to pass to the server.")),
		), func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
			language, err := request.RequireString("language")
			if err != nil {
				logger.Error("did_change_configuration: language parsing failed", err)
				return mcp.NewToolResultError(err.Error()), nil
			}

			if result, ok := CheckReadyOrReturn(bridge); !ok {
				return result, nil
			}

			settingsJSON := request.GetString("settings_json", "")
			var settings any
			if settingsJSON == "" {
				settings = map[string]any{}
			} else {
				if err := json.Unmarshal([]byte(settingsJSON), &settings); err != nil {
					return mcp.NewToolResultError(fmt.Sprintf("Invalid settings_json: %v", err)), nil
				}
			}

			if err := bridge.DidChangeConfiguration(language, settings); err != nil {
				logger.Error("did_change_configuration: notification failed", err)
				return mcp.NewToolResultError(fmt.Sprintf("didChangeConfiguration failed: %v", err)), nil
			}

			return mcp.NewToolResultText("ok"), nil
		}
}

func RegisterDidChangeConfigurationTool(mcpServer ToolServer, bridge interfaces.BridgeInterface) {
	mcpServer.AddTool(DidChangeConfigurationTool(bridge))
}
