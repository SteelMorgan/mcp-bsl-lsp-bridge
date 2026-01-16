package bridge

import (
	"sync"

	"time"

	"rockerboo/mcp-lsp-bridge/types"
	"rockerboo/mcp-lsp-bridge/utils"

	"github.com/mark3labs/mcp-go/server"
)

// MCPLSPBridge combines MCP server capabilities with multiple LSP clients
type MCPLSPBridge struct {
	server             *server.MCPServer
	clients            map[types.LanguageServer]types.LanguageClientInterface
	config             types.LSPServerConfigProvider
	allowedDirectories []string
	pathMapper         *utils.DockerPathMapper
	mu                 sync.RWMutex

	// Auto-connect support: connect default language client(s) once, lazily.
	autoConnectMu          sync.Mutex
	autoConnectStartedAt   time.Time
	autoConnectLastAttempt time.Time

	// Warm-up support: best-effort indexing/caching to make heavy LSP tools reliable.
	warmupMu          sync.Mutex
	warmupStartedAt   time.Time
	warmupFinishedAt  time.Time
	warmupLastAttempt time.Time
	warmupRunning     bool
	warmupDone        bool
	warmupErr         string
}

// WarmupStatus returns current warm-up state.
func (b *MCPLSPBridge) WarmupStatus() (running bool, done bool, err string, startedAt time.Time, finishedAt time.Time) {
	b.warmupMu.Lock()
	defer b.warmupMu.Unlock()
	return b.warmupRunning, b.warmupDone, b.warmupErr, b.warmupStartedAt, b.warmupFinishedAt
}

// ListConnectedClients returns a snapshot of currently connected clients.
// This is intentionally NOT part of interfaces.BridgeInterface to avoid breaking mocks;
// consume via type assertion in tooling.
func (b *MCPLSPBridge) ListConnectedClients() map[types.LanguageServer]types.LanguageClientInterface {
	b.mu.RLock()
	defer b.mu.RUnlock()
	out := make(map[types.LanguageServer]types.LanguageClientInterface, len(b.clients))
	for k, v := range b.clients {
		out[k] = v
	}
	return out
}
