# Contributing

Thanks for considering a contribution.

## Development setup

### Prerequisites

- Go toolchain (see `go.mod`)
- `make` (optional but recommended)

### Build

```bash
go build ./...
```

### Test

```bash
go test ./...
```

## Project structure (where to change what)

- **MCP tool definitions/handlers**: `mcpserver/tools/`
- **Tool exposure (what’s enabled by default)**: `mcpserver/tools.go`
- **Bridge logic (URI/path mapping, allowlist, didOpen)**: `bridge/`
- **LSP clients (stdio/tcp/websocket/session)**: `lsp/`
- **Session manager (persistent LSP session + file watcher)**: `cmd/lsp-session-manager/`

## Adding a new tool

1. Implement the tool in `mcpserver/tools/<tool>.go` using `mcp.NewTool(...)`.
2. Wire it into `mcpserver/tools.go` if it should be exposed by default.
3. Document it in:
   - `docs/tools/tools-reference.md`
   - `docs/tools/lsp-methods-map.md` (tool → LSP mapping)

## Code style

- Run `go fmt ./...` before committing.
- Keep tools safe by default (prefer preview-only, avoid executing commands).
- Never bypass path allowlisting. If a tool reads/writes files, it must go through `bridge.IsAllowedDirectory(...)`.

