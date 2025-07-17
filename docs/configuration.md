# Configuration Guide

## Requirements

A valid `lsp_config.json` file is required for the bridge to function. The bridge needs:

- Language server definitions (gopls, pyright-langserver, etc.)
- File extension to language mappings  
- LSP server connection parameters

## Command-Line Options

```bash
# Configuration file options
--config, -c    Path to LSP configuration file (default: "lsp_config.json")

# Logging options
--log-path, -l  Path to log file (overrides config file setting)
--log-level     Log level: debug, info, warn, error (overrides config file setting)
```

## Examples

```bash
# Use default platform-appropriate directories
./mcp-lsp-bridge

# Use custom configuration file
./mcp-lsp-bridge --config /path/to/my-config.json

# Set custom log file and level
./mcp-lsp-bridge --log-path /var/log/mcp-bridge.log --log-level debug

# Combine options
./mcp-lsp-bridge -c /etc/mcp/config.json -l /tmp/debug.log
```

## Default Directory Locations

### Regular Users

- Config: `~/.config/mcp-lsp-bridge/lsp_config.json`
- Logs: `~/.local/share/mcp-lsp-bridge/logs/mcp-lsp-bridge.log`
- Data: `~/.local/share/mcp-lsp-bridge/`
- Cache: `~/.cache/mcp-lsp-bridge/`

### Root User

- Config: `/etc/mcp-lsp-bridge/lsp_config.json`
- Logs: `/var/log/mcp-lsp-bridge/mcp-lsp-bridge.log`
- Data: `/var/lib/mcp-lsp-bridge/`
- Cache: `/var/cache/mcp-lsp-bridge/`

### Windows Users

- Config: `%APPDATA%\mcp-lsp-bridge\lsp_config.json`
- Logs: `%LOCALAPPDATA%\mcp-lsp-bridge\logs\mcp-lsp-bridge.log`

## Configuration Fallback

The bridge attempts to load configuration from multiple locations in order:

1. Command-line specified path
2. Platform-appropriate config directory
3. Current directory (`lsp_config.json`)
4. Alternative config name (`config.json`)
5. Example config (`example.lsp_config.json`)

## Configuration Priority

1. **Command-line flags** (highest priority for logging settings)
2. **Configuration file settings** (required for LSP functionality)

Only logging configuration has fallback defaults - the core LSP functionality requires a valid config file.

## Logging Configuration

Configure logging through the `lsp_config.json` file under the `global` section:

```json
{
  "global": {
    "log_file_path": "/var/log/mcp-lsp-bridge/bridge.log",
    "log_level": "info",
    "max_log_files": 5
  }
}
```

### Parameters

- `log_file_path`: Path to the log file. Defaults to a temporary file if not specified.
- `log_level`: Logging verbosity. Options:
  - `info`: Default, logs informational messages
  - `debug`: Logs debug messages in addition to info and error
  - `error`: Logs only error messages
- `max_log_files`: Maximum number of log files to keep before rotation. Default is 5.

## Docker Usage

Base image available (LSP servers not included):

```bash
docker pull ghcr.io/rockerboo/mcp-lsp-bridge:latest
docker run -it --rm ghcr.io/rockerboo/mcp-lsp-bridge:latest
```

### Extending with LSP Servers

Create your own image with needed LSP servers:

```Dockerfile
# Use the official rockerboo/mcp-lsp-bridge image as a base
FROM ghcr.io/rockerboo/mcp-lsp-bridge:latest

# Install additional LSP servers
RUN apk add --no-cache npm

# Set the user
USER user

RUN npm install -g typescript-language-server typescript

# Optional: Add more LSP servers as needed
# RUN npm install -g <another-lsp-server>

# Create the config directory
RUN mkdir -p /home/user/.config/mcp-lsp-bridge

# Copy your LSP configuration file to the docker container
COPY lsp_config.json /home/user/.config/mcp-lsp-bridge/lsp_config.json

# Set the command to run when the container starts
CMD ["mcp-lsp-bridge"]
```