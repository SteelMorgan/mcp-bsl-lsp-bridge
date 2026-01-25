# Tool → LSP Methods Map

This document describes which LSP requests/notifications each MCP tool triggers.

Notes:
- **Some tools are composites** (they call multiple LSP methods, sometimes repeatedly).
- **Some tools do not call LSP at all** (they operate on the filesystem or bridge internals).
- The default exposed tool set is defined in `mcpserver/tools.go`.

## Exposed by default

| MCP tool | LSP method(s) | Notes |
|---|---|---|
| `project_analysis` | Depends on `analysis_type` | Composite “Swiss army knife” tool. See breakdown below. |
| `symbol_explore` | `workspace/symbol`, `textDocument/hover`, `textDocument/references`, `textDocument/documentSymbol`, `textDocument/semanticTokens/range` | Also uses filesystem for language detection and code extraction. |
| `hover` | `textDocument/hover` | Uses URI normalization + ensure `didOpen` when needed. |
| `definition` | `textDocument/definition` | Supports optional `language` override; uses URI normalization for Docker/session mode. |
| `selection_range` | `textDocument/selectionRange` | Accepts one position or `positions_json` array. |
| `call_hierarchy` | `textDocument/prepareCallHierarchy`, `callHierarchy/incomingCalls`, `callHierarchy/outgoingCalls` | `direction` controls which callHierarchy method(s) are called. |
| `call_graph` | `textDocument/prepareCallHierarchy`, `callHierarchy/incomingCalls`, `callHierarchy/outgoingCalls` | Composite: recursively expands callers/callees in parallel, with depth/node limits, cycle markers, BSL entry-point heuristics. |
| `code_actions` | `textDocument/codeAction` | Marked destructive because actions can imply edits; this tool returns actions, it does not apply them. |
| `prepare_rename` | `textDocument/prepareRename` | Used to validate rename + get exact range. |
| `rename` | `textDocument/rename` | Bridge applies returned `WorkspaceEdit` to files when `apply=true`. |
| `document_diagnostics` | `textDocument/diagnostic` | Requires LSP 3.17+ diagnostics support. |
| `did_change_watched_files` | `workspace/didChangeWatchedFiles` (notification) | Used when files change outside `didOpen`/`didChange` flow. Critical for accurate call hierarchy/graph on some servers. |
| `lsp_status` | (none) | Bridge/internal status: client connectivity, `$\/progress` snapshot, and (in session mode) indexing progress. |
| `get_range_content` | (none) | Pure filesystem read with path validation + optional host↔container mapping. |

### `project_analysis` breakdown

`project_analysis` routes to different backends depending on `analysis_type`:

| analysis_type | LSP method(s) | Notes |
|---|---|---|
| `workspace_symbols` | `workspace/symbol` | Primary symbol search. |
| `document_symbols` | `textDocument/documentSymbol` | Requires a file path/URI in `query`. |
| `references` | `workspace/symbol` + `textDocument/references` | Finds a candidate symbol, then resolves its usage sites. |
| `definitions` | `workspace/symbol` + `textDocument/definition` | Finds a candidate symbol, then resolves its definition location(s). |
| `text_search` | (none) | Filesystem text scan fallback. |
| `file_analysis` | (none) | Filesystem read + heuristics/metrics. |
| `workspace_analysis` | (none) | Filesystem scan + summarization. |
| `pattern_analysis` | (none) | Filesystem scan focused on patterns. |
| `symbol_relationships` | Mixed | Typically combines symbol search + defs/refs + range reads; implementation may evolve. |

## Implemented but hidden by default

These tools exist in `mcpserver/tools/*.go`, but are not registered by default.

| MCP tool | LSP method(s) | Notes |
|---|---|---|
| `detect_project_languages` | (none) | Filesystem-based language detection. |
| `infer_language` | (none) | Extension-based language inference. |
| `lsp_connect` | (none) | Bridge-side connection management; not an LSP method itself. |
| `lsp_disconnect` | (none) | Bridge-side connection management; not an LSP method itself. |
| `format_document` | `textDocument/formatting` | May be disabled for some servers (e.g. hangs on large BSL modules). |
| `range_formatting` | `textDocument/rangeFormatting` | Range formatting. |
| `implementation` | `textDocument/implementation` | Some servers do not support this (notably BSL LS). |
| `signature_help` | `textDocument/signatureHelp` | Some servers do not support this (notably BSL LS). |
| `semantic_tokens` | `textDocument/semanticTokens/range` (and/or full) | Used internally by `symbol_explore` when available. |
| `folding_range` | `textDocument/foldingRange` | UI-oriented. |
| `document_link` | `textDocument/documentLink` | UI-oriented. |
| `document_color` | `textDocument/documentColor` | UI-oriented. |
| `color_presentation` | `textDocument/colorPresentation` | UI-oriented. |
| `workspace_diagnostics` | `workspace/diagnostic` | Heavy/noisy on large workspaces. |
| `did_change_configuration` | `workspace/didChangeConfiguration` (notification) | Editor-plumbing. |
| `execute_command` | `workspace/executeCommand` | Potentially dangerous; disabled by default. |
| `mcp_lsp_diagnostics` | (none) | Bridge diagnostics (configuration, connected clients, etc.). |

## Session manager note (file watcher)

When running in **session mode** (see `cmd/lsp-session-manager`), the session manager can automatically send:
- `workspace/didChangeWatchedFiles` notifications based on filesystem events (fsnotify) or polling (for Docker-on-Windows).

In that setup, the `did_change_watched_files` tool is typically only needed when file watching is disabled.

