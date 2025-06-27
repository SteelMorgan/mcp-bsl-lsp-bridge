package tools

import (
	"context"
	"fmt"
	"path/filepath"
	"strings"

	"rockerboo/mcp-lsp-bridge/interfaces"
	"rockerboo/mcp-lsp-bridge/logger"
	"rockerboo/mcp-lsp-bridge/lsp"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
	"github.com/myleshyson/lsprotocol-go/protocol"
)

// RegisterProjectAnalysisTool registers the project_analysis tool
func RegisterProjectAnalysisTool(mcpServer ToolServer, bridge interfaces.BridgeInterface) {
	mcpServer.AddTool(ProjectAnalysisTool(bridge))
}

func ProjectAnalysisTool(bridge interfaces.BridgeInterface) (mcp.Tool, server.ToolHandlerFunc) {
	return mcp.NewTool(
			"project_analysis",
			mcp.WithDescription("Multi-purpose code analysis tool. Use 'definitions' for precise symbol targeting, 'references' for usage locations, 'workspace_symbols' for symbol discovery, 'document_symbols' for file exploration, 'text_search' for content search."),
			mcp.WithString("workspace_uri", mcp.Description("URI to the workspace/project root")),
			mcp.WithString("query", mcp.Description("Symbol name (for definitions/references/workspace_symbols) or file path (for document_symbols) or text pattern (for text_search)")),
			mcp.WithString("analysis_type", mcp.Description("Analysis type: 'definitions' (exact symbol location), 'references' (all usages), 'workspace_symbols' (symbol search), 'document_symbols' (file contents), 'text_search' (content search)")),
			mcp.WithNumber("offset", mcp.Description("Result offset for pagination (default: 0)")),
			mcp.WithNumber("limit", mcp.Description("Maximum number of results to return (default: 20, max: 100)")),
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

			// Parse pagination parameters with defaults
			offset := 0
			if offsetVal, err := request.RequireInt("offset"); err == nil {
				offset = offsetVal
			}

			limit := 20
			if limitVal, err := request.RequireInt("limit"); err == nil {
				if limitVal > 0 && limitVal <= 100 {
					limit = limitVal
				}
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
				logger.Warn("No programming languages detected in project", "Workspace URI: "+workspaceUri)
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
			fmt.Fprintf(&response, "Project Analysis: %s\n", analysisType)
			fmt.Fprintf(&response, "Query: %s\n", query)
			fmt.Fprintf(&response, "Workspace: %s\n", workspaceUri)
			fmt.Fprintf(&response, "Detected Languages: %v\n", languages)
			fmt.Fprintf(&response, "Active Language: %s\n\n", activeLanguage)

			switch analysisType {
			case "workspace_symbols":
				return handleWorkspaceSymbols(lspClient, query, offset, limit, workspaceUri, languages, activeLanguage, &response)
			case "document_symbols":
				return handleDocumentSymbols(bridge, query, offset, limit, &response)
			case "references":
				return handleReferences(bridge, lspClient, query, offset, limit, activeLanguage, &response)
			case "definitions":
				return handleDefinitions(bridge, lspClient, query, offset, limit, activeLanguage, &response)
			case "text_search":
				return handleTextSearch(bridge, query, offset, limit, activeLanguage, &response)
			default:
				return mcp.NewToolResultError("Unknown analysis type: " + analysisType), nil
			}
		}
}

// normalizeURI ensures the URI has the proper file:// scheme
func normalizeURI(uri string) string {
	// If it already has a file scheme, return as-is
	if strings.HasPrefix(uri, "file://") {
		return uri
	}

	// If it has any other scheme (http://, https://://, etc.), return as-is
	if strings.Contains(uri, "://") {
		return uri
	}

	// If it's an absolute path, convert to file URI
	if strings.HasPrefix(uri, "/") {
		return "file://" + uri
	}

	// If it's a relative path, convert to absolute path first, then to file URI
	if absPath, err := filepath.Abs(uri); err == nil {
		return "file://" + absPath
	}

	// Fallback: assume it's a file path and add file:// prefix
	return "file://" + uri
}

// formatDocumentSymbolWithTargeting formats a document symbol with precise targeting coordinates
func formatDocumentSymbolWithTargeting(response *strings.Builder, symbol protocol.DocumentSymbol, depth int, number int, docUri string) {
	indent := strings.Repeat("  ", depth)
	kindStr := symbolKindToString(symbol.Kind)

	// Extract full range coordinates
	startLine := symbol.Range.Start.Line
	startChar := symbol.Range.Start.Character
	endLine := symbol.Range.End.Line
	endChar := symbol.Range.End.Character

	// Use SelectionRange for precise targeting (this should point to the symbol name itself)
	targetLine := symbol.SelectionRange.Start.Line
	targetChar := symbol.SelectionRange.Start.Character

	// Check if SelectionRange is actually different from Range
	selectionRangeUseful := targetLine != startLine || targetChar != startChar

	if depth == 0 {
		fmt.Fprintf(response, "%s%d. %s (%s)\n",
			indent, number, symbol.Name, kindStr)
		if docUri != "" {
			fmt.Fprintf(response, "%s    URI: %s\n", indent, docUri)
		}
		fmt.Fprintf(response, "%s    Range: line=%d, character=%d to line=%d, character=%d\n",
			indent, startLine, startChar, endLine, endChar)

		if selectionRangeUseful {
			fmt.Fprintf(response, "%s    Target coordinates: line=%d, character=%d (precise symbol location)\n", indent, targetLine, targetChar)
			fmt.Fprintf(response, "%s    Recommended hover coordinate: uri=\"%s\", line=%d, character=%d\n", indent, docUri, targetLine, targetChar)
		} else {
			fmt.Fprintf(response, "%s    Target coordinates: line=%d, character=%d\n", indent, targetLine, targetChar)

			// Suggest appropriate tools based on symbol type and agent needs
			fmt.Fprintf(response, "%s    Recommended tools for this symbol:\n", indent)

			switch symbol.Kind {
			case protocol.SymbolKindFunction, protocol.SymbolKindMethod:
				fmt.Fprintf(response, "%s      - definitions: Get exact function declaration location\n", indent)
				fmt.Fprintf(response, "%s      - references: Find all usage locations of this function\n", indent)
				fmt.Fprintf(response, "%s      - hover: Get function signature and documentation (position-sensitive)\n", indent)

			case protocol.SymbolKindClass, protocol.SymbolKindInterface:
				fmt.Fprintf(response, "%s      - definitions: Get exact class/interface declaration\n", indent)
				fmt.Fprintf(response, "%s      - implementation: Find concrete implementations\n", indent)
				fmt.Fprintf(response, "%s      - references: Find all usage locations\n", indent)

			case protocol.SymbolKindVariable, protocol.SymbolKindConstant:
				fmt.Fprintf(response, "%s      - definitions: Get exact declaration location\n", indent)
				fmt.Fprintf(response, "%s      - references: Find all usage locations\n", indent)
				fmt.Fprintf(response, "%s      - hover: Get type information and value\n", indent)

			default:
				fmt.Fprintf(response, "%s      - definitions: Get exact declaration location\n", indent)
				fmt.Fprintf(response, "%s      - references: Find all usage locations\n", indent)
			}

			fmt.Fprintf(response, "%s    Example: project_analysis with analysis_type='definitions', query='%s'\n", indent, symbol.Name)
		}
	} else {
		fmt.Fprintf(response, "%s%s (%s)\n",
			indent, symbol.Name, kindStr)
		fmt.Fprintf(response, "%s    Range: line=%d, character=%d to line=%d, character=%d\n",
			indent, startLine, startChar, endLine, endChar)
		fmt.Fprintf(response, "%s    Target: line=%d, character=%d\n", indent, targetLine, targetChar)
	}

	// Recursively format children
	for _, child := range symbol.Children {
		formatDocumentSymbolWithTargeting(response, child, depth+1, 0, "")
	}
}

// handleWorkspaceSymbols handles the 'workspace_symbols' analysis type
func handleWorkspaceSymbols(lspClient *lsp.LanguageClient, query string, offset, limit int, workspaceUri string, languages []string, activeLanguage string, response *strings.Builder) (*mcp.CallToolResult, error) {
	symbols, err := lspClient.WorkspaceSymbols(query)
	if err != nil {
		logger.Error("Workspace symbols query failed", fmt.Sprintf("Language: %s, Query: %s, Error: %v", activeLanguage, query, err))
		response.WriteString("=== WORKSPACE SYMBOLS ===\n")
		fmt.Fprintf(response, "Error: Failed to get workspace symbols for language '%s': %v\n", activeLanguage, err)
		return mcp.NewToolResultText(response.String()), nil
	}

	// Apply pagination
	totalCount := len(symbols)

	// Handle offset
	if offset >= totalCount {
		fmt.Fprintf(response, "Offset %d exceeds total results (%d). No results to display.\n", offset, totalCount)
		return mcp.NewToolResultText(response.String()), nil
	}

	// Apply offset and limit
	end := min(offset+limit, totalCount)

	paginatedSymbols := symbols[offset:end]
	resultCount := len(paginatedSymbols)

	// Format pagination info
	if offset > 0 || end < totalCount {
		fmt.Fprintf(response, "Showing results %d-%d of %d total:\n", offset+1, offset+resultCount, totalCount)
	} else {
		fmt.Fprintf(response, "Found %d results:\n", totalCount)
	}

	for i, symbol := range paginatedSymbols {
		switch v := symbol.Location.Value.(type) {

		case protocol.Location:
			// Extract filename from URI
			uri := string(v.Uri)
			filename := filepath.Base(strings.TrimPrefix(uri, "file://"))

			// Format symbol kind in a readable way
			kindStr := symbolKindToString(symbol.Kind)

			// Extract location coordinates
			startLine := v.Range.Start.Line
			startChar := v.Range.Start.Character
			endLine := v.Range.End.Line
			endChar := v.Range.End.Character

			// Format with coordinates optimized for LLM agent consumption
			fmt.Fprintf(response, "%d. %s (%s) in %s\n",
				offset+i+1,
				symbol.Name,
				kindStr,
				filename)
			fmt.Fprintf(response, "\tURI: %s\n", uri)
			fmt.Fprintf(response, "\tRange: line=%d, character=%d to line=%d, character=%d\n",
				startLine, startChar, endLine, endChar)

			// Provide agent-optimized targeting coordinates
			nameLen := len(symbol.Name)
			response.WriteString("\tTarget coordinates for hover/references/definitions:\n")
			fmt.Fprintf(response, "\t - Primary: line=%d, character=%d\n", startLine, startChar)

			// Calculate precise positions within the identifier
			if nameLen > 3 {
				midChar := startChar + uint32(nameLen/2)
				fmt.Fprintf(response, "\t - Alternative: line=%d, character=%d\n", startLine, midChar)
			}

			// Provide the most reliable coordinate for hover operations
			bestHoverChar := startChar
			if nameLen > 1 {
				offset := min(nameLen/2, 5)
				bestHoverChar = startChar + uint32(offset)
			}
			fmt.Fprintf(response, "\tRecommended hover coordinate: uri=\"%s\", line=%d, character=%d\n",
				uri, startLine, bestHoverChar)
		default:
			response.WriteString("Unhandled hover method protocol.Location")
		}
	}

	// Show pagination info
	if end < totalCount {
		remaining := totalCount - end
		fmt.Fprintf(response, "\n... and %d more results available (use offset=%d to see next page)\n", remaining, end)
	}
	return mcp.NewToolResultText(response.String()), nil
}

// handleDocumentSymbols handles the 'document_symbols' analysis type
func handleDocumentSymbols(bridge interfaces.BridgeInterface, query string, offset, limit int, response *strings.Builder) (*mcp.CallToolResult, error) {
	// For document symbols, the query should be a file URI
	docUri := query
	if !strings.HasPrefix(query, "file://") {
		// If query is not a URI, treat it as a file path and normalize it
		docUri = normalizeURI(query)
	}

	response.WriteString("=== DOCUMENT SYMBOLS ===\n")
	fmt.Fprintf(response, "Document: %s\n", docUri)

	symbols, err := bridge.GetDocumentSymbols(docUri)
	if err != nil {
		logger.Error("Document symbols query failed", fmt.Sprintf("URI: %s, Error: %v", docUri, err))
		fmt.Fprintf(response, "Error: Failed to get document symbols: %v\n", err)
		return mcp.NewToolResultText(response.String()), nil
	}

	if len(symbols) == 0 {
		response.WriteString("No symbols found in document.\n")
		return mcp.NewToolResultText(response.String()), nil
	}

	// Apply pagination to document symbols
	totalCount := len(symbols)

	// Handle offset
	if offset >= totalCount {
		fmt.Fprintf(response, "Offset %d exceeds total results (%d). No results to display.\n", offset, totalCount)
		return mcp.NewToolResultText(response.String()), nil
	}

	// Apply offset and limit
	end := min(offset+limit, totalCount)

	paginatedSymbols := symbols[offset:end]
	resultCount := len(paginatedSymbols)

	// Format pagination info
	if offset > 0 || end < totalCount {
		fmt.Fprintf(response, "Showing symbols %d-%d of %d total:\n", offset+1, offset+resultCount, totalCount)
	} else {
		fmt.Fprintf(response, "Found %d symbols:\n", totalCount)
	}

	// Format symbols with hierarchy
	for i, sym := range paginatedSymbols {
		formatDocumentSymbolWithTargeting(response, sym, 0, offset+i+1, docUri)
	}

	// Show pagination info
	if end < totalCount {
		remaining := totalCount - end
		fmt.Fprintf(response, "\n... and %d more symbols available (use offset=%d to see next page)\n", remaining, end)
	}
	return mcp.NewToolResultText(response.String()), nil
}

// handleReferences handles the 'references' analysis type
func handleReferences(bridge interfaces.BridgeInterface, lspClient *lsp.LanguageClient, query string, offset, limit int, activeLanguage string, response *strings.Builder) (*mcp.CallToolResult, error) {
	// For references, search for the symbol first
	symbols, err := lspClient.WorkspaceSymbols(query)
	if err != nil {
		response.WriteString("=== REFERENCES ===\n")
		fmt.Fprintf(response, "Error: Cannot find references - workspace symbols search failed: %v\n", err)
		return mcp.NewToolResultText(response.String()), nil
	}

	response.WriteString("=== REFERENCES ===\n")
	if len(symbols) == 0 {
		fmt.Fprintf(response, "No symbols found matching the query '%s'.\n", query)
		return mcp.NewToolResultText(response.String()), nil
	}

	// Use the first symbol found
	symbol := symbols[0]
	switch v := symbol.Location.Value.(type) {
	case protocol.Location:
		uri := string(v.Uri)
		line := v.Range.Start.Line
		character := v.Range.Start.Character

		references, err := bridge.FindSymbolReferences(activeLanguage, uri, uint32(line), uint32(character), true)
		if err != nil {
			fmt.Fprintf(response, "Failed to find references: %v\n", err)
			return mcp.NewToolResultText(response.String()), nil
		}

		if len(references) == 0 {
			fmt.Fprintf(response, "No references found for symbol '%s'.\n", symbol.Name)
			return mcp.NewToolResultText(response.String()), nil
		}

		fmt.Fprintf(response, "Found %d references for symbol '%s':\n", len(references), symbol.Name)
		for i, ref := range references {
			fmt.Fprintf(response, "%d. %v\n", i+1, ref)
		}
	default:
		return mcp.NewToolResultError(fmt.Sprintf("Unsupported reference format: %s", v)), nil
	}
	return mcp.NewToolResultText(response.String()), nil
}

// handleDefinitions handles the 'definitions' analysis type
func handleDefinitions(bridge interfaces.BridgeInterface, lspClient *lsp.LanguageClient, query string, offset, limit int, activeLanguage string, response *strings.Builder) (*mcp.CallToolResult, error) {
	// For definitions, search for the symbol first
	symbols, err := lspClient.WorkspaceSymbols(query)
	if err != nil {
		response.WriteString("=== DEFINITIONS ===\n")
		fmt.Fprintf(response, "Error: Cannot find definitions - workspace symbols search failed: %v\n", err)
		return mcp.NewToolResultText(response.String()), nil
	}

	response.WriteString("=== DEFINITIONS ===\n")
	if len(symbols) == 0 {
		fmt.Fprintf(response, "No symbols found matching the query '%s'.\n", query)
		return mcp.NewToolResultText(response.String()), nil

	} else if len(symbols) > 1 {
		// If multiple symbols found, list them and ask for clarification
		fmt.Fprintf(response, "Multiple symbols found matching the query '%s'.\n", query)
		fmt.Fprintf(response, "Please clarify which one you mean:\n")
		// Iterate through symbols and format them similar to workspace_symbols
		for i, symbol := range symbols {
			if v, ok := symbol.Location.Value.(protocol.Location); ok {
				uri := string(v.Uri)
				filename := filepath.Base(strings.TrimPrefix(uri, "file://"))
				kindStr := symbolKindToString(symbol.Kind)
				startLine := v.Range.Start.Line
				startChar := v.Range.Start.Character
				endLine := v.Range.End.Line
				endChar := v.Range.End.Character

				fmt.Fprintf(response, "%d. %s (%s) in %s\n", i+1, symbol.Name, kindStr, filename)
				fmt.Fprintf(response, "	URI: %s\n", uri)
				fmt.Fprintf(response, "	Range: line=%d, character=%d to line=%d, character=%d\n",
					startLine, startChar, endLine, endChar)
			} else {
				fmt.Fprintf(response, "%d. %s (Unsupported Location Type: %T)\n", i+1, symbol.Name, symbol.Location.Value)
			}
		}
		fmt.Fprintf(response, "Please provide a more specific query or the full path to the file containing the desired symbol.\n")
		return mcp.NewToolResultText(response.String()), nil
	}

	// If only one symbol found, proceed to find its definitions
	// Use the first (and only) symbol found
	symbol := symbols[0]

	switch v := symbol.Location.Value.(type) {

	case protocol.Location:
		uri := string(v.Uri)
		line := v.Range.Start.Line
		character := v.Range.Start.Character

		definitions, err := bridge.FindSymbolDefinitions(activeLanguage, uri, uint32(line), uint32(character))
		if err != nil {
			fmt.Fprintf(response, "Failed to find definitions: %v\n", err)
			return mcp.NewToolResultText(response.String()), nil
		}

		if len(definitions) == 0 {
			fmt.Fprintf(response, "No definitions found for symbol '%s'.\n", symbol.Name)
			return mcp.NewToolResultText(response.String()), nil
		}

		fmt.Fprintf(response, "Found %d definitions for symbol '%s':\n", len(definitions), symbol.Name)
		for i, def := range definitions {
			// A definition can be LocationLink or Location (protocol.Or2[protocol.LocationLink, protocol.Location])
			// Need to switch on the value of the Or2
			if loc, ok := def.Value.(protocol.Location); ok {
				defUri := string(loc.Uri)
				defFilename := filepath.Base(strings.TrimPrefix(defUri, "file://"))
				defStartLine := loc.Range.Start.Line
				defStartChar := loc.Range.Start.Character
				defEndLine := loc.Range.End.Line
				defEndChar := loc.Range.End.Character
				fmt.Fprintf(response, "%d. %s:line=%d, character=%d to line=%d, character=%d\n",
					i+1, defFilename, defStartLine, defStartChar, defEndLine, defEndChar)
				fmt.Fprintf(response, "	URI: %s\n", defUri)
			} else if locLink, ok := def.Value.(protocol.LocationLink); ok {
				// LocationLink has OriginSelectionRange and TargetUri/Range/SelectionRange
				defUri := string(locLink.TargetUri)
				defFilename := filepath.Base(strings.TrimPrefix(defUri, "file://"))
				defStartLine := locLink.TargetRange.Start.Line
				defStartChar := locLink.TargetRange.Start.Character
				defEndLine := locLink.TargetRange.End.Line
				defEndChar := locLink.TargetRange.End.Character
				fmt.Fprintf(response, "%d. %s:line=%d, character=%d to line=%d, character=%d\n",
					i+1, defFilename, defStartLine, defStartChar, defEndLine, defEndChar)
				fmt.Fprintf(response, "	URI: %s\n", defUri)
			} else {
				fmt.Fprintf(response, "%d. Definition with unsupported type: %T\n", i+1, def.Value)
			}
		}
	default:
		return mcp.NewToolResultError(fmt.Sprintf("Unexpected symbol location format from workspace search: %T\n", v)), nil
	}
	return mcp.NewToolResultText(response.String()), nil
}

// handleTextSearch handles the 'text_search' analysis type
func handleTextSearch(bridge interfaces.BridgeInterface, query string, offset, limit int, activeLanguage string, response *strings.Builder) (*mcp.CallToolResult, error) {
	response.WriteString("=== TEXT SEARCH ===\n")
	searchResults, err := bridge.SearchTextInWorkspace(activeLanguage, query)
	if err != nil {
		fmt.Fprintf(response, "Text search failed: %v\n", err)
		return mcp.NewToolResultText(response.String()), nil
	}

	if len(searchResults) == 0 {
		fmt.Fprintf(response, "No results found for query '%s'.\n", query)
		return mcp.NewToolResultText(response.String()), nil
	}

	// Apply pagination to text search results
	totalCount := len(searchResults)

	// Handle offset
	if offset >= totalCount {
		fmt.Fprintf(response, "Offset %d exceeds total results (%d). No results to display.\n", offset, totalCount)
		return mcp.NewToolResultText(response.String()), nil
	}

	// Apply offset and limit
	end := min(offset+limit, totalCount)

	paginatedResults := searchResults[offset:end]
	resultCount := len(paginatedResults)

	// Format pagination info
	if offset > 0 || end < totalCount {
		fmt.Fprintf(response, "Showing results %d-%d of %d total:\n", offset+1, offset+resultCount, totalCount)
	} else {
		fmt.Fprintf(response, "Found %d results:\n", totalCount)
	}

	for i, result := range paginatedResults {
		fmt.Fprintf(response, "%d. %v\n", offset+i+1, result)
	}

	// Show pagination info
	if end < totalCount {
		remaining := totalCount - end
		fmt.Fprintf(response, "\n... and %d more results available (use offset=%d to see next page)\n", remaining, end)
	}
	return mcp.NewToolResultText(response.String()), nil
}
