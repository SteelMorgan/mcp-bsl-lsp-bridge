package tools

import (
	"context"
	"fmt"
	"path/filepath"
	"strings"

	"rockerboo/mcp-lsp-bridge/async"
	"rockerboo/mcp-lsp-bridge/collections"
	"rockerboo/mcp-lsp-bridge/interfaces"
	"rockerboo/mcp-lsp-bridge/logger"
	"rockerboo/mcp-lsp-bridge/types"
	"rockerboo/mcp-lsp-bridge/utils"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
	"github.com/myleshyson/lsprotocol-go/protocol"
)

// RegisterImplementationTool registers the implementation tool
func RegisterImplementationTool(mcpServer ToolServer, bridge interfaces.BridgeInterface) {
	mcpServer.AddTool(ImplementationTool(bridge))
}

func ImplementationTool(bridge interfaces.BridgeInterface) (mcp.Tool, server.ToolHandlerFunc) {
	return mcp.NewTool("implementation",
			mcp.WithDescription("Find implementations of a symbol (interfaces, abstract methods). EXCELLENT for understanding inheritance hierarchies - reveals concrete implementations that manual searching often misses."),
			mcp.WithDestructiveHintAnnotation(false),
			mcp.WithString("uri", mcp.Description("URI to the file")),
			mcp.WithNumber("line", mcp.Description("Line number (0-based)")),
			mcp.WithNumber("character", mcp.Description("Character position (0-based)")),
		), func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
			// Parse and validate parameters
			uri, err := request.RequireString("uri")
			if err != nil {
				logger.Error("implementation: URI parsing failed", err)
				return mcp.NewToolResultError(err.Error()), nil
			}

			line, err := request.RequireInt("line")
			if err != nil {
				logger.Error("implementation: Line parsing failed", err)
				return mcp.NewToolResultError(err.Error()), nil
			}

			character, err := request.RequireInt("character")
			if err != nil {
				logger.Error("implementation: Character parsing failed", err)
				return mcp.NewToolResultError(err.Error()), nil
			}

			// Validate parameters
			lineUint32, err := safeUint32(line)
			if err != nil {
				return mcp.NewToolResultError(fmt.Sprintf("Invalid line number: %v", err)), nil
			}
			characterUint32, err := safeUint32(character)
			if err != nil {
				return mcp.NewToolResultError(fmt.Sprintf("Invalid character position: %v", err)), nil
			}

			// Normalize URI to ensure proper file:// scheme
			normalizedURI := utils.NormalizeURI(uri)

			// For implementations, we want to search across multiple languages
			// since interfaces can be implemented in different languages
			dirs := bridge.AllowedDirectories()
			projectPath := strings.TrimPrefix(dirs[0], "file://")

			// Detect project languages
			languages, err := bridge.DetectProjectLanguages(projectPath)
			if err != nil {
				logger.Error("implementation: Project language detection failed", err)
				return mcp.NewToolResultError(fmt.Sprintf("Failed to detect project languages: %v", err)), nil
			}

			if len(languages) == 0 {
				return mcp.NewToolResultError("No languages detected in project"), nil
			}

			// Get clients for multiple languages
			languageStrings := collections.ToString(languages)
			clients, err := bridge.GetMultiLanguageClients(languageStrings)
			if err != nil || len(clients) == 0 {
				return mcp.NewToolResultError("No LSP clients available for detected languages"), nil
			}

			// Convert clients to async operations
			ops := collections.TransformMap(clients, func(client types.LanguageClientInterface) func() ([]protocol.Location, error) {
				return func() ([]protocol.Location, error) {
					return client.Implementation(normalizedURI, lineUint32, characterUint32)
				}
			})

			// Execute implementation search across all clients in parallel
			results, err := async.MapWithKeys(ctx, ops)
			if err != nil {
				logger.Error("implementation: async execution failed", err)
				return mcp.NewToolResultError(fmt.Sprintf("Failed to execute implementation search: %v", err)), nil
			}

			// Flatten results and collect errors
			flattened := utils.FlattenKeyedResults(results)
			allImplementations := flattened.Values

			// Log any errors from individual clients
			for _, err := range flattened.Errors {
				logger.Warn(fmt.Sprintf("Implementation search failed: %v", err))
			}

			// Format and return result
			content := formatMultiLanguageImplementations(allImplementations, flattened.Errors, normalizedURI, line, character, languages)

			return mcp.NewToolResultText(content), nil
		}
}

// formatMultiLanguageImplementations formats implementation results across multiple languages
func formatMultiLanguageImplementations(implementations []protocol.Location, errors []error, uri string, line, character int, languages []types.Language) string {
	var result strings.Builder

	// Header with summary
	fmt.Fprintf(&result, "IMPLEMENTATIONS:\n")
	fmt.Fprintf(&result, "Position: %s:%d:%d\n", uri, line, character)
	fmt.Fprintf(&result, "Languages searched: %v\n", languages)
	fmt.Fprintf(&result, "Implementations found: %d\n", len(implementations))
	if len(errors) > 0 {
		fmt.Fprintf(&result, "Errors: %d\n", len(errors))
	}
	fmt.Fprintf(&result, "\n")

	// Show errors if any
	if len(errors) > 0 {
		fmt.Fprintf(&result, "ERRORS:\n")
		for i, err := range errors {
			fmt.Fprintf(&result, "%d. %v\n", i+1, err)
		}
		fmt.Fprintf(&result, "\n")
	}

	// Show implementations
	if len(implementations) == 0 {
		fmt.Fprintf(&result, "No implementations found.\n")
		if len(errors) == 0 {
			fmt.Fprintf(&result, "This may indicate:\n")
			fmt.Fprintf(&result, "- The symbol is not an interface or abstract method\n")
			fmt.Fprintf(&result, "- No implementations exist in the current workspace\n")
			fmt.Fprintf(&result, "- The position does not correspond to a valid symbol\n")
		}
	} else {
		fmt.Fprintf(&result, "IMPLEMENTATIONS:\n")
		for i, impl := range implementations {
			uri := string(impl.Uri)
			filename := filepath.Base(strings.TrimPrefix(uri, "file://"))

			fmt.Fprintf(&result, "%d. %s\n", i+1, filename)
			fmt.Fprintf(&result, "   URI: %s\n", uri)
			fmt.Fprintf(&result, "   Range: %d:%d-%d:%d\n",
				impl.Range.Start.Line, impl.Range.Start.Character,
				impl.Range.End.Line, impl.Range.End.Character)
			fmt.Fprintf(&result, "   Position: line=%d, character=%d\n",
				impl.Range.Start.Line, impl.Range.Start.Character)
			fmt.Fprintf(&result, "\n")
		}
	}

	return result.String()
}
