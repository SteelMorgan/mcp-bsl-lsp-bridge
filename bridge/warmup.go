package bridge

import (
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"time"

	"rockerboo/mcp-lsp-bridge/logger"
)

// StartWarmup triggers best-effort warm-up (indexing/cache building) for connected language clients.
// It is non-blocking and safe to call multiple times; it includes simple throttling.
func (b *MCPLSPBridge) StartWarmup() {
	b.warmupMu.Lock()
	defer b.warmupMu.Unlock()

	now := time.Now()
	// Throttle repeated warmups
	if !b.warmupLastAttempt.IsZero() && now.Sub(b.warmupLastAttempt) < 10*time.Second {
		return
	}
	b.warmupLastAttempt = now

	// Do not start again if already done successfully
	if b.warmupDone {
		return
	}
	// Do not start if already running
	if b.warmupRunning {
		return
	}

	b.warmupRunning = true
	if b.warmupStartedAt.IsZero() {
		b.warmupStartedAt = now
	}
	b.warmupErr = ""

	go b.runWarmup()
}

func (b *MCPLSPBridge) finishWarmup(err error) {
	b.warmupMu.Lock()
	defer b.warmupMu.Unlock()
	b.warmupRunning = false
	b.warmupFinishedAt = time.Now()
	if err != nil {
		b.warmupErr = err.Error()
		b.warmupDone = false
	} else {
		b.warmupErr = ""
		b.warmupDone = true
	}
}

// SyncWarmup performs synchronous warm-up for connected language clients.
// This BLOCKS until warm-up completes.
// Use this in docker exec scenarios where stdin closes immediately after sending a request.
func (b *MCPLSPBridge) SyncWarmup() {
	b.warmupMu.Lock()
	// Do not start again if already done successfully
	if b.warmupDone {
		b.warmupMu.Unlock()
		return
	}
	// Do not start if already running
	if b.warmupRunning {
		b.warmupMu.Unlock()
		return
	}

	b.warmupRunning = true
	now := time.Now()
	b.warmupLastAttempt = now
	if b.warmupStartedAt.IsZero() {
		b.warmupStartedAt = now
	}
	b.warmupErr = ""
	b.warmupMu.Unlock()

	// Run warmup synchronously (blocking)
	b.runWarmup()
}

func (b *MCPLSPBridge) runWarmup() {
	// For now, warm up the default BSL language client if available.
	langs := parseAutoConnectLanguages()
	if len(langs) == 0 {
		langs = []string{"bsl"}
	}

	// Resolve a workspace root for scanning.
	roots := b.AllowedDirectories()
	workspaceRoot := ""
	if len(roots) > 0 {
		workspaceRoot = roots[0]
	}
	if workspaceRoot == "" {
		b.finishWarmup(fmt.Errorf("warmup: no allowed directories configured"))
		return
	}

	logger.Info("Warm-up: starting", "workspaceRoot", workspaceRoot, "langs", strings.Join(langs, ","))

	// Connect clients synchronously (best effort) so that warmup work can run.
	for _, lang := range langs {
		if _, err := b.GetClientForLanguage(lang); err != nil {
			logger.Error("Warm-up: failed to connect language client", lang, err)
			// Keep going; maybe other langs succeed.
		}
	}

	// Pick a small number of .bsl files to touch (parse) to trigger indexing.
	// Keep it bounded to avoid huge startup cost.
	const maxFiles = 5
	var files []string
	_ = filepath.WalkDir(workspaceRoot, func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return nil
		}
		if d.IsDir() {
			return nil
		}
		if strings.HasSuffix(strings.ToLower(d.Name()), ".bsl") {
			files = append(files, p)
			if len(files) >= maxFiles {
				return fs.SkipAll
			}
		}
		return nil
	})

	if len(files) == 0 {
		// Still mark warmup done; nothing to scan.
		logger.Warn("Warm-up: no .bsl files found under workspace root", workspaceRoot)
		b.finishWarmup(nil)
		return
	}

	// Touch documents to force parse/symbol tables.
	for _, f := range files {
		// Read once to ensure file exists in server filesystem.
		if _, err := os.Stat(f); err != nil {
			continue
		}
		_, _ = b.GetDocumentSymbols(f) // best effort: triggers didOpen + documentSymbol
	}

	// Attempt a cheap workspace symbol query to encourage cross-file indexing.
	// Ignore errors; some servers may not support it.
	_, _ = b.SearchTextInWorkspace("bsl", "ПараметрыОперации")

	// Wait for server progress (if reported) to settle, but do not block forever.
	deadline := time.Now().Add(2 * time.Minute)
	stableSince := time.Time{}
	for time.Now().Before(deadline) {
		clients := b.ListConnectedClients()
		// If no clients, give up.
		if len(clients) == 0 {
			break
		}
		anyActive := false
		for _, c := range clients {
			if ps, ok := c.(interface{ ProgressSnapshot() any }); ok {
				_ = ps // just type check; actual snapshot is exposed on concrete lsp client, but interface varies.
			}
			// We can't reliably access snapshot from the interface here without importing lsp package.
			// So we just break out early; readiness gate will still block until explicitly marked done.
			anyActive = false
		}
		if !anyActive {
			if stableSince.IsZero() {
				stableSince = time.Now()
			}
			if time.Since(stableSince) > 2*time.Second {
				break
			}
		} else {
			stableSince = time.Time{}
		}
		time.Sleep(200 * time.Millisecond)
	}

	logger.Info("Warm-up: finished")
	b.finishWarmup(nil)
}

