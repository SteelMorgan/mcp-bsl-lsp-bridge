# MCP Tools Reference

Complete reference for all MCP tools provided by the LSP bridge.

## Core Analysis Tools

### `project_analysis`
Multi-purpose code analysis tool with 9 different analysis types.

**Quick Examples:**
- File analysis: `analysis_type="file_analysis"`, `query="path/to/file.go"`
- Find symbols: `analysis_type="workspace_symbols"`, `query="calculateTotal"`
- Pattern analysis: `analysis_type="pattern_analysis"`, `query="error_handling"`
- Workspace overview: `analysis_type="workspace_analysis"`, `query="entire_project"`

**📖 See [project-analysis-guide.md](project-analysis-guide.md) for complete documentation with examples and sample outputs.**

### `symbol_explore`
Intelligent code symbol search with context-aware filtering and progressive detail levels.

**Quick Examples:**
- Basic search: `query="getUserData"`
- Context filtering: `query="validateUser"`, `file_context="auth"`
- Full details: `query="connectDB"`, `detail_level="full"`

**📖 See [symbol-exploration-guide.md](symbol-exploration-guide.md) for complete usage guide.**

### `get_range_content`
Get text content from file range. Efficient for specific code blocks. Range parameters should be precise, typically from `project_analysis` (`definitions` or `document_symbols` modes).

### `analyze_code`
Analyze code for completion suggestions and insights.

## Language Detection Tools

### `detect_project_languages`
Detect all programming languages used in a project by examining root markers and file extensions.

### `infer_language`
Infer the programming language for a file.

## LSP Connection Management

### `lsp_connect`
Connect to a language server for a specific language.

### `lsp_disconnect`
Disconnect all active language server clients.

## Code Intelligence Tools

### `hover`
Get detailed symbol information (signatures, documentation, types).

### `signature_help`
Get function parameter information at call sites. Use when positioned inside function calls (between parentheses) to see parameter details and overloads.

### `semantic_tokens`
Get semantic tokens for a specific range of a file.

### `workspace_diagnostics`
Get comprehensive diagnostics for entire workspace.

## Code Improvement & Refactoring Tools

### `code_actions`
Get intelligent code actions including quick fixes, refactoring suggestions, and automated improvements for a code range. Returns language server suggested actions like import fixes, error corrections, extract method, add missing imports, implement interfaces, and other context-aware improvements. Use at error locations for fixes or at any code location for refactoring suggestions.

### `format_document`
**ACTIONABLE**: Format a document according to language conventions with dual-mode operation.

**PREVIEW MODE** (`apply='false'`, default): Shows detailed formatting changes without modifying files - displays line-by-line changes, whitespace adjustments, and content modifications.

**APPLY MODE** (`apply='true'`): Actually applies all formatting changes to the file.

Supports customizable indentation and language-specific formatting rules. Always preview first for safety.

### `rename`
**ACTIONABLE**: Rename a symbol across the entire codebase with LSP precision and cross-file support.

**PREVIEW MODE** (`apply='false'`, default): Shows all files and exact locations that would be modified, with line numbers and replacement text.

**APPLY MODE** (`apply='true'`): Actually performs the rename across all affected files in the codebase.

Requires precise coordinates from definitions or hover for accurate targeting. Supports functions, variables, types, and other symbols. Always preview first to verify scope.

## Advanced Navigation Tools

### `implementation`
Find implementations of a symbol (interfaces, abstract methods).

### `call_hierarchy`
Show call hierarchy (callers and callees) for a symbol.

## System Tools

### `mcp_lsp_diagnostics`
Provides diagnostic information about the MCP-LSP bridge, including registered language servers, configuration details, connected servers, and detected project languages.

## Safety Features

For tools that modify code (`format_document`, `rename`), the bridge provides crucial safety mechanisms:

- **Preview Mode**: Shows exactly what changes will be made across all affected files without modifying them
- **Apply Mode**: Once reviewed and approved, applies the changes to your codebase

This dual-mode operation ensures full control and visibility over automated code modifications.

## Tool Categories Summary

**Analysis & Discovery**: `project_analysis`, `symbol_explore`, `get_range_content`, `analyze_code`

**Language Detection**: `detect_project_languages`, `infer_language`

**Connection Management**: `lsp_connect`, `lsp_disconnect`

**Code Intelligence**: `hover`, `signature_help`, `semantic_tokens`, `workspace_diagnostics`

**Safe Refactoring**: `code_actions`, `format_document`, `rename` (with preview/apply modes)

**Navigation**: `implementation`, `call_hierarchy`

**System**: `mcp_lsp_diagnostics`