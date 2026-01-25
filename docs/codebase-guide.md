# Codebase Guide

Practical overview of the MCP-LSP Bridge architecture and code layout.

## Project in one sentence

This is a Go MCP server that exposes Language Server Protocol capabilities as MCP tools by running (or connecting to) one or more LSP clients.

## High-level architecture

**Data flow:** MCP client → this MCP server → `bridge` → `lsp` clients → language servers.

Key ideas:
- A single bridge instance multiplexes requests to one or more language servers.
- Paths/URIs are normalized and (optionally) mapped between host and container filesystems.
- Tool exposure is curated: many LSP methods are implemented, but only a subset is exposed by default.

## Runtime modes (how we talk to an LSP server)

The bridge can create LSP clients in different modes (configured via `lsp_config.json`):

- **stdio**: spawn the language server process and speak JSON-RPC/LSP over stdin/stdout.
- **tcp**: connect to an LSP server behind `cmd/lsp-proxy` (JSON-RPC over TCP using VSCode/LSP framing).
- **websocket**: connect to a WebSocket LSP server.
- **session**: connect to `cmd/lsp-session-manager` (persistent LSP session + indexing tracking + optional file watcher).

## Tool surface

Implemented tools live in `mcpserver/tools/`. The default exposed set is registered in `mcpserver/tools.go`.

Docs:
- `docs/tools/tools-reference.md` (what tools exist and which are exposed by default)
- `docs/tools/lsp-methods-map.md` (exact tool → LSP method mapping)

## Directory layout (what matters)

### Entry points

- `main.go`: CLI parsing, config loading, MCP server setup.
- `cmd/lsp-proxy/`: optional TCP proxy for LSP.
- `cmd/lsp-session-manager/`: persistent LSP session daemon (critical for large BSL workspaces).

### MCP server layer

- `mcpserver/`: MCP server setup + tool registration (`mcpserver/tools.go`).
- `mcpserver/tools/`: tool implementations (each tool is defined as `mcp.NewTool(...)` + handler).

### Bridge layer (glue + policy)

- `bridge/bridge.go`: concrete `MCPLSPBridge` implementation, path normalization/mapping, document open logic, LSP method wrappers.
- `bridge/auto_connect.go`: background auto-connect logic (so agents do not need to call `lsp_connect` explicitly).
- `bridge/warmup.go`: warmup/indexing gating (used by readiness checks).

### LSP client layer

- `lsp/client.go`: stdio client + request/notification plumbing.
- `lsp/tcp_client.go`: TCP client.
- `lsp/websocket_client.go`: WebSocket client.
- `lsp/session_adapter.go` + `lsp/session_client.go`: client for `cmd/lsp-session-manager`.
- `lsp/methods.go`: typed wrappers for common LSP methods.
- `lsp/progress.go`: `$\/progress` tracking (used by `lsp_status`).

### Shared utilities

- `security/`: path allowlisting, safe argument checks.
- `utils/`: URI normalization, docker path mapping, misc helpers.
- `types/` + `interfaces/`: shared types and interfaces (enables mocking and alternative bridge implementations).
- `analysis/`: filesystem-based analysis engine used by `project_analysis` for non-LSP queries.

## File watcher (why it exists)

Some language servers (notably for BSL) require explicit `workspace/didChangeWatchedFiles` notifications to notice new/changed files that were not opened via `textDocument/didOpen`.

We support this in two ways:
- **Manual tool**: `did_change_watched_files` (MCP tool) sends `workspace/didChangeWatchedFiles` to the LSP.
- **Automatic watcher (session mode)**: `cmd/lsp-session-manager` can watch the workspace and send `workspace/didChangeWatchedFiles` automatically:
  - `fsnotify` mode (native file events, typically Linux)
  - `polling` mode (Docker-on-Windows friendly)
  - controlled by env vars like `FILE_WATCHER_MODE`, `FILE_WATCHER_INTERVAL`, `FILE_WATCHER_WORKERS`

## Suggested reading order (for new contributors)

1. `mcpserver/tools.go` (what is exposed and why)
2. `mcpserver/tools/call_graph.go` and `mcpserver/tools/did_change_watched_files.go` (the non-trivial pieces)
3. `bridge/bridge.go` (URI/path mapping, ensure-didOpen)
4. `lsp/client.go` + `lsp/methods.go` (how LSP calls are executed)
5. `cmd/lsp-session-manager/main.go` (persistent session + indexing + file watcher)

