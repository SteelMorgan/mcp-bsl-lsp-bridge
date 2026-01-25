package tools

import (
	"context"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	"rockerboo/mcp-lsp-bridge/analysis"
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

// RegisterProjectAnalysisTool registers the project_analysis tool
func RegisterProjectAnalysisTool(mcpServer ToolServer, bridge interfaces.BridgeInterface) {
	mcpServer.AddTool(ProjectAnalysisTool(bridge))
}

func ProjectAnalysisTool(bridge interfaces.BridgeInterface) (mcp.Tool, server.ToolHandlerFunc) {
	return mcp.NewTool(
			"project_analysis",
			mcp.WithDescription(`Multi-purpose code analysis tool with multiple analysis types. Use this as a "Swiss army knife" when exploring unfamiliar codebases.

USAGE:
- Find symbols: analysis_type="workspace_symbols", query="calculateTotal"
- Analyze files: analysis_type="file_analysis", query="src/auth.go"
- Workspace overview: analysis_type="workspace_analysis", query="entire_project"

ANALYSIS TYPES:
workspace_symbols, document_symbols, references, definitions, text_search, workspace_analysis, symbol_relationships, file_analysis, pattern_analysis

QUICK GUIDE (what each type does + what query means):
- workspace_symbols: find symbol candidates in the whole project. query = symbol name / substring.
- document_symbols: list symbols in a single file. query = file path or file URI.
- references: find usage sites of the first matching symbol (includes declaration). query = symbol name.
- definitions: find definition location(s) of the first matching symbol. query = symbol name.
- text_search: search raw text across workspace files (fast fallback when LSP is not enough). query = substring.
- file_analysis: analyze a file (structure/metrics/patterns). query = file path or file URI.
- workspace_analysis: high-level overview of the workspace. query = "entire_project" (or any placeholder).
- symbol_relationships: analyze relationships around a symbol. query = symbol name.
- pattern_analysis: analyze patterns across files. query = keyword/pattern.

PAGINATION:
- offset: skip N results (default 0)
- limit: max results (default 20, max 100)

OPTIONAL:
- workspace_uri: project root URI (defaults to the first allowed directory).

PARAMETERS: analysis_type (required), query (required), limit (default: 20)`),
			mcp.WithDestructiveHintAnnotation(false),
			mcp.WithString("workspace_uri", mcp.Description("Project root URI (optional, defaults to detected project root).")),
			mcp.WithString("query", mcp.Description("Symbol name OR file path OR text pattern (see examples above)."), mcp.Required()),
			mcp.WithString("analysis_type", mcp.Description("Choose: workspace_symbols, document_symbols, references, definitions, text_search, workspace_analysis, symbol_relationships, file_analysis, pattern_analysis."), mcp.Required()),
			mcp.WithNumber("offset", mcp.Description("Skip N results (default: 0)."), mcp.DefaultNumber(0), mcp.Min(0)),
			mcp.WithNumber("limit", mcp.Description("Max results (default: 20)."), mcp.Min(0), mcp.Max(100), mcp.DefaultNumber(20)),
		), func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
			workspaceUri := request.GetString("workspace_uri", "")

			// Normalize workspace_uri - handles both host paths and container paths
			if workspaceUri != "" {
				workspaceUri = bridge.NormalizeURIForLSP(workspaceUri)
			}

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

			if result, ok := CheckReadyOrReturn(bridge); !ok {
				return result, nil
			}

			// Handle options parameter - since GetObject might not be available, create empty map for now
			options := make(map[string]interface{})

			if workspaceUri == "" {
				dirs := bridge.AllowedDirectories()
				workspaceUri = dirs[0] // Get the first allow dir
			}

			// Convert URI to local file path (Windows-safe)
			projectPath := utils.URIToFilePath(workspaceUri)

			// Fast path: if clients are already connected (e.g., via auto-connect),
			// use their languages instead of expensive filesystem scan.
			// This is especially important for large BSL projects (2000+ files).
			var languages []types.Language
			if connectedLangs := bridge.GetConnectedLanguages(); len(connectedLangs) > 0 {
				languages = connectedLangs
				logger.Debug("Using pre-connected languages: " + fmt.Sprintf("%v", languages))
			} else {
				// Fallback to project language detection (slower)
				languages, err = bridge.DetectProjectLanguages(projectPath)
				if err != nil {
					logger.Error("Project language detection failed", fmt.Sprintf("Workspace URI: %s, Error: %v", workspaceUri, err))
					return mcp.NewToolResultError(fmt.Sprintf("Failed to detect project languages: %v", err)), nil
				}
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
			var lspClient types.LanguageClientInterface

			var activeLanguage types.Language

			for _, lang := range languages {
				if client, exists := clients[lang]; exists {
					lspClient = client
					activeLanguage = lang
					break
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
				return handleWorkspaceSymbols(lspClient, query, offset, limit, activeLanguage, &response)
			case "document_symbols":
				return handleDocumentSymbols(bridge, query, offset, limit, &response)
			case "references":
				return handleReferences(bridge, clients, query, offset, limit, activeLanguage, &response)
			case "definitions":
				return handleDefinitions(bridge, lspClient, query, activeLanguage, &response)
			case "text_search":
				return handleTextSearch(ctx, bridge, projectPath, query, offset, limit, activeLanguage, &response)
			case "workspace_analysis":
				return handleWorkspaceAnalysis(bridge, clients, query, options, &response)
			case "symbol_relationships":
				return handleSymbolRelationships(bridge, clients, query, options, &response)
			case "file_analysis":
				return handleFileAnalysis(bridge, clients, query, options, &response)
			case "pattern_analysis":
				return handlePatternAnalysis(bridge, clients, query, options, &response)
			default:
				return mcp.NewToolResultError("Unknown analysis type: " + analysisType), nil
			}
		}
}

type textSearchHit struct {
	URI       string
	Line      int
	Character int
	Preview   string
}

func handleTextSearch(ctx context.Context, bridge interfaces.BridgeInterface, projectPath string, query string, offset, limit int, activeLanguage types.Language, response *strings.Builder) (*mcp.CallToolResult, error) {
	query = strings.TrimSpace(query)
	if query == "" {
		return mcp.NewToolResultError("query must be non-empty for text_search"), nil
	}

	if limit < 0 {
		limit = 0
	}
	if offset < 0 {
		offset = 0
	}

	exts := defaultTextSearchExtensions(activeLanguage)
	extSet := make(map[string]struct{}, len(exts))
	for _, e := range exts {
		extSet[strings.ToLower(e)] = struct{}{}
	}

	ignoredDirs := map[string]struct{}{
		".git":         {},
		".hg":          {},
		".svn":         {},
		".idea":        {},
		".vscode":      {},
		"node_modules": {},
		"vendor":       {},
		"dist":         {},
		"build":        {},
		"out":          {},
		"target":       {},
		"_bin":         {},
	}

	const maxFileSizeBytes int64 = 2 * 1024 * 1024 // 2MB guardrail
	const maxPreviewLen = 220

	var (
		scannedFiles  int
		seenMatches   int
		returnedHits  []textSearchHit
		truncatedScan bool
	)

	errStopWalk := errors.New("text_search: stop walk")

	need := offset + limit
	if limit == 0 {
		need = offset // still scan until offset? but nothing to return → we can short-circuit.
	}

	walkErr := filepath.WalkDir(projectPath, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return nil
		}
		if ctx.Err() != nil {
			return ctx.Err()
		}

		if d.IsDir() {
			if _, ok := ignoredDirs[strings.ToLower(d.Name())]; ok {
				return fs.SkipDir
			}
			return nil
		}

		ext := strings.ToLower(filepath.Ext(d.Name()))
		if len(extSet) > 0 {
			if _, ok := extSet[ext]; !ok {
				return nil
			}
		}

		info, statErr := d.Info()
		if statErr != nil {
			return nil
		}
		if info.Size() > maxFileSizeBytes {
			return nil
		}

		f, openErr := os.Open(path) // #nosec G304 -- walking within user workspace
		if openErr != nil {
			return nil
		}
		defer func() { _ = f.Close() }()

		scannedFiles++

		// Read as bytes and split by '\n' manually to avoid Scanner token limits on huge lines.
		data, readErr := io.ReadAll(f)
		if readErr != nil {
			return nil
		}

		// Simple line scan
		start := 0
		line := 0
		for start <= len(data) {
			end := start
			for end < len(data) && data[end] != '\n' {
				end++
			}
			// Strip trailing '\r'
			lineBytes := data[start:end]
			if len(lineBytes) > 0 && lineBytes[len(lineBytes)-1] == '\r' {
				lineBytes = lineBytes[:len(lineBytes)-1]
			}

			lineStr := string(lineBytes)
			if idx := strings.Index(lineStr, query); idx >= 0 {
				seenMatches++
				if seenMatches > offset && limit > 0 && len(returnedHits) < limit {
					u := bridge.NormalizeURIForLSP(utils.FilePathToURI(path))
					preview := strings.TrimSpace(lineStr)
					if len(preview) > maxPreviewLen {
						preview = preview[:maxPreviewLen] + "…"
					}
					returnedHits = append(returnedHits, textSearchHit{
						URI:       u,
						Line:      line,
						Character: idx,
						Preview:   preview,
					})
				}

				// Early stop if we have enough matches for this page.
				if need > 0 && seenMatches >= need && (limit == 0 || len(returnedHits) >= limit) {
					truncatedScan = true
					return errStopWalk
				}
			}

			line++
			if end >= len(data) {
				break
			}
			start = end + 1
		}

		return nil
	})

	if walkErr != nil && !errors.Is(walkErr, errStopWalk) && walkErr != context.Canceled && walkErr != context.DeadlineExceeded {
		logger.Warn(fmt.Sprintf("text_search: walk error: %v", walkErr))
	}

	fmt.Fprintf(response, "TEXT_SEARCH|%s|offset=%d|limit=%d\n", query, offset, limit)
	fmt.Fprintf(response, "LANG=%s|EXTS=%s\n", activeLanguage, strings.Join(exts, ","))
	fmt.Fprintf(response, "SCANNED_FILES=%d|SEEN_MATCHES=%d|RETURNED=%d|TRUNCATED=%t\n", scannedFiles, seenMatches, len(returnedHits), truncatedScan)
	response.WriteString("\n")

	if len(returnedHits) == 0 {
		response.WriteString("NO_MATCHES\n")
		return mcp.NewToolResultText(response.String()), nil
	}

	for i, h := range returnedHits {
		fmt.Fprintf(response, "%d|%d:%d|%s|%s\n", offset+i+1, h.Line, h.Character, h.URI, h.Preview)
	}

	if truncatedScan {
		next := offset + len(returnedHits)
		fmt.Fprintf(response, "MORE|next_offset=%d\n", next)
	}

	return mcp.NewToolResultText(response.String()), nil
}

