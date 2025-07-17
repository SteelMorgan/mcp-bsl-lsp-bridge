# MCP LSP Bridge

Brings Language Server Protocol capabilities to MCP-compatible agents like Claude Code. Analyze, navigate, and refactor code across 20+ programming languages.

## Status

**Under active development** - Core functionality works today, but expect rapid improvements and interface changes.

## Quick Start

1. **Install**: Download from releases or build with `go build`
2. **Configure**: Add to your MCP client (like Claude Code):
   ```json
   {
     "mcpServers": {
       "lsp": {
         "type": "stdio",
         "command": "mcp-lsp-bridge",
         "args": [],
         "env": {}
       }
     }
   }
   ```
3. **Use**: Access LSP tools through your MCP client

See [docs/configuration.md](docs/configuration.md) for detailed setup.

## What It Does

Provides MCP-compatible agents with LSP capabilities:

**Code Intelligence**: Get documentation, find definitions, trace references, identify errors

**Safe Refactoring**: Rename symbols, format code, apply quick fixes (with preview mode)

**Project Analysis**: Search across codebases, understand project structure, detect languages

Supports 20+ languages including Go, Python, TypeScript, Rust, Java, C#, C++.

## Configuration

Requires an `lsp_config.json` file to define language servers.

Basic example:

```bash
# Build and run
go build -o mcp-lsp-bridge
./mcp-lsp-bridge --config lsp_config.json
```

See [docs/configuration.md](docs/configuration.md) for complete setup instructions.

## Available Tools

16 MCP tools for code analysis and manipulation:

- **Analysis**: Symbol search, project exploration, diagnostics
- **Navigation**: Find definitions, references, implementations
- **Refactoring**: Rename symbols, format code, apply fixes
- **Intelligence**: Hover info, signatures, semantic tokens

**📚 Documentation:**

- [Tools Reference](docs/tools/tools-reference.md) - Complete tool list
- [Analysis Overview](docs/analysis-overview.md) - Quick start guide
- [Project Analysis Guide](docs/tools/project-analysis-guide.md) - Detailed analysis tool guide
- [Symbol Exploration Guide](docs/tools/symbol-exploration-guide.md) - Smart symbol search guide

## Docker

Base image available (LSP servers not included):

```bash
docker pull ghcr.io/rockerboo/mcp-lsp-bridge:latest
```

Extend the image to add your needed LSP servers. See [docs/configuration.md](docs/configuration.md) for examples.
