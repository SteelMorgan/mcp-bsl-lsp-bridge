# Symbol Exploration Tool Guide

Guide for the `symbol_explore` tool - intelligent code symbol search and exploration with context-aware information gathering.

## Overview

The `symbol_explore` tool provides an intelligent way to search for and explore code symbols across your project with progressive detail levels and smart filtering capabilities.

## Features

- **Intelligent Search**: Smart symbol matching with fuzzy search capabilities
- **Context-Aware**: Filter results by file context for more targeted searches
- **Progressive Detail**: Automatic or manual detail level control
- **Session State**: Maintains search context for progressive exploration
- **Multi-Language**: Works across all supported programming languages

## Basic Usage

### Simple Symbol Search

```bash
● symbol_explore (MCP)(query: "getUserData")
```

Find any symbol named or containing "getUserData".

### File Context Filtering

```bash
● symbol_explore (MCP)(query: "getUserData", file_context: "auth")
```

Search for "getUserData" but only in files related to "auth" (matches filenames, directories, or path components).

### Detail Level Control

```bash
● symbol_explore (MCP)(query: "connectDB", detail_level: "full")
```

Get comprehensive details about "connectDB" symbols.

## Parameters

### Required Parameters
- `query`: Symbol name to search for

### Optional Parameters
- `file_context`: Fuzzy file filter (filename, directory, or path component)
- `detail_level`: Information depth - "auto", "basic", "full" (default: "auto")
- `workspace_scope`: Search scope - "project", "current_dir" (default: "project")
- `limit`: Maximum number of detailed results to show (default: 3)
- `offset`: Number of results to skip for detailed view (default: 0)

## Detail Levels

### Auto (default)
- Single match: Returns full details immediately
- Multiple matches: Returns summary with disambiguation options

### Basic
- Symbol name, type, and location
- Minimal context information

### Full
- Complete symbol information
- Documentation if available
- Usage context and relationships
- Code preview

## Behavior Patterns

### Single Match Found
When only one symbol matches your query, `symbol_explore` automatically provides detailed information:

```bash
● symbol_explore (MCP)(query: "NewProjectAnalyzer")
```

Returns immediate detailed information about the constructor function.

### Multiple Matches Found
When multiple symbols match, you get a summary with options for deeper exploration:

```bash
● symbol_explore (MCP)(query: "Handler")
```

Returns a list of all symbols containing "Handler" with basic information, allowing you to refine your search.

## File Context Examples

### Search in Authentication Module
```bash
● symbol_explore (MCP)(query: "validateUser", file_context: "auth")
```

Finds "validateUser" symbols but only in files with "auth" in their path.

### Search in Specific Directory
```bash
● symbol_explore (MCP)(query: "parseConfig", file_context: "config")
```

Limits search to files in config-related directories.

### Search by File Extension Context
```bash
● symbol_explore (MCP)(query: "testHelper", file_context: "test")
```

Finds symbols in test files or test directories.

## Advanced Usage

### Pagination Through Results
```bash
# Get first 5 results
● symbol_explore (MCP)(query: "Handler", limit: 5)

# Get next 5 results
● symbol_explore (MCP)(query: "Handler", limit: 5, offset: 5)
```

### Workspace Scope Control
```bash
# Search entire project (default)
● symbol_explore (MCP)(query: "utils", workspace_scope: "project")

# Search only current directory
● symbol_explore (MCP)(query: "utils", workspace_scope: "current_dir")
```

## Use Cases

### API Discovery
```bash
# Find all authentication-related functions
● symbol_explore (MCP)(query: "auth", file_context: "api")

# Discover configuration options
● symbol_explore (MCP)(query: "config", detail_level: "full")
```

### Code Navigation
```bash
# Find entry points
● symbol_explore (MCP)(query: "main")

# Locate initialization functions
● symbol_explore (MCP)(query: "init", file_context: "server")
```

### Refactoring Preparation
```bash
# Find all uses of deprecated function
● symbol_explore (MCP)(query: "oldFunction", detail_level: "full")

# Discover related functions
● symbol_explore (MCP)(query: "user", file_context: "model")
```

### Learning Codebase
```bash
# Explore core interfaces
● symbol_explore (MCP)(query: "interface", detail_level: "basic")

# Find example implementations
● symbol_explore (MCP)(query: "example", file_context: "test")
```

## Best Practices

### Effective Queries
- **Start broad, then narrow**: Begin with general terms, then add file context
- **Use partial matches**: "auth" finds "authenticate", "authorization", etc.
- **Leverage file context**: Combine symbol names with module context

### Progressive Exploration
1. Start with auto detail level to understand scope
2. Use basic level for quick overviews
3. Use full level when you need complete information

### Performance Tips
- Use file context to reduce search scope
- Limit results when exploring large codebases
- Use specific queries when you know what you're looking for

## Integration with Other Tools

### With Project Analysis
```bash
# First explore symbols
● symbol_explore (MCP)(query: "ProjectAnalyzer")

# Then analyze relationships
● project_analysis (MCP)(analysis_type: "symbol_relationships", query: "ProjectAnalyzer")
```

### With Hover Information
```bash
# Find symbol location
● symbol_explore (MCP)(query: "calculateTotal", detail_level: "full")

# Get detailed type information (use coordinates from symbol_explore result)
● hover (MCP)(uri: "file:///path/to/file.go", line: 42, character: 15)
```

## Multi-Language Examples

### Go
```bash
● symbol_explore (MCP)(query: "struct", file_context: "types")
```

### Python
```bash
● symbol_explore (MCP)(query: "class", file_context: "model")
```

### JavaScript/TypeScript
```bash
● symbol_explore (MCP)(query: "function", file_context: "util")
```

### Java
```bash
● symbol_explore (MCP)(query: "Service", file_context: "service")
```

The `symbol_explore` tool is designed to be your first stop when exploring unfamiliar codebases or when you need to quickly locate and understand symbols across your project.