func defaultTextSearchExtensions(lang types.Language) []string {
	switch strings.ToLower(string(lang)) {
	case "bsl":
		return []string{".bsl", ".os"}
	case "go":
		return []string{".go"}
	case "python":
		return []string{".py"}
	case "typescript":
		return []string{".ts", ".tsx", ".js", ".jsx"}
	default:
		// Safe default: keep it narrow to avoid scanning binaries/noise.
		return []string{".bsl"}
	}
}

// handleWorkspaceSymbols handles the 'workspace_symbols' analysis type
func handleWorkspaceSymbols(lspClient types.LanguageClientInterface, query string, offset, limit int, activeLanguage types.Language, response *strings.Builder) (*mcp.CallToolResult, error) {
	symbols, err := lspClient.WorkspaceSymbols(query)
	if err != nil {
		logger.Error("Workspace symbols query failed", fmt.Sprintf("Language: %s, Query: %s, Error: %v", activeLanguage, query, err))
		response.WriteString("WORKSPACE SYMBOLS:\n")

		// Check for unhandled method error and provide helpful message
		errorMsg := err.Error()
		if strings.Contains(errorMsg, "Unhandled method workspace/symbol") {
			fmt.Fprintf(response, "Warning: The %s language server does not support workspace symbol search.\n", activeLanguage)
			fmt.Fprintf(response, "This is a known limitation of some language servers.\n")
			fmt.Fprintf(response, "Try using 'document_symbols' analysis type with a specific file path instead.\n")
		} else {
			fmt.Fprintf(response, "Error: Failed to get workspace symbols for language '%s': %v\n", activeLanguage, err)
		}

		return mcp.NewToolResultText(response.String()), nil
	}

	// Apply pagination using shared utility
	paginatedSymbols, paginationResult := ApplyPagination(symbols, offset, limit)

	// Handle offset exceeding total
	if paginationResult.Count == 0 {
		fmt.Fprintf(response, "%s\n", FormatPaginationInfo(paginationResult))
		return mcp.NewToolResultText(response.String()), nil
	}

	// Format pagination info
	fmt.Fprintf(response, "%s:\n", FormatPaginationInfo(paginationResult))

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

	// Show pagination controls
	fmt.Fprintf(response, "%s\n", FormatPaginationControls(paginationResult))

	return mcp.NewToolResultText(response.String()), nil
}

