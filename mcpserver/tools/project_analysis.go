package tools

import (
	"context"
	"fmt"
	"path/filepath"
	"strings"

	"rockerboo/mcp-lsp-bridge/logger"
	"rockerboo/mcp-lsp-bridge/lsp"
	"rockerboo/mcp-lsp-bridge/interfaces"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
)

// RegisterProjectAnalysisTool registers the project_analysis tool
func RegisterProjectAnalysisTool(mcpServer *server.MCPServer, bridge interfaces.BridgeInterface) {
	mcpServer.AddTool(mcp.NewTool("project_analysis",
		mcp.WithDescription("Analyze project structure, find references, and search across files"),
		mcp.WithString("workspace_uri", mcp.Description("URI to the workspace/project root")),
		mcp.WithString("query", mcp.Description("Symbol or text to search for")),
		mcp.WithString("analysis_type", mcp.Description("Type of analysis: 'references', 'definitions', 'workspace_symbols', or 'text_search'")),
	), func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		workspaceUri, err := request.RequireString("workspace_uri")
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}

		query, err := request.RequireString("query")
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}

		analysisType, err := request.RequireString("analysis_type")
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}

		// Convert URI to local file path
		projectPath := strings.TrimPrefix(workspaceUri, "file://")

		// Use the project language detection method instead of single file inference
		languages, err := bridge.DetectProjectLanguages(projectPath)

		if err != nil {
			logger.Error("Project language detection failed", fmt.Sprintf("Workspace URI: %s, Error: %v", workspaceUri, err))
			return mcp.NewToolResultError(fmt.Sprintf("Failed to detect project languages: %v", err)), nil
		}

		// Use the first detected language
		if len(languages) == 0 {
			logger.Warn("No programming languages detected in project", fmt.Sprintf("Workspace URI: %s", workspaceUri))
			return mcp.NewToolResultError("No languages detected in project"), nil
		}

		// Try to get clients for multiple languages with fallback
		clients, err := bridge.GetMultiLanguageClients(languages)
		if err != nil || len(clients) == 0 {
			return mcp.NewToolResultError("No LSP clients available for detected languages"), nil
		}

		// Use the first available client in priority order
		var lspClient *lsp.LanguageClient
		var activeLanguage string
		for _, lang := range languages {
			if client, exists := clients[lang]; exists {
				if typedClient, ok := client.(*lsp.LanguageClient); ok {
					lspClient = typedClient
					activeLanguage = lang
					break
				}
			}
		}

		if lspClient == nil {
			return mcp.NewToolResultError("Invalid LSP client type"), nil
		}

		var response strings.Builder
		response.WriteString(fmt.Sprintf("Project Analysis: %s\n", analysisType))
		response.WriteString(fmt.Sprintf("Query: %s\n", query))
		response.WriteString(fmt.Sprintf("Workspace: %s\n", workspaceUri))
		response.WriteString(fmt.Sprintf("Detected Languages: %v\n", languages))
		response.WriteString(fmt.Sprintf("Active Language: %s\n\n", activeLanguage))

		switch analysisType {
		case "workspace_symbols":
			symbols, err := lspClient.WorkspaceSymbols(query)
			if err != nil {
				logger.Error("Workspace symbols query failed", fmt.Sprintf("Language: %s, Query: %s, Error: %v", activeLanguage, query, err))
				response.WriteString("=== WORKSPACE SYMBOLS ===\n")
				response.WriteString(fmt.Sprintf("Error: Failed to get workspace symbols for language '%s': %v\n", activeLanguage, err))
				break
			}

			// Limit results and format nicely
			maxResults := 20
			actualCount := len(symbols)
			if actualCount > maxResults {
				response.WriteString(fmt.Sprintf("Showing first %d of %d results:\n\n", maxResults, actualCount))
				symbols = symbols[:maxResults]
			} else {
				response.WriteString(fmt.Sprintf("Found %d results:\n\n", actualCount))
			}

			for i, symbol := range symbols {
				// Extract filename from URI
				uri := string(symbol.Location.Uri)
				filename := filepath.Base(strings.TrimPrefix(uri, "file://"))

				// Format symbol kind in a readable way
				kindStr := symbolKindToString(symbol.Kind)

				response.WriteString(fmt.Sprintf("%d. %s (%s) in %s\n", 
					i+1, 
					symbol.Name, 
					kindStr,
					filename))
			}

			if actualCount > maxResults {
				response.WriteString(fmt.Sprintf("\n... and %d more results\n", actualCount-maxResults))
			}

		case "references":
			// For references, search for the symbol first
			symbols, err := lspClient.WorkspaceSymbols(query)
			if err != nil {
				response.WriteString("=== REFERENCES ===\n")
				response.WriteString(fmt.Sprintf("Error: Cannot find references - workspace symbols search failed: %v\n", err))
				break
			}

			response.WriteString("=== REFERENCES ===\n")
			if len(symbols) == 0 {
				response.WriteString(fmt.Sprintf("No symbols found matching the query '%s'.\n", query))
				break
			}

			// Use the first symbol found
			symbol := symbols[0]
			uri := string(symbol.Location.Uri)
			line := symbol.Location.Range.Start.Line
			character := symbol.Location.Range.Start.Character

			references, err := bridge.FindSymbolReferences(activeLanguage, uri, int32(line), int32(character), true)
			if err != nil {
				response.WriteString(fmt.Sprintf("Failed to find references: %v\n", err))
				break
			}

			if len(references) == 0 {
				response.WriteString(fmt.Sprintf("No references found for symbol '%s'.\n", symbol.Name))
				break
			}

			response.WriteString(fmt.Sprintf("Found %d references for symbol '%s':\n", len(references), symbol.Name))
			for i, ref := range references {
				response.WriteString(fmt.Sprintf("%d. %v\n", i+1, ref))
			}

		case "definitions":
			// For definitions, search for the symbol first
			symbols, err := lspClient.WorkspaceSymbols(query)
			if err != nil {
				response.WriteString("=== DEFINITIONS ===\n")
				response.WriteString(fmt.Sprintf("Error: Cannot find definitions - workspace symbols search failed: %v\n", err))
				break
			}

			response.WriteString("=== DEFINITIONS ===\n")
			if len(symbols) == 0 {
				response.WriteString(fmt.Sprintf("No symbols found matching the query '%s'.\n", query))
				break
			}

			// Use the first symbol found
			symbol := symbols[0]
			uri := string(symbol.Location.Uri)
			line := symbol.Location.Range.Start.Line
			character := symbol.Location.Range.Start.Character

			definitions, err := bridge.FindSymbolDefinitions(activeLanguage, uri, int32(line), int32(character))
			if err != nil {
				response.WriteString(fmt.Sprintf("Failed to find definitions: %v\n", err))
				break
			}

			if len(definitions) == 0 {
				response.WriteString(fmt.Sprintf("No definitions found for symbol '%s'.\n", symbol.Name))
				break
			}

			response.WriteString(fmt.Sprintf("Found %d definitions for symbol '%s':\n", len(definitions), symbol.Name))
			for i, def := range definitions {
				response.WriteString(fmt.Sprintf("%d. %v\n", i+1, def))
			}

		case "text_search":
			response.WriteString("=== TEXT SEARCH ===\n")
			searchResults, err := bridge.SearchTextInWorkspace(activeLanguage, query)
			if err != nil {
				response.WriteString(fmt.Sprintf("Text search failed: %v\n", err))
				break
			}

			if len(searchResults) == 0 {
				response.WriteString(fmt.Sprintf("No results found for query '%s'.\n", query))
				break
			}

			// Limit results 
			maxResults := 20
			actualCount := len(searchResults)
			if actualCount > maxResults {
				response.WriteString(fmt.Sprintf("Showing first %d of %d results:\n\n", maxResults, actualCount))
				searchResults = searchResults[:maxResults]
			} else {
				response.WriteString(fmt.Sprintf("Found %d results:\n\n", actualCount))
			}
			
			for i, result := range searchResults {
				response.WriteString(fmt.Sprintf("%d. %v\n", i+1, result))
			}
			
			if actualCount > maxResults {
				response.WriteString(fmt.Sprintf("\n... and %d more results\n", actualCount-maxResults))
			}

		default:
			return mcp.NewToolResultError(fmt.Sprintf("Unknown analysis type: %s", analysisType)), nil
		}

		return mcp.NewToolResultText(response.String()), nil
	})
}