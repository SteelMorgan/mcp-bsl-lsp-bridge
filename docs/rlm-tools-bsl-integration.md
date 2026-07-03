# rlm-tools-bsl integration

This repository integrates upstream `Dach-Coin/rlm-tools-bsl` as a runtime dependency, not as a source fork.

## Borrowing Boundary

Pinned upstream package:

- Docker build arg: `RLM_TOOLS_BSL_VERSION`
- Default version: `1.26.0`
- Installed into `/opt/rlm-tools-bsl` Python venv
- Runtime binaries used:
  - `rlm-tools-bsl`
  - `rlm-bsl-index`

Local code owns only:

- container installation and s6 supervision;
- Go MCP proxy tool declarations;
- JSON-RPC forwarding to the local upstream streamable HTTP MCP endpoint;
- file watcher integration that schedules `rlm-bsl-index index build/update`;
- operational defaults in `docker-compose*.yml`.

Local code must not copy or modify upstream Python helpers, parsers, sandbox code, index schema, or business recipes. When upstream changes those internals, update `RLM_TOOLS_BSL_VERSION` and test behavior through the public tools.

## Runtime Processes

The container starts two long-running s6 services:

- `bsl-ls`: existing `lsp-session-manager` daemon that owns BSL LS.
- `rlm-tools-bsl`: upstream MCP server on `RLM_MCP_URL` (`http://127.0.0.1:9000/mcp` by default).

The external MCP entrypoint remains `mcp-lsp-bridge` over stdio, typically via `docker exec -i 1c-dev-mcp-bsl-rlm mcp-lsp-bridge`.

## Exposed RLM Tools

The Go MCP server exposes the upstream tool names as thin proxies:

- `rlm_start`
- `rlm_execute`
- `rlm_end`
- `rlm_help`
- `rlm_projects`
- `rlm_index`

Proxy behavior:

- sends `tools/call` JSON-RPC to the local upstream MCP endpoint;
- accepts upstream SSE responses and returns text content to the caller;
- does not reimplement upstream helper logic.

If upstream adds a new MCP tool, add only a matching Go proxy declaration and forwarder argument mapping.

## Indexing and Updates

`lsp-session-manager` owns the shared file watcher. It now watches a broader source set for RLM while still sending only `.bsl` and `.os` changes to BSL LS.

BSL LS receives:

- `.bsl`
- `.os`

RLM update scheduling sees:

- `.bsl`
- `.os`
- `.xml`
- `.mdo`
- `.json`
- `.rights`
- `.form`
- `.query`
- `.txt`
- `.md`
- `.properties`

On startup:

- single-project mode schedules RLM build/update for the detected configuration root;
- multi-project mode schedules RLM build/update when a project is warmed by BSL LS.

On file changes:

- watcher groups changes by detected project root;
- RLM updates are debounced by `RLM_INDEX_DEBOUNCE` (`10s` default);
- repeated changes while an index operation is running are coalesced into one follow-up update.

Relevant env:

- `RLM_INDEX_ENABLED=1`
- `RLM_INDEX_BUILD_ON_START=1`
- `RLM_INDEX_UPDATE_ON_CHANGE=1`
- `RLM_INDEX_BIN=rlm-bsl-index`
- `RLM_INDEX_DEBOUNCE=10s`
- `RLM_INDEX_TIMEOUT=30m`

## Capability Priority

Agent trigger rule:

Use RLM when the agent needs to discover project-wide 1C/BSL context to complete
the task, even if the user did not explicitly phrase the request as "search" or
"find". In Claude Code terms, prefer `rlm_start` + `rlm_execute` before broad
`Grep`, `Glob`, repeated `Read`, shell `rg`/`grep`/`find`, or manual recursive
file reads over 1C source.

Use BSL LS first for cursor-precise language intelligence:

- hover;
- definition;
- signature help;
- completion;
- rename/prepare rename;
- code actions;
- semantic tokens;
- diagnostics and complexity.

Use RLM first for broad static discovery:

- replacing raw grep across 1C source;
- finding modules, objects, methods, procedures;
- full-text search;
- metadata/XML/form/right/query discovery;
- references/usages of metadata objects;
- call/search workflows that benefit from indexed project-wide context.

Overlap is intentional but not a hard conflict:

- BSL LS is authoritative for LSP semantics at a file position.
- RLM is authoritative for indexed static search across the full 1C configuration.
- If both can answer, prefer the one matching the task shape: cursor operation -> BSL LS, broad project search -> RLM.

## Updating Upstream

1. Change `RLM_TOOLS_BSL_VERSION`.
2. Rebuild the image.
3. Verify `rlm-tools-bsl /health`.
4. Verify proxied MCP calls: `rlm_help`, `rlm_projects(action=list)`, `rlm_index(action=info, path=...)`.
5. If upstream tool signatures changed, update only `mcpserver/tools/rlm_proxy.go` schemas and argument forwarding.
6. Do not vendor upstream Python modules unless there is an explicit decision to maintain a fork.
