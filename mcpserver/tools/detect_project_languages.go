package tools

import (
	"context"
	"fmt"

	"rockerboo/mcp-lsp-bridge/interfaces"
	"rockerboo/mcp-lsp-bridge/logger"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
)

// RegisterProjectLanguageDetectionTool registers the detect_project_languages tool
func RegisterProjectLanguageDetectionTool(mcpServer ToolServer, bridge interfaces.BridgeInterface) {
	mcpServer.AddTool(ProjectLanguageDetectionTool(bridge))
}

func ProjectLanguageDetectionTool(bridge interfaces.BridgeInterface) (mcp.Tool, server.ToolHandlerFunc) {
	return mcp.NewTool("detect_project_languages",
			mcp.WithDescription("Detect all programming languages used in a project by examining root markers and file extensions. ESSENTIAL first step for any project analysis - automatically identifies languages and configures appropriate tooling for optimal LSP support."),
			mcp.WithDestructiveHintAnnotation(false),
			mcp.WithString("project_path", mcp.Description("Path to the project directory to analyze")),
			mcp.WithString("mode", mcp.Description("Detection mode: 'all' for all languages, 'primary' for primary language only (default: 'all')")),
		), func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
			projectPath, err := request.RequireString("project_path")
			if err != nil {
				logger.Error("detect_project_languages: Project path parsing failed", err)
				return mcp.NewToolResultError(err.Error()), nil
			}

			mode, err := request.RequireString("mode")
			if err != nil {
				// Default to "all" if mode is not specified
				mode = "all"
			}

			logger.Info("detect_project_languages: Starting language detection",
				fmt.Sprintf("Path: %s, Mode: %s", projectPath, mode),
			)

			switch mode {
			case "primary":
				primaryLanguage, err := bridge.DetectPrimaryProjectLanguage(projectPath)
				if err != nil {
					logger.Error("detect_project_languages: Primary language detection failed", err)
					return mcp.NewToolResultError(fmt.Sprintf("Failed to detect primary language: %v", err)), nil
				}

				logger.Info("detect_project_languages: Primary language detected",
					"Language: "+*primaryLanguage,
				)

				return mcp.NewToolResultText("Primary language: " + string(*primaryLanguage)), nil

			case "all":
				fallthrough
			default:
				languages, err := bridge.DetectProjectLanguages(projectPath)
				if err != nil {
					logger.Error("detect_project_languages: Language detection failed", err)
					return mcp.NewToolResultError(fmt.Sprintf("Failed to detect languages: %v", err)), nil
				}

				if len(languages) == 0 {
					return mcp.NewToolResultText("No programming languages detected in project"), nil
				}

				logger.Info("detect_project_languages: Languages detected",
					fmt.Sprintf("Count: %d, Languages: %v", len(languages), languages),
				)

				// Format the result
				result := "Detected languages (in priority order):\n"

				for i, lang := range languages {
					priority := "Primary"
					if i > 0 {
						priority = "Secondary"
					}

					result += fmt.Sprintf("%d. %s (%s)\n", i+1, lang, priority)
				}

				return mcp.NewToolResultText(result), nil
			}
		}
}
