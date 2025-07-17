# Analysis Tools Overview

Quick overview of the main analysis and exploration tools in MCP-LSP Bridge.

## Primary Analysis Tools

### 🔍 `project_analysis` - Comprehensive Code Analysis
The most powerful analysis tool with 9 different analysis types:

- **File Analysis**: Complexity metrics, code quality, recommendations
- **Pattern Analysis**: Error handling, naming conventions, architecture patterns  
- **Workspace Analysis**: Project-wide overview and health metrics
- **Symbol Analysis**: References, definitions, relationships
- **Text Search**: Find patterns across codebase

**📖 [Complete Guide](tools/project-analysis-guide.md)**

### 🎯 `symbol_explore` - Intelligent Symbol Discovery
Smart symbol search with context-aware filtering:

- **Progressive Detail**: Auto-adjust detail based on results
- **Context Filtering**: Search within specific modules or files
- **Multi-Language**: Works across all supported languages

**📖 [Complete Guide](tools/symbol-exploration-guide.md)**

## Quick Reference

### Most Common Use Cases

**Understanding a file:**
```bash
● project_analysis (MCP)(analysis_type: "file_analysis", query: "path/to/file.go")
```

**Finding a symbol:**
```bash
● symbol_explore (MCP)(query: "functionName", file_context: "module")
```

**Project health check:**
```bash
● project_analysis (MCP)(analysis_type: "workspace_analysis", query: "entire_project")
```

**Code pattern analysis:**
```bash
● project_analysis (MCP)(analysis_type: "pattern_analysis", query: "error_handling")
```

**Impact analysis before refactoring:**
```bash
● project_analysis (MCP)(analysis_type: "symbol_relationships", query: "OldFunction")
```

## Tool Selection Guide

| Need | Tool | Analysis Type |
|------|------|---------------|
| Understand file complexity | `project_analysis` | `file_analysis` |
| Find symbol location | `symbol_explore` | - |
| Check code patterns | `project_analysis` | `pattern_analysis` |
| Project overview | `project_analysis` | `workspace_analysis` |
| Impact assessment | `project_analysis` | `symbol_relationships` |
| Search codebase | `project_analysis` | `text_search` |
| Find symbol usage | `project_analysis` | `references` |

## Related Tools

- **`hover`**: Get detailed symbol information at specific coordinates
- **`code_actions`**: Get fixes and refactoring suggestions
- **`format_document`**: Format code with preview/apply modes
- **`rename`**: Safely rename symbols across codebase

See [tools-reference.md](tools/tools-reference.md) for complete tool list.