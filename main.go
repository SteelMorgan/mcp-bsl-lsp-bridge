package main

import (
	"log"

	"rockerboo/mcp-lsp-bridge/bridge"
	"rockerboo/mcp-lsp-bridge/mcpserver"

	"github.com/mark3labs/mcp-go/server"
)

func main() {
	// Configure logging
	log.Println("Starting MCP-LSP Bridge...")

	// Create and initialize the bridge
	bridgeInstance := bridge.NewMCPLSPBridge()

	// Setup MCP server with bridge
	mcpServer := mcpserver.SetupMCPServer(bridgeInstance)

	// Store the server reference in the bridge
	bridgeInstance.SetServer(mcpServer)

	// Start MCP server
	log.Println("Starting MCP server...")
	if err := server.ServeStdio(mcpServer); err != nil {
		log.Printf("MCP server error: %v", err)
	}
}
