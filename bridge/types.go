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

// GetConnectedLanguages returns a list of languages for which clients are already connected.
// This provides a fast path for tools that need language info without expensive filesystem scans.
func (b *MCPLSPBridge) GetConnectedLanguages() []types.Language {
	b.mu.RLock()
	defer b.mu.RUnlock()

	var languages []types.Language
	for server := range b.clients {
		// Server key can be either a language name (e.g., "bsl") or a server name (e.g., "bsl-language-server")
		// Try it as a language first
		languages = append(languages, types.Language(server))
	}
	return languages
}

// AllClientsInSessionMode returns true if ALL connected clients use session mode.
// In session mode, LSP Session Manager handles initialization and warmup,
// so mcp-lsp-bridge should skip its own warmup gate.
func (b *MCPLSPBridge) AllClientsInSessionMode() bool {
	b.mu.RLock()
	defer b.mu.RUnlock()

	if len(b.clients) == 0 {
		return false
	}

	// Get all server configs
	serverConfigs := b.config.GetLanguageServers()

	// Check each connected client's server config
	// Note: b.clients keys are language names (e.g., "bsl"), not server names
	for langKey := range b.clients {
		// Try to find server config by treating key as server name first
		serverConfig, exists := serverConfigs[langKey]
		if !exists {
			// Key might be a language, find the server name for it
			serverName := b.config.GetServerNameFromLanguage(types.Language(langKey))
			if serverName == "" {
				return false
			}
			serverConfig, exists = serverConfigs[serverName]
		}
		if !exists || serverConfig == nil || !serverConfig.IsSessionMode() {
			return false
		}
	}
	return true
}