// handleDocumentSymbols handles the 'document_symbols' analysis type
func handleDocumentSymbols(bridge interfaces.BridgeInterface, query string, offset, limit int, response *strings.Builder) (*mcp.CallToolResult, error) {
	// For document symbols, the query should be a file URI
	// NormalizeURIForLSP handles both host paths and container paths for Docker mode
	docUri := bridge.NormalizeURIForLSP(query)

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

// Handles the 'references' analysis type
func handleReferences(bridge interfaces.BridgeInterface, clients map[types.Language]types.LanguageClientInterface, query string, offset, limit int, activeLanguage types.Language, response *strings.Builder) (*mcp.CallToolResult, error) {
	// Convert clients to async operations
	ops := collections.TransformMap(clients, func(client types.LanguageClientInterface) func() ([]protocol.WorkspaceSymbol, error) {
		return func() ([]protocol.WorkspaceSymbol, error) {
			return client.WorkspaceSymbols(query)
		}
	})

	// Execute symbol search across all clients in parallel
	ctx := context.Background() // TODO: Pass context from caller
	results, err := async.MapWithKeys(ctx, ops)
	if err != nil {
		fmt.Fprintf(response, "ERROR: %v\n", err)
		return mcp.NewToolResultText(response.String()), nil
	}

	// Flatten results and collect errors
	flattened := utils.FlattenKeyedResults(results)
	allSymbols := flattened.Values

	// Log any errors from individual clients with helpful context
	for _, err := range flattened.Errors {
		errorMsg := err.Error()
		if strings.Contains(errorMsg, "Unhandled method workspace/symbol") {
			logger.Warn(fmt.Sprintf("Language server does not support workspace/symbol method: %v", err))
		} else {
			logger.Warn(fmt.Sprintf("Workspace symbols search failed: %v", err))
		}
	}

	if len(allSymbols) == 0 {
		fmt.Fprintf(response, "NO_SYMBOL: %s\n", query)
		return mcp.NewToolResultText(response.String()), nil
	}

	// Use the first symbol found
	symbol := allSymbols[0]
	switch v := symbol.Location.Value.(type) {
	case protocol.Location:
		uri := string(v.Uri)
		line := v.Range.Start.Line
		character := v.Range.Start.Character

		// Get precise coordinates using semantic tokens
		preciseChar := FindPreciseCharacterPosition(bridge, uri, line, character, symbol.Name)

		references, err := bridge.FindSymbolReferences(string(activeLanguage), uri, uint32(line), uint32(preciseChar), true)
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
func handleDefinitions(bridge interfaces.BridgeInterface, lspClient types.LanguageClientInterface, query string, activeLanguage types.Language, response *strings.Builder) (*mcp.CallToolResult, error) {
	// For definitions, search for the symbol first
	symbols, err := lspClient.WorkspaceSymbols(query)
	if err != nil {
		response.WriteString("DEFINITIONS:\n")

		// Check for unhandled method error and provide helpful message
		errorMsg := err.Error()
		if strings.Contains(errorMsg, "Unhandled method workspace/symbol") {
			fmt.Fprintf(response, "Warning: The %s language server does not support workspace symbol search.\n", activeLanguage)
			fmt.Fprintf(response, "This is a known limitation of some language servers.\n")
			fmt.Fprintf(response, "Try using 'document_symbols' analysis type with a specific file path instead.\n")
		} else {
			fmt.Fprintf(response, "Error: Cannot find definitions - workspace symbols search failed: %v\n", err)
		}

		return mcp.NewToolResultText(response.String()), nil
	}

	response.WriteString("DEFINITIONS:\n")

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

		// Get precise coordinates using semantic tokens
		preciseChar := FindPreciseCharacterPosition(bridge, uri, line, character, symbol.Name)

		definitions, err := bridge.FindSymbolDefinitions(string(activeLanguage), uri, uint32(line), uint32(preciseChar))
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

// ComplexityMetrics represents file complexity metrics
type ComplexityMetrics struct {
	TotalLines      int
	FunctionCount   int
	ClassCount      int
	VariableCount   int
	ComplexityScore float64
	ComplexityLevel string
}

// handleFileAnalysis handles the 'file_analysis' analysis type
func handleFileAnalysis(bridge interfaces.BridgeInterface, clients map[types.Language]types.LanguageClientInterface, query string, options map[string]interface{}, response *strings.Builder) (*mcp.CallToolResult, error) {
	response.WriteString("FILE ANALYSIS:\n")

	// Try intelligent file context resolution first
	var fileUri string
	if strings.HasPrefix(query, "file://") {
		// Already a URI, use as-is
		fileUri = query
	} else {
		// Get workspace directory from bridge
		dirs := bridge.AllowedDirectories()
		if len(dirs) == 0 {
			return mcp.NewToolResultError("no workspace directories configured"), nil
		}
		workspaceDir := dirs[0] // Use first allowed directory as workspace

		// Try to resolve the file context to an actual file path
		resolved, err := ResolveFileContext(bridge, query, workspaceDir)
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("file context resolution failed: %v", err)), nil
		}

		// If we found a specific file, use it
		if resolved.ResolvedPath != "" {
			fileUri = bridge.NormalizeURIForLSP(resolved.ResolvedPath)
		} else if resolved.ErrorMessage != "" {
			// File not found - provide helpful error with suggestions
			return mcp.NewToolResultError(resolved.ErrorMessage), nil
		} else {
			// Fallback to original behavior - NormalizeURIForLSP handles Docker path mapping
			fileUri = bridge.NormalizeURIForLSP(query)
		}
	}

	fmt.Fprintf(response, "Analyzing file: %s\n\n", fileUri)

	// Create analysis engine with clients and language detector
	analyzer := analysis.NewProjectAnalyzer(clients,
		analysis.WithLanguageDetector(bridge.InferLanguage))

	// Create analysis request
	request := analysis.AnalysisRequest{
		Type:    analysis.FileAnalysis,
		Target:  fileUri,
		Scope:   "file",
		Depth:   "detailed",
		Options: options,
	}

	// Perform analysis
	result, err := analyzer.Analyze(request)
	if err != nil {
		fmt.Fprintf(response, "Analysis failed: %v\n", err)
		return mcp.NewToolResultText(response.String()), nil
	}

	// Format results
	if fileData, ok := result.Data.(analysis.FileAnalysisData); ok {
		fmt.Fprintf(response, "Language: %s\n", fileData.Language)
		fmt.Fprintf(response, "Symbols found: %d\n\n", len(fileData.Symbols))

		// Complexity metrics
		complexity := fileData.Complexity
		fmt.Fprintf(response, "COMPLEXITY METRICS:\n")
		fmt.Fprintf(response, "  Total Lines: %d\n", complexity.TotalLines)
		fmt.Fprintf(response, "  Functions: %d\n", complexity.FunctionCount)
		fmt.Fprintf(response, "  Classes: %d\n", complexity.ClassCount)
		fmt.Fprintf(response, "  Variables: %d\n", complexity.VariableCount)
		fmt.Fprintf(response, "  Complexity Score: %.2f\n", complexity.ComplexityScore)
		fmt.Fprintf(response, "  Complexity Level: %s\n\n", complexity.ComplexityLevel)

		// Import/Export analysis
		importExport := fileData.ImportExport
		fmt.Fprintf(response, "IMPORT/EXPORT ANALYSIS:\n")
		fmt.Fprintf(response, "  Imports: %d\n", len(importExport.Imports))
		fmt.Fprintf(response, "  Exports: %d\n", len(importExport.Exports))
		fmt.Fprintf(response, "  External Dependencies: %d\n", len(importExport.ExternalDeps))
		fmt.Fprintf(response, "  Internal Dependencies: %d\n", len(importExport.InternalDeps))
		fmt.Fprintf(response, "  Circular Dependencies: %d\n", len(importExport.CircularDeps))
		fmt.Fprintf(response, "  Unused Imports: %d\n\n", len(importExport.UnusedImports))

		// Cross-file relationships
		fmt.Fprintf(response, "CROSS-FILE RELATIONSHIPS:\n")
		fmt.Fprintf(response, "  Related files: %d\n", len(fileData.CrossFileRelations))
		for _, relation := range fileData.CrossFileRelations {
			fmt.Fprintf(response, "  - %s (%s, strength: %.2f)\n",
				relation.TargetFile, relation.RelationType, relation.Strength)
		}

		// Code quality metrics
		quality := fileData.CodeQuality
		fmt.Fprintf(response, "\nCODE QUALITY METRICS:\n")
		fmt.Fprintf(response, "  Duplication Score: %.2f\n", quality.DuplicationScore)
		fmt.Fprintf(response, "  Cohesion Score: %.2f\n", quality.CohesionScore)
		fmt.Fprintf(response, "  Coupling Score: %.2f\n", quality.CouplingScore)
		fmt.Fprintf(response, "  Maintainability Index: %.2f\n", quality.MaintainabilityIndex)
		fmt.Fprintf(response, "  Test Coverage: %.2f%%\n", quality.TestCoverage*100)
		fmt.Fprintf(response, "  Documentation Score: %.2f\n\n", quality.DocumentationScore)

		// Recommendations
		fmt.Fprintf(response, "RECOMMENDATIONS:\n")
		for _, rec := range fileData.Recommendations {
			fmt.Fprintf(response, "  - [%s] %s: %s (effort: %s)\n",
				rec.Priority, rec.Type, rec.Description, rec.Effort)
		}

		// Analysis metadata
		fmt.Fprintf(response, "\nANALYSIS METADATA:\n")
		fmt.Fprintf(response, "  Duration: %v\n", result.Metadata.Duration)
		fmt.Fprintf(response, "  Languages used: %v\n", result.Metadata.LanguagesUsed)
		if len(result.Metadata.Errors) > 0 {
			fmt.Fprintf(response, "- Errors: %d\n", len(result.Metadata.Errors))
			for _, err := range result.Metadata.Errors {
				languageInfo := "unknown"
				if err.Language != "" {
					languageInfo = string(err.Language)
				} else {
					// Try to find the language from the metadata
					for _, lang := range result.Metadata.LanguagesUsed {
						languageInfo = string(lang)
						break
					}
				}
				fmt.Fprintf(response, "  - [%s] %s\n", languageInfo, err.Message)
			}
		}
	}

	return mcp.NewToolResultText(response.String()), nil
}

// handlePatternAnalysis handles the 'pattern_analysis' analysis type
func handlePatternAnalysis(bridge interfaces.BridgeInterface, clients map[types.Language]types.LanguageClientInterface, query string, options map[string]interface{}, response *strings.Builder) (*mcp.CallToolResult, error) {
	response.WriteString("PATTERN ANALYSIS:\n")

	// Determine pattern type from options or use query as pattern type
	patternType := query
	if pt, exists := options["pattern_type"]; exists {
		if ptStr, ok := pt.(string); ok {
			patternType = ptStr
		}
	}

	fmt.Fprintf(response, "Pattern Type: %s\n\n", patternType)

	// Create analysis engine with clients and language detector
	analyzer := analysis.NewProjectAnalyzer(clients,
		analysis.WithLanguageDetector(bridge.InferLanguage))

	// Add pattern_type to options if not present
	if options == nil {
		options = make(map[string]interface{})
	}
	options["pattern_type"] = patternType

	// Create analysis request
	request := analysis.AnalysisRequest{
		Type:    analysis.PatternAnalysis,
		Target:  patternType,
		Scope:   "project",
		Depth:   "detailed",
		Options: options,
	}

	// Perform analysis
	result, err := analyzer.Analyze(request)
	if err != nil {
		fmt.Fprintf(response, "Analysis failed: %v\n", err)
		return mcp.NewToolResultText(response.String()), nil
	}

	// Format results
	if patternData, ok := result.Data.(analysis.PatternAnalysisData); ok {
		fmt.Fprintf(response, "Scope: %s\n", patternData.Scope)
		fmt.Fprintf(response, "Consistency Score: %.1f%%\n\n", patternData.ConsistencyScore*100)

		// Pattern instances
		fmt.Fprintf(response, "PATTERN INSTANCES FOUND:\n")
		for i, instance := range patternData.PatternInstances {
			fmt.Fprintf(response, "%d. %s (confidence: %.1f%%, quality: %s)\n",
				i+1, instance.Pattern, instance.Confidence*100, instance.Quality)
			for _, variation := range instance.Variations {
				fmt.Fprintf(response, "   - Variation: %s\n", variation)
			}
		}

		// Pattern violations
		if len(patternData.Violations) > 0 {
			fmt.Fprintf(response, "\nPATTERN VIOLATIONS:\n")
			for i, violation := range patternData.Violations {
				fmt.Fprintf(response, "%d. [%s] %s\n", i+1, violation.Severity, violation.Rule)
				fmt.Fprintf(response, "   Expected: %s\n", violation.Expected)
				fmt.Fprintf(response, "   Actual: %s\n", violation.Actual)
				fmt.Fprintf(response, "   Suggestion: %s\n", violation.Suggestion)
			}
		}

		// Trend analysis
		trend := patternData.TrendAnalysis
		fmt.Fprintf(response, "\nTREND ANALYSIS:\n")
		fmt.Fprintf(response, "  Direction: %s (confidence: %.1f%%)\n", trend.Direction, trend.Confidence*100)
		fmt.Fprintf(response, "  Contributing factors:\n")
		for _, factor := range trend.Factors {
			fmt.Fprintf(response, "    - %s\n", factor)
		}
		fmt.Fprintf(response, "  Predictions:\n")
		for _, prediction := range trend.Predictions {
			fmt.Fprintf(response, "    - %s\n", prediction)
		}

		// Analysis metadata
		fmt.Fprintf(response, "\nANALYSIS METADATA:\n")
		fmt.Fprintf(response, "  Duration: %v\n", result.Metadata.Duration)
		fmt.Fprintf(response, "  Languages used: %v\n", result.Metadata.LanguagesUsed)
		if len(result.Metadata.Errors) > 0 {
			fmt.Fprintf(response, "- Errors: %d\n", len(result.Metadata.Errors))
			for _, err := range result.Metadata.Errors {
				languageInfo := "unknown"
				if err.Language != "" {
					languageInfo = string(err.Language)
				} else {
					// Try to find the language from the metadata
					for _, lang := range result.Metadata.LanguagesUsed {
						languageInfo = string(lang)
						break
					}
				}
				fmt.Fprintf(response, "  - [%s] %s\n", languageInfo, err.Message)
			}
		}
	}

	return mcp.NewToolResultText(response.String()), nil
}

// handleWorkspaceAnalysis handles the 'workspace_analysis' analysis type
func handleWorkspaceAnalysis(bridge interfaces.BridgeInterface, clients map[types.Language]types.LanguageClientInterface, query string, options map[string]interface{}, response *strings.Builder) (*mcp.CallToolResult, error) {
	response.WriteString("WORKSPACE ANALYSIS:\n")

	fmt.Fprintf(response, "Analyzing workspace for: %s\n\n", query)

	// Create analysis engine with clients and language detector
	analyzer := analysis.NewProjectAnalyzer(clients,
		analysis.WithLanguageDetector(bridge.InferLanguage))

	// Create analysis request
	request := analysis.AnalysisRequest{
		Type:    analysis.WorkspaceAnalysis,
		Target:  query,
		Scope:   "project",
		Depth:   "comprehensive",
		Options: options,
	}

	// Perform analysis
	result, err := analyzer.Analyze(request)
	if err != nil {
		fmt.Fprintf(response, "Analysis failed: %v\n", err)
		return mcp.NewToolResultText(response.String()), nil
	}

	// Format results
	if workspaceData, ok := result.Data.(analysis.WorkspaceAnalysisData); ok {
		fmt.Fprintf(response, "LANGUAGE DISTRIBUTION:\n")
		for lang, stats := range workspaceData.LanguageDistribution {
			fmt.Fprintf(response, "- %s: %d files (%.1f%%), %d symbols, avg complexity: %.2f\n",
				lang, stats.FileCount, stats.Percentage, stats.SymbolCount, stats.ComplexityAvg)
		}

		fmt.Fprintf(response, "\nPROJECT OVERVIEW:\n")
		fmt.Fprintf(response, "- Total symbols: %d\n", workspaceData.TotalSymbols)
		fmt.Fprintf(response, "- Total files: %d\n", workspaceData.TotalFiles)
		fmt.Fprintf(response, "- Dependency patterns: %d\n", len(workspaceData.DependencyPatterns))

		// Dependency patterns
		if len(workspaceData.DependencyPatterns) > 0 {
			fmt.Fprintf(response, "\nDEPENDENCY PATTERNS:\n")
			for i, pattern := range workspaceData.DependencyPatterns {
				if i >= 5 { // Limit to first 5 patterns
					fmt.Fprintf(response, "... and %d more patterns\n", len(workspaceData.DependencyPatterns)-5)
					break
				}
				circular := ""
				if pattern.IsCircular {
					circular = " (circular)"
				}
				fmt.Fprintf(response, "- %s → %s (%s, freq: %d, depth: %d)%s\n",
					pattern.Source, pattern.Target, pattern.Type, pattern.Frequency, pattern.Depth, circular)
			}
		}

		// Architectural health
		health := workspaceData.ArchitecturalHealth
		fmt.Fprintf(response, "\nARCHITECTURAL HEALTH:\n")
		fmt.Fprintf(response, "- Code Organization: %.1f%% (%s)\n", health.CodeOrganization.Score, health.CodeOrganization.Level)
		fmt.Fprintf(response, "- Naming Consistency: %.1f%% (%s)\n", health.NamingConsistency.Score, health.NamingConsistency.Level)
		fmt.Fprintf(response, "- Error Handling: %.1f%% (%s)\n", health.ErrorHandling.Score, health.ErrorHandling.Level)
		fmt.Fprintf(response, "- Test Coverage: %.1f%% (%s)\n", health.TestCoverage.Score, health.TestCoverage.Level)
		fmt.Fprintf(response, "- Documentation: %.1f%% (%s)\n", health.Documentation.Score, health.Documentation.Level)
		fmt.Fprintf(response, "- Overall Score: %.1f%% (%s)\n", health.OverallScore.Score, health.OverallScore.Level)

		// Suggestions
		if len(health.OverallScore.Suggestions) > 0 {
			fmt.Fprintf(response, "\nSUGGESTIONS:\n")
			for _, suggestion := range health.OverallScore.Suggestions {
				fmt.Fprintf(response, "- %s\n", suggestion)
			}
		}

		// Analysis metadata
		fmt.Fprintf(response, "\nANALYSIS METADATA:\n")
		fmt.Fprintf(response, "- Duration: %v\n", result.Metadata.Duration)
		fmt.Fprintf(response, "- Languages used: %v\n", result.Metadata.LanguagesUsed)
		if len(result.Metadata.Errors) > 0 {
			fmt.Fprintf(response, "- Errors: %d\n", len(result.Metadata.Errors))
			for _, err := range result.Metadata.Errors {
				languageInfo := "unknown"
				if err.Language != "" {
					languageInfo = string(err.Language)
				} else {
					// Try to find the language from the metadata
					for _, lang := range result.Metadata.LanguagesUsed {
						languageInfo = string(lang)
						break
					}
				}
				fmt.Fprintf(response, "  - [%s] %s\n", languageInfo, err.Message)
			}
		}
	}

	return mcp.NewToolResultText(response.String()), nil
}

// handleSymbolRelationships handles the 'symbol_relationships' analysis type
func handleSymbolRelationships(bridge interfaces.BridgeInterface, clients map[types.Language]types.LanguageClientInterface, query string, options map[string]interface{}, response *strings.Builder) (*mcp.CallToolResult, error) {
	response.WriteString("SYMBOL RELATIONSHIPS:\n")

	fmt.Fprintf(response, "Analyzing symbol: %s\n\n", query)

	// Create analysis engine with clients and language detector
	analyzer := analysis.NewProjectAnalyzer(clients,
		analysis.WithLanguageDetector(bridge.InferLanguage))

	// Create analysis request
	request := analysis.AnalysisRequest{
		Type:    analysis.SymbolRelationships,
		Target:  query,
		Scope:   "project",
		Depth:   "comprehensive",
		Options: options,
	}

	// Perform analysis
	result, err := analyzer.Analyze(request)
	if err != nil {
		fmt.Fprintf(response, "Analysis failed: %v\n", err)
		return mcp.NewToolResultText(response.String()), nil
	}

	// Format results
	if symbolData, ok := result.Data.(analysis.SymbolRelationshipsData); ok {
		fmt.Fprintf(response, "SYMBOL INFORMATION:\n")
		fmt.Fprintf(response, "- Name: %s\n", symbolData.Symbol.Name)
		fmt.Fprintf(response, "- Language: %s\n", symbolData.Language)
		fmt.Fprintf(response, "- Kind: %s\n", symbolKindToString(symbolData.Symbol.Kind))

		fmt.Fprintf(response, "\nRELATIONSHIPS:\n")
		fmt.Fprintf(response, "- References: %d\n", len(symbolData.References))
		fmt.Fprintf(response, "- Definitions: %d\n", len(symbolData.Definitions))
		fmt.Fprintf(response, "- Call hierarchy items: %d\n", len(symbolData.CallHierarchy))
		fmt.Fprintf(response, "- Incoming calls: %d\n", len(symbolData.IncomingCalls))
		fmt.Fprintf(response, "- Outgoing calls: %d\n", len(symbolData.OutgoingCalls))
		fmt.Fprintf(response, "- Implementations: %d\n", len(symbolData.Implementations))
		fmt.Fprintf(response, "- Type hierarchy: %d\n", len(symbolData.TypeHierarchy))

		// Show detailed call hierarchy if present
		if len(symbolData.IncomingCalls) > 0 || len(symbolData.OutgoingCalls) > 0 {
			fmt.Fprintf(response, "\nCALL HIERARCHY DETAILS:\n")

			if len(symbolData.IncomingCalls) > 0 {
				fmt.Fprintf(response, "- Incoming calls:\n")
				for i, call := range symbolData.IncomingCalls {
					if i >= 5 { // Limit to first 5 to avoid overwhelming output
						fmt.Fprintf(response, "  ... and %d more\n", len(symbolData.IncomingCalls)-5)
						break
					}
					// Show caller with location details
					fmt.Fprintf(response, "  - %s (from %s", call.From.Name, call.From.Uri)
					if len(call.FromRanges) > 0 {
						// Show the first call location (there could be multiple calls from the same function)
						firstRange := call.FromRanges[0]
						fmt.Fprintf(response, ":%d:%d", firstRange.Start.Line+1, firstRange.Start.Character+1) // Convert to 1-based for readability
						if len(call.FromRanges) > 1 {
							fmt.Fprintf(response, " +%d more", len(call.FromRanges)-1)
						}
					}
					fmt.Fprintf(response, ")\n")
				}
			}

			if len(symbolData.OutgoingCalls) > 0 {
				fmt.Fprintf(response, "- Outgoing calls:\n")
				for i, call := range symbolData.OutgoingCalls {
					if i >= 5 { // Limit to first 5 to avoid overwhelming output
						fmt.Fprintf(response, "  ... and %d more\n", len(symbolData.OutgoingCalls)-5)
						break
					}
					// Show callee with location details
					fmt.Fprintf(response, "  - %s (to %s", call.To.Name, call.To.Uri)
					if len(call.FromRanges) > 0 {
						// Show where in the current function this call is made
						firstRange := call.FromRanges[0]
						fmt.Fprintf(response, " called at line %d:%d", firstRange.Start.Line+1, firstRange.Start.Character+1) // Convert to 1-based for readability
						if len(call.FromRanges) > 1 {
							fmt.Fprintf(response, " +%d more", len(call.FromRanges)-1)
						}
					}
					fmt.Fprintf(response, ")\n")
				}
			}
		}

		// Usage patterns
		usage := symbolData.UsagePatterns
		fmt.Fprintf(response, "\nUSAGE PATTERNS:\n")
		fmt.Fprintf(response, "- Primary usage: %s\n", usage.PrimaryUsage)
		fmt.Fprintf(response, "- Secondary usage: %s\n", usage.SecondaryUsage)
		fmt.Fprintf(response, "- Usage frequency: %d\n", usage.UsageFrequency)

		// Caller patterns
		if len(usage.CallerPatterns) > 0 {
			fmt.Fprintf(response, "- Caller patterns:\n")
			for _, pattern := range usage.CallerPatterns {
				fmt.Fprintf(response, "  - %s: %d calls\n", pattern.CallerType, pattern.CallFrequency)
			}
		}

		// Related symbols
		if len(symbolData.RelatedSymbols) > 0 {
			fmt.Fprintf(response, "\nRELATED SYMBOLS:\n")
			for _, related := range symbolData.RelatedSymbols {
				fmt.Fprintf(response, "- %s (%s, strength: %.2f)\n",
					related.Symbol.Name, related.Relationship, related.Strength)
			}
		}

		// Impact analysis
		impact := symbolData.ImpactAnalysis
		fmt.Fprintf(response, "\nIMPACT ANALYSIS:\n")
		fmt.Fprintf(response, "- Files affected: %d\n", impact.FilesAffected)
		fmt.Fprintf(response, "- Critical paths: %d\n", len(impact.CriticalPaths))
		fmt.Fprintf(response, "- Dependencies: %d\n", len(impact.Dependencies))
		fmt.Fprintf(response, "- Refactoring complexity: %s\n", impact.RefactoringComplexity)

		// Breaking changes
		if len(impact.BreakingChanges) > 0 {
			fmt.Fprintf(response, "- Potential breaking changes:\n")
			for _, change := range impact.BreakingChanges {
				fmt.Fprintf(response, "  - [%s] %s: %s\n", change.Severity, change.Type, change.Description)
			}
		}

		// Analysis metadata
		fmt.Fprintf(response, "\nANALYSIS METADATA:\n")
		fmt.Fprintf(response, "- Duration: %v\n", result.Metadata.Duration)
		fmt.Fprintf(response, "- Languages used: %v\n", result.Metadata.LanguagesUsed)
		if len(result.Metadata.Errors) > 0 {
			fmt.Fprintf(response, "- Errors: %d\n", len(result.Metadata.Errors))
			for _, err := range result.Metadata.Errors {
				languageInfo := "unknown"
				if err.Language != "" {
					languageInfo = string(err.Language)
				} else {
					// Try to find the language from the metadata
					for _, lang := range result.Metadata.LanguagesUsed {
						languageInfo = string(lang)
						break
					}
				}
				fmt.Fprintf(response, "  - [%s] %s\n", languageInfo, err.Message)
			}
		}
	}

	return mcp.NewToolResultText(response.String()), nil
}

// calculateFileComplexityFromSymbols calculates complexity metrics from document symbols
func calculateFileComplexityFromSymbols(symbols []protocol.DocumentSymbol) ComplexityMetrics {
	metrics := ComplexityMetrics{
		TotalLines:    0,
		FunctionCount: 0,
		ClassCount:    0,
		VariableCount: 0,
	}

	for _, symbol := range symbols {
		switch symbol.Kind {
		case protocol.SymbolKindFunction, protocol.SymbolKindMethod:
			metrics.FunctionCount++
		case protocol.SymbolKindClass, protocol.SymbolKindInterface:
			metrics.ClassCount++
		case protocol.SymbolKindVariable, protocol.SymbolKindConstant:
			metrics.VariableCount++
		}

		// Estimate lines of code from symbol range
		metrics.TotalLines += int(symbol.Range.End.Line - symbol.Range.Start.Line + 1)
	}

	// Calculate complexity score
	metrics.ComplexityScore = float64(metrics.FunctionCount*2 + metrics.ClassCount*3 + metrics.VariableCount)

	// Categorize complexity level
	if metrics.ComplexityScore < 10 {
		metrics.ComplexityLevel = "low"
	} else if metrics.ComplexityScore < 50 {
		metrics.ComplexityLevel = "medium"
	} else {
		metrics.ComplexityLevel = "high"
	}

	return metrics
}
