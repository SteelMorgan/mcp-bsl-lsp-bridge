package bridge

import (
	"os"
	"strings"
	"time"

	"rockerboo/mcp-lsp-bridge/logger"
)

const defaultAutoConnectLanguages = "bsl"

func parseAutoConnectLanguages() []string {
	raw := strings.TrimSpace(os.Getenv("MCP_AUTO_CONNECT_LANGUAGES"))
	if raw == "" {
		raw = defaultAutoConnectLanguages
	}

	parts := strings.Split(raw, ",")
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		lang := strings.TrimSpace(p)
		if lang == "" {
			continue
		}
		out = append(out, lang)
	}
	return out
}

// StartAutoConnect triggers best-effort background connection to default language server clients.
// This is intentionally non-blocking and safe to call multiple times.
// It includes simple throttling to allow retries after failures.
func (b *MCPLSPBridge) StartAutoConnect() {
	b.autoConnectMu.Lock()
	defer b.autoConnectMu.Unlock()

	now := time.Now()
	if !b.autoConnectLastAttempt.IsZero() && now.Sub(b.autoConnectLastAttempt) < 5*time.Second {
		return
	}
	b.autoConnectLastAttempt = now
	if b.autoConnectStartedAt.IsZero() {
		b.autoConnectStartedAt = now
	}

	langs := parseAutoConnectLanguages()
	logger.Info("Auto-connect: starting language clients", strings.Join(langs, ","))

	go func() {
		for _, lang := range langs {
			if _, err := b.GetClientForLanguage(lang); err != nil {
				logger.Error("Auto-connect: failed to connect language client", lang, err)
				continue
			}
			logger.Info("Auto-connect: connected language client", lang)
		}
		// Trigger warm-up after best-effort connect.
		b.StartWarmup()
	}()
}

// SyncAutoConnect performs synchronous connection to language server clients.
// This BLOCKS until all connections complete (or fail).
// Does NOT trigger warmup - that can happen in background after MCP server starts.
// Use this in docker exec scenarios where stdin closes immediately after sending a request.
func (b *MCPLSPBridge) SyncAutoConnect() error {
	b.autoConnectMu.Lock()
	defer b.autoConnectMu.Unlock()

	now := time.Now()
	b.autoConnectLastAttempt = now
	if b.autoConnectStartedAt.IsZero() {
		b.autoConnectStartedAt = now
	}

	langs := parseAutoConnectLanguages()
	logger.Info("Sync auto-connect: connecting language clients", strings.Join(langs, ","))

	var lastErr error
	for _, lang := range langs {
		if _, err := b.GetClientForLanguage(lang); err != nil {
			logger.Error("Sync auto-connect: failed to connect language client", lang, err)
			lastErr = err
			continue
		}
		logger.Info("Sync auto-connect: connected language client", lang)
	}

	// Note: warmup is triggered in background by StartWarmup() which is called by tool handlers.
	// We don't block here on warmup because BSL LS may take minutes to index large projects.

	return lastErr
}
