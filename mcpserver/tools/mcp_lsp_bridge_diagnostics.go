package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strings"

	"rockerboo/mcp-lsp-bridge/interfaces"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
)

func MCPLSPDiagnosticsTool(bridge interfaces.BridgeInterface) (mcp.Tool, server.ToolHandlerFunc) {
	return mcp.NewTool("mcp_lsp_diagnostics",
			mcp.WithDescription("Provides diagnostic information about the MCP-LSP bridge, including registered language servers, configuration details, connected servers, and detected project languages."),
			mcp.WithDestructiveHintAnnotation(false),
			mcp.WithString("report_type", mcp.Description("Type of diagnostic report to generate: 'summary', 'config', 'connected_clients', 'project_languages', or 'all'. Default: 'summary'")),
			mcp.WithString("summary", mcp.Description("config, connected_clients, project_languages, all")),
			mcp.WithString("project_path", mcp.Description("Optional: Path to the project directory for project_languages report. Defaults to current working directory.")),
		), func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
			// Parse and validate parameters
			reportType := request.GetString("report_type", "summary")

			wd, err := os.Getwd()
			if err != nil {
				return mcp.NewToolResultText("Error getting current working directory: " + err.Error()), nil
			}

			projectPath := request.GetString("project_path", wd)

			var sb strings.Builder
			sb.WriteString("MCP-LSP Bridge Diagnostics Report\n")
			sb.WriteString("---------------------------------\n")

			config := bridge.GetConfig()
			if config == nil {
				sb.WriteString("Error: LSP configuration is not loaded.\n")
				return mcp.NewToolResultText(sb.String()), nil
			}

			globalConfig := config.GetGlobalConfig()
			languageServers := config.GetLanguageServers()

			if reportType == "summary" || reportType == "all" {
				sb.WriteString("\n### Summary\n")
				sb.WriteString(fmt.Sprintf("Global Log Level: %s\n", globalConfig.LogLevel))
				sb.WriteString(fmt.Sprintf("Global Log Path: %s\n", globalConfig.LogPath))
				sb.WriteString(fmt.Sprintf("Max Restart Attempts: %d\n", globalConfig.MaxRestartAttempts))
				sb.WriteString(fmt.Sprintf("Restart Delay: %dms\n", globalConfig.RestartDelayMs))
				sb.WriteString(fmt.Sprintf("Number of Registered Language Servers: %d\n", len(languageServers)))
			}

			if reportType == "config" || reportType == "all" {
				sb.WriteString("\n### Language Server Configuration\n")
				if len(languageServers) == 0 {
					sb.WriteString("No language servers configured.\n")
				} else {
					for lang, lsConfig := range languageServers {
						initializationOptions, err := json.MarshalIndent(lsConfig.GetInitializationOptions(), "", "  ")
						if err != nil {
							sb.WriteString(fmt.Sprintf("Error unmarshaling initialization options for %s: %v\n", lang, err))
						}

						sb.WriteString(fmt.Sprintf("  Language: %s\n", lang))
						sb.WriteString(fmt.Sprintf("    Command: %s %s\n", lsConfig.GetCommand(), strings.Join(lsConfig.GetArgs(), " ")))
						sb.WriteString(fmt.Sprintf("    Filetypes: %s\n", "N/A - available in concrete type"))
						sb.WriteString(fmt.Sprintf("    Initialization Options: %s\n", string(initializationOptions)))
					}
				}
			}

			if reportType == "connected_clients" || reportType == "all" {
				sb.WriteString("\n### Connected Language Clients\n")
				var configuredLanguages []string
				for lang := range languageServers {
					configuredLanguages = append(configuredLanguages, string(lang))
				}

				connectedClients, err := bridge.GetMultiLanguageClients(configuredLanguages)
				if err != nil {
					sb.WriteString(fmt.Sprintf("Error retrieving connected clients: %v\n", err))
				} else if len(connectedClients) == 0 {
					sb.WriteString("No language clients currently connected.\n")
				} else {
					sb.WriteString("Currently connected clients:\n")
					for lang, client := range connectedClients {
						metrics := client.GetMetrics()
						sb.WriteString(fmt.Sprintf("  Language: %s\n", lang))
						sb.WriteString(fmt.Sprintf("    Status: %d\n", metrics.GetStatus()))
						sb.WriteString(fmt.Sprintf("    Last Error: %v\n", metrics.GetLastError()))
						sb.WriteString(fmt.Sprintf("    Connection Attempts: %d\n", metrics.GetTotalRequests()))
						sb.WriteString(fmt.Sprintf("    Last Connected At: %s\n", metrics.GetLastInitialized()))
					}
				}
			}

			if reportType == "project_languages" || reportType == "all" {
				sb.WriteString("\n### Detected Project Languages\n")
				if projectPath != "" {
					languages, err := bridge.DetectProjectLanguages(projectPath)
					if err != nil {
						sb.WriteString(fmt.Sprintf("Error detecting project languages for '%s': %v\n", projectPath, err))
					} else if len(languages) == 0 {
						sb.WriteString(fmt.Sprintf("No languages detected for project '%s'.\n", projectPath))
					} else {
						sb.WriteString(fmt.Sprintf("Detected languages for '%s':\n", projectPath))
						for _, lang := range languages {
							sb.WriteString(fmt.Sprintf("  - %s\n", lang))
						}
					}
				} else {
					sb.WriteString("Project path not provided. Cannot detect project languages.\n")
				}
			}

			return mcp.NewToolResultText(sb.String()), nil
		}
}

// Register the tool with the MCP server.
func RegisterMCPLSPBridgeDiagnosticsTool(mcpServer ToolServer, bridge interfaces.BridgeInterface) {
	mcpServer.AddTool(MCPLSPDiagnosticsTool(bridge))
}
