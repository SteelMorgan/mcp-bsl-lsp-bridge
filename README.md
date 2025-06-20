# MCP LSP Bridge

A Go-based bridge that combines MCP (Model Context Protocol) server capabilities with LSP (Language Server Protocol) client functionality for advanced code analysis and interaction.

## Features

### Core Capabilities
- **Cross-platform directory management** following system conventions
- **Smart configuration loading** with multiple fallback locations
- **XDG Base Directory Specification** compliance on Unix systems
- **Root user support** with appropriate system directories
- **Comprehensive LSP server support** for 20+ programming languages

### Advanced LSP Tools (New!)
- **Code Intelligence**: Hover documentation, signature help, diagnostics
- **Code Actions**: Quick fixes, refactoring suggestions, and code improvements
- **Code Formatting**: Document formatting with customizable options
- **Advanced Navigation**: Symbol renaming, implementation finding, call hierarchy
- **Multi-Language Support**: Works across Go, Python, TypeScript, Rust, and more

## Usage

### Basic Usage

```bash
# Build the project
go build

# Run with default configuration
./mcp-lsp-bridge
```

### Command-Line Options

The MCP-LSP Bridge supports flexible configuration through command-line flags:

```bash
# Configuration file options
--config, -c    Path to LSP configuration file (default: "lsp_config.json")

# Logging options
--log-path, -l  Path to log file (overrides config file setting)
--log-level     Log level: debug, info, warn, error (overrides config file setting)
```

### Examples

```bash
# Use default platform-appropriate directories
./mcp-lsp-bridge

# Use custom configuration file
./mcp-lsp-bridge --config /path/to/my-config.json

# Set custom log file and level
./mcp-lsp-bridge --log-path /var/log/mcp-bridge.log --log-level debug

# Combine options
./mcp-lsp-bridge -c /etc/mcp/config.json -l /tmp/debug.log

# Configuration file is required for LSP functionality
./mcp-lsp-bridge --config /path/to/valid-config.json
```

### Default Directory Locations

**Regular Users:**

- Config: `~/.config/mcp-lsp-bridge/lsp_config.json`
- Logs: `~/.local/share/mcp-lsp-bridge/logs/mcp-lsp-bridge.log`
- Data: `~/.local/share/mcp-lsp-bridge/`
- Cache: `~/.cache/mcp-lsp-bridge/`

**Root User:**

- Config: `/etc/mcp-lsp-bridge/lsp_config.json`
- Logs: `/var/log/mcp-lsp-bridge/mcp-lsp-bridge.log`
- Data: `/var/lib/mcp-lsp-bridge/`
- Cache: `/var/cache/mcp-lsp-bridge/`

**Windows Users:**

- Config: `%APPDATA%\mcp-lsp-bridge\lsp_config.json`
- Logs: `%LOCALAPPDATA%\mcp-lsp-bridge\logs\mcp-lsp-bridge.log`

### Configuration Fallback

The bridge attempts to load configuration from multiple locations in order:

1. Command-line specified path
2. Platform-appropriate config directory
3. Current directory (`lsp_config.json`)
4. Alternative config name (`config.json`)
5. Example config (`example.lsp_config.json`)

## Available MCP Tools

The bridge exposes the following MCP tools for Claude Code integration:

### Core Analysis Tools
- **`mcp__lsp__analyze_code`**: Analyze code for completion suggestions and insights
- **`mcp__lsp__infer_language`**: Detect programming language from file path
- **`mcp__lsp__detect_project_languages`**: Detect all languages in a project

### LSP Connection Management
- **`mcp__lsp__lsp_connect`**: Connect to appropriate language server for a file
- **`mcp__lsp__lsp_disconnect`**: Disconnect all active language servers

### Code Intelligence Tools ✨ New!
- **`mcp__lsp__hover`**: Get symbol documentation and type information
- **`mcp__lsp__signature_help`**: Get function parameter assistance
- **`mcp__lsp__diagnostics`**: Get error and warning diagnostics

### Code Improvement Tools ✨ New!
- **`mcp__lsp__code_actions`**: Get quick fixes and refactoring suggestions
- **`mcp__lsp__format_document`**: Format code with customizable options

### Advanced Navigation Tools ✨ New!
- **`mcp__lsp__rename`**: Rename symbols with optional preview
- **`mcp__lsp__implementation`**: Find symbol implementations
- **`mcp__lsp__call_hierarchy`**: Prepare call hierarchy analysis

### Project Analysis Tools
- **`mcp__lsp__project_analysis`**: Enhanced project-wide analysis with:
  - Workspace symbol search
  - Reference finding
  - Definition location
  - Text-based search

## Configuration

### Configuration File Requirement

**A valid LSP configuration file is required for the bridge to function.** The bridge will exit if it cannot load the configuration file, as it needs:

- Language server definitions (gopls, pyright-langserver, etc.)
- File extension to language mappings
- LSP server connection parameters

### Configuration Priority

1. **Command-line flags** (highest priority for logging settings)
2. **Configuration file settings** (required for LSP functionality)

Only logging configuration has fallback defaults - the core LSP functionality requires a valid config file.

## Logging Configuration

The MCP LSP Bridge provides a flexible logging system with the following features:

### Configuration Options

The logger can be configured through the `lsp_config.json` file under the `global` section:

```json
{
  "global": {
    "log_file_path": "/var/log/mcp-lsp-bridge/bridge.log",
    "log_level": "info",
    "max_log_files": 5
  }
}
```

#### Configuration Parameters

- `log_file_path`: Path to the log file. Defaults to a temporary file if not specified.
- `log_level`: Logging verbosity. Options:
  - `info`: Default, logs informational messages
  - `debug`: Logs debug messages in addition to info and error
  - `error`: Logs only error messages
- `max_log_files`: Maximum number of log files to keep before rotation. Default is 5.

## Docker

Docker implementation is available but no LSP servers are installed. Ideally you'd make your own extended container which includes the LSP servers you want to support.

```bash
docker pull docker.io/rockerboo/mcp-lsp-bridge
docker run -it --rm rockerboo/mcp-lsp-bridge:latest
```

```Dockerfile
# Use the official rockerboo/mcp-lsp-bridge image as a base
FROM rockerboo/mcp-lsp-bridge:latest

# Install additional LSP servers
RUN apt-get update && apt-get install -y \
    npm \
    && npm install -g @typescript-language-server typescript-language-server \
    && npm install -g diagnostic-languageserver

# Optional: Add more LSP servers as needed
# RUN npm install -g <another-lsp-server>

# Create the config directory
RUN mkdir -p /home/user/.config/mcp-lsp-bridge

# Copy your LSP configuration file to the docker container
COPY lsp_config.json /home/user/.config/mcp-lsp-bridge/lsp_config.json

# Set the user
USER user

# Set the command to run when the container starts
CMD ["mcp-lsp-bridge"]
```

### Programmatic Configuration

You can also configure logging programmatically:

```go
logConfig := logger.LoggerConfig{
    LogPath:     "/path/to/logfile.log",
    LogLevel:    "debug",
    MaxLogFiles: 3,
}
logger.InitLogger(logConfig)
```

### Logging Methods

- `logger.Info()`: Log informational messages
- `logger.Debug()`: Log debug messages (only when log level is "debug")
- `logger.Error()`: Log error messages
