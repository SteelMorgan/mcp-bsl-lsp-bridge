# Project Analysis Tool Guide

Comprehensive guide for the `project_analysis` tool - the most powerful analysis tool in the MCP-LSP Bridge.

## Overview

The `project_analysis` tool provides 9 different analysis types for deep codebase understanding, from individual file analysis to workspace-wide architectural assessment.

## Analysis Types

### 1. File Analysis (`file_analysis`)

Analyzes individual files for complexity metrics, structure, and code quality.

**Usage:**
```bash
● project_analysis (MCP)(analysis_type: "file_analysis", query: "path/to/file.go")
```

**Features:**
- Complexity metrics (lines, functions, classes, variables)
- Import/export analysis
- Code quality metrics
- Actionable recommendations
- Language detection

**Example:**
```bash
● project_analysis (MCP)(analysis_type: "file_analysis", query: "mcpserver/tools/project_analysis.go")
```

**Sample Output:**
```
=== FILE ANALYSIS ===
Analyzing file: file:///home/rockerboo/code/mcp-lsp-bridge/mcpserver/tools/project_analysis.go

Language: go
Symbols found: 16

COMPLEXITY METRICS:
  Total Lines: 968
  Functions: 15
  Classes: 0
  Variables: 0
  Complexity Score: 0.94
  Complexity Level: very_low

IMPORT/EXPORT ANALYSIS:
  Imports: 0
  Exports: 0
  External Dependencies: 0
  Internal Dependencies: 0
  Circular Dependencies: 0
  Unused Imports: 0

RECOMMENDATIONS:
  - [medium] test: Add more unit tests to improve code reliability (effort: medium)
  - [low] document: Add more comments and documentation to improve code understanding (effort: low)
```

### 2. Pattern Analysis (`pattern_analysis`)

Detects and analyzes code patterns and consistency across the project.

**Usage:**
```bash
● project_analysis (MCP)(analysis_type: "pattern_analysis", query: "pattern_type")
```

**Supported Pattern Types:**
- `error_handling` - Analyzes error handling patterns
- `naming_conventions` - Checks naming consistency
- `architecture_patterns` - Detects architectural patterns

**Example:**
```bash
● project_analysis (MCP)(analysis_type: "pattern_analysis", query: "error_handling")
```

**Sample Output:**
```
=== PATTERN ANALYSIS ===
Pattern Type: error_handling

Scope: project
Consistency Score: 80.0%

PATTERN INSTANCES FOUND:
1. explicit_error_return (confidence: 70.0%, quality: good)
2. error_wrapping (confidence: 70.0%, quality: good)
3. centralized_error_handle (confidence: 70.0%, quality: good)

TREND ANALYSIS:
  Direction: stable (confidence: 70.0%)
  Contributing factors:
    - increasing code quality
    - consistent team practices
```

### 3. Workspace Analysis (`workspace_analysis`)

Comprehensive analysis of the entire workspace including language distribution, complexity metrics, and architectural health.

**Usage:**
```bash
● project_analysis (MCP)(analysis_type: "workspace_analysis", query: "entire_project")
```

**Features:**
- Language distribution analysis
- Overall architectural health metrics
- Project-wide complexity assessment
- Dependency analysis

**Sample Output:**
```
=== WORKSPACE ANALYSIS ===
Analyzing workspace for: entire_project

LANGUAGE DISTRIBUTION:
• python: 0 files (NaN%), 0 symbols, avg complexity: 0.00
• go: 0 files (NaN%), 0 symbols, avg complexity: 0.00

ARCHITECTURAL HEALTH:
• Code Organization: 75.0% (good)
• Naming Consistency: 90.0% (excellent)
• Error Handling: 85.0% (good)
• Test Coverage: 80.0% (good)
• Documentation: 70.0% (moderate)
• Overall Score: 80.0% (good)
```

### 4. Symbol Relationships (`symbol_relationships`)

Analyze symbol dependencies, usage patterns, and impact analysis.

**Usage:**
```bash
● project_analysis (MCP)(analysis_type: "symbol_relationships", query: "symbol_name")
```

**Features:**
- Reference and definition counts
- Usage pattern analysis
- Impact assessment for refactoring
- Related symbol discovery

**Example:**
```bash
● project_analysis (MCP)(analysis_type: "symbol_relationships", query: "ProjectAnalyzer")
```

