# MCP Tools Reference

This document describes the MCP tools implemented in this repository.

Important:
- **Not every implemented tool is exposed by default.** The default exposed set is defined in `mcpserver/tools.go`.
- Some tools are intentionally hidden because they are too noisy for agent workflows, unstable with specific language servers, or not useful for LLM-driven navigation.

## Quick Reference

### Exposed by default (registered in `mcpserver/tools.go`)

- **Discovery & analysis**: `project_analysis`, `symbol_explore`
- **Navigation**: `hover`, `definition`, `selection_range`, `call_hierarchy`, `call_graph`
- **Refactoring & edits**: `code_actions`, `prepare_rename`, `rename`
- **Diagnostics**: `document_diagnostics`
- **Workspace / LSP plumbing**: `did_change_watched_files`, `lsp_status`
- **Utilities**: `get_range_content`

### Implemented but hidden by default

These exist in `mcpserver/tools/*.go` but are not registered by default:

- **Language detection**: `detect_project_languages`, `infer_language`
- **Connection management**: `lsp_connect`, `lsp_disconnect`
- **Formatting**: `format_document`, `range_formatting`
- **Additional LSP features**: `implementation`, `signature_help`, `semantic_tokens`, `folding_range`, `document_link`, `document_color`, `color_presentation`, `workspace_diagnostics`, `did_change_configuration`, `execute_command`
- **Bridge diagnostics**: `mcp_lsp_diagnostics`

## Tool → LSP mapping (high level)

If you want the exact LSP method mapping for every tool (including composite tools), see `docs/tools/lsp-methods-map.md`.

## Exposed tools (default)

### `project_analysis`
Multi-purpose code analysis with multiple analysis types for symbols, files, and workspace patterns.

**Common Usage:**
- Find symbols: `analysis_type="workspace_symbols"`, `query="calculateTotal"`
- Analyze files: `analysis_type="file_analysis"`, `query="src/auth.go"`
- Workspace overview: `analysis_type="workspace_analysis"`, `query="entire_project"`

**Key Parameters**: analysis_type (required), query (required), limit (default: 20), offset (default: 0)
**Output**: Structured analysis results with metadata and suggestions

### `symbol_explore`
Intelligent symbol search with contextual filtering and detailed code information.

**Common Usage:**
- Find symbols: `query="getUserData"`
- Filter by context: `query="validateUser"`, `file_context="auth"`
- Detailed view: `query="connectDB"`, `detail_level="full"`

**Key Parameters**: query (required), file_context (optional), detail_level (auto/basic/full)
**Output**: Symbol matches with documentation, implementation, and references

### `get_range_content`
Extract text content from specific file ranges with precise line/character positioning.

**Common Usage:**
- Extract function: `uri="file://path"`, `start_line=10`, `end_line=25`
- Get code block: Use coordinates from `project_analysis` definitions

**Key Parameters**: uri (required), start_line/start_character/end_line/end_character (required), strict (default: false)
**Output**: Exact text content from specified range

### `hover`
Get detailed symbol information including signatures, documentation, and type details.

**Common Usage:**
- Symbol info: `uri="file://path"`, `line=15`, `character=10`
- Type details: Position cursor on variable/function name

**Key Parameters**: uri (required), line/character (required, 0-based)
**Output**: Formatted documentation with code examples and pkg.go.dev links

### `definition`
Get definition location(s) for the symbol at a specific cursor position (LSP `textDocument/definition`).

**Common Usage:**
- Go to definition: `uri="file://path"`, `line=15`, `character=10`

**Key Parameters**: uri (required), line/character (required, 0-based), language (optional override)
**Output**: One or more target locations (file + range)

### `selection_range`
Get selection ranges for positions (LSP `textDocument/selectionRange`). Useful for expanding selection from expression → statement → block.

**Common Usage:**
- Single position: `uri="file://path"`, `line=10`, `character=5`
- Multiple positions: `uri="file://path"`, `positions_json="[{\"line\":10,\"character\":5}]"`

**Key Parameters**: uri (required), (positions_json) OR (line + character)
**Output**: JSON array of selection range trees

### `code_actions`
Get intelligent quick fixes, refactoring suggestions, and automated code improvements.

**Common Usage:**
- Fix errors: Position at error location for import fixes, syntax corrections
- Refactor: Position at symbol for extract method, implement interface options

**Key Parameters**: uri (required), line/character (required)
**Output**: Available actions with descriptions and edit previews

### `prepare_rename`
Check whether rename is valid at a position and return the rename range (LSP `textDocument/prepareRename`).

### `rename`
Rename symbols across entire codebase with cross-file precision. Always preview first.

**Common Usage:**
- Preview: `uri="file://path"`, `line=10`, `character=5`, `new_name="newFunc"`, `apply="false"`
- Apply: Same parameters with `apply="true"`

**Key Parameters**: uri (required), line/character (required), new_name (required), apply (default: false)
**Output**: All affected files with exact change locations

### `call_hierarchy`
Show call hierarchy (callers and callees) for a symbol.

### `call_graph`
Build a full call graph by recursively traversing LSP call hierarchy (incoming + outgoing).
This is a **composite** tool (it calls multiple LSP methods repeatedly) and is optimized for BSL workflows:
- Entry-point detection (common event handler names)
- Cycle detection
- Depth/node limits and timeout

### `document_diagnostics`
Get diagnostics for a specific file using LSP 3.17+ `textDocument/diagnostic`.

### `did_change_watched_files`
Notify the language server about external file changes using `workspace/didChangeWatchedFiles`.

### `lsp_status`
Show current bridge-side LSP connection status and server progress (`$/progress`), plus indexing progress when running in session-manager mode.

## Common Workflows

**Explore a codebase**: `project_analysis` → `symbol_explore` → `definition` → `get_range_content`  
**Understand flow**: `call_hierarchy` (local) → `call_graph` (full traversal)  
**Fix issues**: `document_diagnostics` → `code_actions` → `rename` (preview) → `rename` (apply)  
**New/changed files**: run `did_change_watched_files` (or enable session-manager file watcher) before `call_graph`

## Safety Features

For tools that modify code (`format_document`, `rename`), the bridge provides crucial safety mechanisms:

- **Preview Mode**: Shows exactly what changes will be made across all affected files without modifying them
- **Apply Mode**: Once reviewed and approved, applies the changes to your codebase

This dual-mode operation ensures full control and visibility over automated code modifications.
