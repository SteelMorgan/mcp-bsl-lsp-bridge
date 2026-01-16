#!/bin/sh
set -eu

# MCP-LSP Bridge container startup script
# Runs mcp-lsp-bridge in daemon mode with pre-warming of language servers

echo "Starting MCP-LSP Bridge with pre-warming..."
echo "Workspace: ${WORKSPACE_ROOT:-/projects}"

# Pre-warm the bridge by making a simple request
# This triggers BSL LS startup and workspace indexing
prewarm() {
  echo "Pre-warming BSL Language Server (this may take a while for large projects)..."
  
  # Send initialize + lsp_status request to trigger LSP connection
  INIT_REQ='{"jsonrpc":"2.0","id":1,"method":"initialize","params":{"protocolVersion":"2024-11-05","clientInfo":{"name":"prewarm","version":"1.0"},"capabilities":{}}}'
  STATUS_REQ='{"jsonrpc":"2.0","id":2,"method":"tools/call","params":{"name":"lsp_status","arguments":{}}}'
  
  # Run mcp-lsp-bridge with timeout, send requests
  printf '%s\n%s\n' "$INIT_REQ" "$STATUS_REQ" | timeout 120 mcp-lsp-bridge 2>/dev/null || true
  
  echo "Pre-warming complete."
}

# Run pre-warming in background
prewarm &

# Keep container running
trap 'exit 0' TERM INT
while true; do
  sleep 3600 || exit 0
done