**Sample Output:**
```
=== SYMBOL RELATIONSHIPS ===
Analyzing symbol: ProjectAnalyzer

SYMBOL INFORMATION:
• Name: ProjectAnalyzer
• Language: go
• Kind: struct

RELATIONSHIPS:
• References: 72
• Definitions: 1
• Call hierarchy items: 0
• Implementations: 0

USAGE PATTERNS:
• Primary usage: general
• Usage frequency: 72
• Caller patterns:
  - generic: 72 calls

IMPACT ANALYSIS:
• Files affected: 1
• Critical paths: 0
• Dependencies: 1
• Refactoring complexity: low
```

### 5. Workspace Symbols (`workspace_symbols`)

Find and analyze symbols across the entire project.

**Usage:**
```bash
● project_analysis (MCP)(analysis_type: "workspace_symbols", query: "symbol_name")
```

**Features:**
- Multi-language symbol search
- Precise location coordinates
- Symbol type identification
- Hover coordinate recommendations

**Example:**
```bash
● project_analysis (MCP)(analysis_type: "workspace_symbols", query: "ProjectAnalyzer")
```

### 6. Document Symbols (`document_symbols`)

List all symbols within a specific file.

**Usage:**
```bash
● project_analysis (MCP)(analysis_type: "document_symbols", query: "file_path")
```

**Example:**
```bash
● project_analysis (MCP)(analysis_type: "document_symbols", query: "analysis/engine.go")
```

### 7. References (`references`)

Find all references to a specific symbol.

**Usage:**
```bash
● project_analysis (MCP)(analysis_type: "references", query: "symbol_name")
```

**Example:**
```bash
● project_analysis (MCP)(analysis_type: "references", query: "ProjectAnalyzer")
```

### 8. Definitions (`definitions`)

Find the definition(s) of a specific symbol.

**Usage:**
```bash
● project_analysis (MCP)(analysis_type: "definitions", query: "symbol_name")
```

**Example:**
```bash
● project_analysis (MCP)(analysis_type: "definitions", query: "NewProjectAnalyzer")
```

### 9. Text Search (`text_search`)

Search for text patterns across the project.

**Usage:**
```bash
● project_analysis (MCP)(analysis_type: "text_search", query: "search_term")
```

**Examples:**
```bash
● project_analysis (MCP)(analysis_type: "text_search", query: "TODO")
● project_analysis (MCP)(analysis_type: "text_search", query: "error")
```

## Parameters

### Required Parameters
- `analysis_type`: The type of analysis to perform
- `query`: The target for analysis (symbol name, file path, or pattern type)

### Optional Parameters
- `workspace_uri`: Project root URI (auto-detected if not provided)
- `offset`: Skip N results (default: 0)
- `limit`: Max results (default: 20, max: 100)

### Example with Parameters
```bash
● project_analysis (MCP)(analysis_type: "workspace_symbols", query: "Handler", offset: 10, limit: 5)
```

## Use Cases

### Code Review
```bash
# Analyze file complexity before review
● project_analysis (MCP)(analysis_type: "file_analysis", query: "path/to/changed/file.go")

# Check for consistent error handling
● project_analysis (MCP)(analysis_type: "pattern_analysis", query: "error_handling")
```

### Refactoring Planning
```bash
# Find all references before refactoring
● project_analysis (MCP)(analysis_type: "references", query: "OldFunctionName")

# Analyze impact
● project_analysis (MCP)(analysis_type: "symbol_relationships", query: "OldFunctionName")
```

### Codebase Understanding
```bash
# Get workspace overview
● project_analysis (MCP)(analysis_type: "workspace_analysis", query: "entire_project")

# Explore symbols in a module
● project_analysis (MCP)(analysis_type: "workspace_symbols", query: "Authentication")
```

### Technical Debt Assessment
```bash
# Check naming consistency
● project_analysis (MCP)(analysis_type: "pattern_analysis", query: "naming_conventions")

# Find complex files
● project_analysis (MCP)(analysis_type: "file_analysis", query: "large/complex/file.go")
```

## Best Practices

### Performance Optimization
- Use specific queries rather than wildcards when possible
- Leverage pagination for large result sets
- Cache results when performing multiple related analyses

### Analysis Strategy
- Start with `workspace_analysis` for project overview
- Use `file_analysis` for detailed file understanding
- Use `symbol_relationships` before major refactoring
- Regular `pattern_analysis` to maintain code quality

## Multi-Language Support

The analysis automatically detects and works with multiple programming languages:
- Go, Python, JavaScript/TypeScript, Java, C/C++, Rust, and more...

Example with Python file:
```bash
● project_analysis (MCP)(analysis_type: "file_analysis", query: "scripts/test_mcp_tools.py")
```