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
			mcp.WithDescription("Multi-purpose code analysis. 'definitions': Precise symbol location (URI, line, char); use this output for 'hover', 'signature_help', 'rename', or 'get_range_content'. 'references': All symbol usages. 'workspace_symbols': Project-wide symbol search. 'document_symbols': File symbol outline. 'text_search': Workspace content search."),
			mcp.WithString("workspace_uri", mcp.Description("URI to project root (e.g., 'file:///home/user/my_project').")),
			mcp.WithString("query", mcp.Description("Symbol name (definitions/references/workspace_symbols), file URI (document_symbols), or text pattern (text_search)."), mcp.Required()),
			mcp.WithString("analysis_type", mcp.Description("Analysis type: 'definitions' (exact symbol location), 'references' (all usages), 'workspace_symbols' (symbol search), 'document_symbols' (file contents), 'text_search' (content search)."), mcp.Required()),
			mcp.WithNumber("offset", mcp.Description("Result offset (default: 0)."), mcp.DefaultNumber(0), mcp.Min(0)),
			mcp.WithNumber("limit", mcp.Description("Max results (default: 20, max: 100)."), mcp.Min(0), mcp.Max(100), mcp.DefaultNumber(20)),
		), func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
			workspaceUri := request.GetString("workspace_uri", "")

			query, err := request.RequireString("query")
			if err != nil {
				return mcp.NewToolResultError(err.Error()), nil
			}

			analysisType, err := request.RequireString("analysis_type")
			if err != nil {
				return mcp.NewToolResultError(err.Error()), nil
			}

			offset := request.GetInt("offset", 0)
			limit := request.GetInt("limit", 20)

			if workspaceUri == "" {
				dirs := bridge.AllowedDirectories()
				workspaceUri = dirs[0] // Get the first allow dir
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

			var languageStrings []string
			for _, lang := range languages {
				languageStrings = append(languageStrings, string(lang))
			}

			// Try to get clients for multiple languages with fallback
			clients, err := bridge.GetMultiLanguageClients(languageStrings)
			if err != nil || len(clients) == 0 {
				return mcp.NewToolResultError("No LSP clients available for detected languages"), nil
			}

			// Use the first available client in priority order
			var lspClient *lsp.LanguageClient

			var activeLanguage lsp.Language

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

// handleWorkspaceSymbols handles the 'workspace_symbols' analysis type
func handleWorkspaceSymbols(lspClient *lsp.LanguageClient, query string, offset, limit int, workspaceUri string, languages []lsp.Language, activeLanguage lsp.Language, response *strings.Builder) (*mcp.CallToolResult, error) {
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
				midOffset, err := safeUint32(nameLen / 2)
				if err != nil {
					midOffset = 0
				}
				midChar := startChar + midOffset
				fmt.Fprintf(response, "\t - Alternative: line=%d, character=%d\n", startLine, midChar)
			}

			// Provide the most reliable coordinate for hover operations
			bestHoverChar := startChar

			if nameLen > 1 {
				offset := min(nameLen/2, 5)
				offsetUint32, err := safeUint32(offset)
				if err != nil {
					offsetUint32 = 0
				}
				bestHoverChar = startChar + offsetUint32
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

	symbols, err := bridge.GetDocumentSymbols(docUri)
	if err != nil {
		logger.Error("Document symbols query failed", fmt.Sprintf("URI: %s, Error: %v", docUri, err))
		fmt.Fprintf(response, "ERROR: %v\n", err)
		return mcp.NewToolResultText(response.String()), nil
	}

	if len(symbols) == 0 {
		response.WriteString("NO_SYMBOLS\n")
		return mcp.NewToolResultText(response.String()), nil
	}

	// Apply pagination
	totalCount := len(symbols)
	if offset >= totalCount {
		fmt.Fprintf(response, "OFFSET_EXCEEDED: %d >= %d\n", offset, totalCount)
		return mcp.NewToolResultText(response.String()), nil
	}

	end := min(offset+limit, totalCount)
	paginatedSymbols := symbols[offset:end]

	// Structured header
	fmt.Fprintf(response, "SYMBOLS|%s|%d|%d|%d\n", docUri, offset, len(paginatedSymbols), totalCount)

	// Compact symbol format
	for i, sym := range paginatedSymbols {
		formatCompactSymbol(response, &sym, offset+i+1)
	}

	// Pagination indicator
	if end < totalCount {
		fmt.Fprintf(response, "MORE|%d\n", totalCount-end)
	}

	return mcp.NewToolResultText(response.String()), nil
}

func formatCompactSymbol(response *strings.Builder, sym *protocol.DocumentSymbol, index int) {
	// Format: INDEX|NAME|KIND|LINE:COL|RANGE_END
	startLine := sym.Range.Start.Line
	startChar := sym.Range.Start.Character
	endLine := sym.Range.End.Line
	endChar := sym.Range.End.Character

	fmt.Fprintf(response, "%d|%s|%s|%d:%d|%d:%d\n",
		index, sym.Name, symbolKindToString(sym.Kind),
		startLine, startChar, endLine, endChar)

	// Format children with indentation
	for _, child := range sym.Children {
		formatCompactSymbolChild(response, &child, index, 1)
	}
}

func formatCompactSymbolChild(response *strings.Builder, sym *protocol.DocumentSymbol, parentIndex, depth int) {
	indent := strings.Repeat("  ", depth)
	startLine := sym.Range.Start.Line
	startChar := sym.Range.Start.Character

	fmt.Fprintf(response, "%s%d.%d|%s|%s|%d:%d\n",
		indent, parentIndex, depth, sym.Name, symbolKindToString(sym.Kind),
		startLine, startChar)

	// Recursively format children
	for _, child := range sym.Children {
		formatCompactSymbolChild(response, &child, parentIndex, depth+1)
	}
}

// handleReferences handles the 'references' analysis type
func handleReferences(bridge interfaces.BridgeInterface, lspClient *lsp.LanguageClient, query string, offset, limit int, activeLanguage lsp.Language, response *strings.Builder) (*mcp.CallToolResult, error) {
	// Search for the symbol first
	symbols, err := lspClient.WorkspaceSymbols(query)
	if err != nil {
		fmt.Fprintf(response, "ERROR: %v\n", err)
		return mcp.NewToolResultText(response.String()), nil
	}

	if len(symbols) == 0 {
		fmt.Fprintf(response, "NO_SYMBOL: %s\n", query)
		return mcp.NewToolResultText(response.String()), nil
	}

	// Use the first symbol found
	symbol := symbols[0]
	switch v := symbol.Location.Value.(type) {
	case protocol.Location:
		uri := string(v.Uri)
		line := v.Range.Start.Line
		character := v.Range.Start.Character

		references, err := bridge.FindSymbolReferences(string(activeLanguage), uri, uint32(line), uint32(character), true)
		if err != nil {
			fmt.Fprintf(response, "ERROR: %v\n", err)
			return mcp.NewToolResultText(response.String()), nil
		}

		if len(references) == 0 {
			fmt.Fprintf(response, "NO_REFS: %s\n", symbol.Name)
			return mcp.NewToolResultText(response.String()), nil
		}

		// Apply pagination
		totalCount := len(references)
		if offset >= totalCount {
			fmt.Fprintf(response, "OFFSET_EXCEEDED: %d >= %d\n", offset, totalCount)
			return mcp.NewToolResultText(response.String()), nil
		}

		end := min(offset+limit, totalCount)
		paginatedRefs := references[offset:end]

		// Structured header: REFS|symbol|offset|count|total
		fmt.Fprintf(response, "REFS|%s|%d|%d|%d\n", symbol.Name, offset, len(paginatedRefs), totalCount)

		// Compact reference format
		for i, ref := range paginatedRefs {
			formatCompactReference(response, ref, offset+i+1)
		}

		// Pagination indicator
		if end < totalCount {
			fmt.Fprintf(response, "MORE|%d\n", totalCount-end)
		}

	default:
		fmt.Fprintf(response, "UNSUPPORTED_FORMAT: %T\n", v)
		return mcp.NewToolResultText(response.String()), nil
	}

	return mcp.NewToolResultText(response.String()), nil
}

func formatCompactReference(response *strings.Builder, ref any, index int) {
	// Parse the reference format from your example
	// Assuming ref is a Location with Range and URI
	refStr := fmt.Sprintf("%v", ref)

	// Extract line, character, and file from the reference string
	// This is a simplified parser - you may need to adjust based on actual ref type
	if location, ok := ref.(protocol.Location); ok {
		line := location.Range.Start.Line
		char := location.Range.Start.Character
		uri := string(location.Uri)

		// Format: INDEX|LINE:CHAR|FILE
		fmt.Fprintf(response, "%d|%d:%d|%s\n", index, line, char, uri)
	} else {
		// Fallback for unknown reference types
		fmt.Fprintf(response, "%d|%s\n", index, refStr)
	}
}

// handleDefinitions handles the 'definitions' analysis type
func handleDefinitions(bridge interfaces.BridgeInterface, lspClient *lsp.LanguageClient, query string, offset, limit int, activeLanguage lsp.Language, response *strings.Builder) (*mcp.CallToolResult, error) {
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

		definitions, err := bridge.FindSymbolDefinitions(string(activeLanguage), uri, uint32(line), uint32(character))
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
func handleTextSearch(bridge interfaces.BridgeInterface, query string, offset, limit int, activeLanguage lsp.Language, response *strings.Builder) (*mcp.CallToolResult, error) {
	response.WriteString("=== TEXT SEARCH ===\n")

	searchResults, err := bridge.SearchTextInWorkspace(string(activeLanguage), query)
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
