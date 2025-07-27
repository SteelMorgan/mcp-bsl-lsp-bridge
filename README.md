# MCP LSP Bridge

Brings Language Server Protocol capabilities to MCP-compatible agents like Claude Code. Analyze, navigate, and refactor code across 20+ programming languages.

## Status

**Under active development** - Core functionality works today, but expect rapid improvements and interface changes.

## Roadmap

- Improve performance for smaller models, Qwen3-4B, quantized models
- Improve detection and usage for LSP tools
- Add functionality to reduce the number of tools we expose (rename, formatting may be low priority and optional)
- Make onboarding easier with default configuration options and separate configuration options that can be scripted (maybe like Lua to allow scripting the LSP configuration)

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
3. Configure LSP servers with lsp_config.json in your configuration location: [default-directory-locations](/docs/configuration.md#default-directory-locations)
  - or with `mcp-lsp-bridge --config /path/to/my-config.json` inside your MCP client configuration.
4. **Use**: Access LSP tools through your MCP client

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

- [Codebase Guide](docs/codebase-guide.md) - Architecture and structure overview 🆕
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
