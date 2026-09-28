package main

import (
	"context"
	"fmt"
	"log"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

var bslLSPSourceExtensions = map[string]bool{
	".bsl": true,
	".os":  true,
}

var rlmSourceExtensions = map[string]bool{
	".bsl":        true,
	".os":         true,
	".xml":        true,
	".mdo":        true,
	".json":       true,
	".rights":     true,
	".form":       true,
	".query":      true,
	".txt":        true,
	".md":         true,
	".properties": true,
}

func watchedSourceExtensions() []string {
	out := make([]string, 0, len(rlmSourceExtensions))
	for ext := range rlmSourceExtensions {
		out = append(out, ext)
	}
	return out
}

func isWatchedSourceExtension(ext string) bool {
	return rlmSourceExtensions[strings.ToLower(ext)]
}

func isBSLLSPSourceURI(uri string) bool {
	return bslLSPSourceExtensions[strings.ToLower(filepath.Ext(uriToPath(uri)))]
}

func filterLSPFileChanges(changes []FileChange) []FileChange {
	out := make([]FileChange, 0, len(changes))
	for _, c := range changes {
		if isBSLLSPSourceURI(c.URI) {
			out = append(out, c)
		}
	}
	return out
}

func filterLSPChangeMaps(changes []map[string]interface{}) []map[string]interface{} {
	out := make([]map[string]interface{}, 0, len(changes))
	for _, c := range changes {
		uri, _ := c["uri"].(string)
		if isBSLLSPSourceURI(uri) {
			out = append(out, c)
		}
	}
	return out
}

type RLMIndexManager struct {
	sm       *SessionManager
	enabled  bool
	bin      string
	debounce time.Duration
	timeout  time.Duration

	mu      sync.Mutex
	timers  map[string]*time.Timer
	running map[string]bool
	queued  map[string]bool
	stopped bool
}

func NewRLMIndexManager(sm *SessionManager) *RLMIndexManager {
	return &RLMIndexManager{
		sm:       sm,
		enabled:  envBool("RLM_INDEX_ENABLED", true),
		bin:      envString("RLM_INDEX_BIN", "rlm-bsl-index"),
		debounce: envDuration("RLM_INDEX_DEBOUNCE", 10*time.Second),
		timeout:  envDuration("RLM_INDEX_TIMEOUT", 30*time.Minute),
		timers:   make(map[string]*time.Timer),
		running:  make(map[string]bool),
		queued:   make(map[string]bool),
	}
}

func (m *RLMIndexManager) Stop() {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.stopped = true
	for _, t := range m.timers {
		t.Stop()
	}
}

func (m *RLMIndexManager) Ensure(root string) {
	if !m.enabled || !envBool("RLM_INDEX_BUILD_ON_START", true) {
		return
	}
	m.enqueue(root, 0)
}

func (m *RLMIndexManager) Update(root string) {
	if !m.enabled || !envBool("RLM_INDEX_UPDATE_ON_CHANGE", true) {
		return
	}
	m.enqueue(root, m.debounce)
}

func (m *RLMIndexManager) enqueue(root string, delay time.Duration) {
	root = normalizeRoot(root)
	if root == "" {
		return
	}

	m.mu.Lock()
	defer m.mu.Unlock()
	if m.stopped {
		return
	}
	if existing := m.timers[root]; existing != nil {
		existing.Stop()
	}
	m.timers[root] = time.AfterFunc(delay, func() {
		m.runSerialized(root)
	})
}

func (m *RLMIndexManager) runSerialized(root string) {
	m.mu.Lock()
	delete(m.timers, root)
	if m.stopped {
		m.mu.Unlock()
		return
	}
	if m.running[root] {
		m.queued[root] = true
		m.mu.Unlock()
		return
	}
	m.running[root] = true
	m.mu.Unlock()

	m.buildOrUpdate(root)

	m.mu.Lock()
	m.running[root] = false
	again := m.queued[root]
	delete(m.queued, root)
	m.mu.Unlock()
	if again {
		m.enqueue(root, m.debounce)
	}
}

func (m *RLMIndexManager) buildOrUpdate(root string) {
	if _, err := os.Stat(root); err != nil {
		log.Printf("RLM index: skip %s: %v", root, err)
		return
	}

	action := "update"
	if !m.indexExists(root) {
		action = "build"
	}
	log.Printf("RLM index: %s %s", action, root)

	ctx, cancel := context.WithTimeout(context.Background(), m.timeout)
	defer cancel()
	cmd := m.command(ctx, "index", action, root)
	out, err := cmd.CombinedOutput()
	if ctx.Err() == context.DeadlineExceeded {
		log.Printf("RLM index: %s timed out for %s after %s", action, root, m.timeout)
		return
	}
	if err != nil {
		log.Printf("RLM index: %s failed for %s: %v\n%s", action, root, err, trimLog(out, 4000))
		return
	}
	log.Printf("RLM index: %s completed for %s\n%s", action, root, trimLog(out, 1000))
}

func (m *RLMIndexManager) indexExists(root string) bool {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	cmd := m.command(ctx, "index", "info", root)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return false
	}
	s := strings.ToLower(string(out))
	return !strings.Contains(s, "not found") && !strings.Contains(s, "missing")
}

func (m *RLMIndexManager) command(ctx context.Context, args ...string) *exec.Cmd {
	cmd := exec.CommandContext(ctx, m.bin, args...)
	cmd.Env = append(os.Environ(),
		"HOME=/home/user",
		"XDG_CONFIG_HOME=/home/user/.config",
		"XDG_CACHE_HOME=/home/user/.cache",
		"XDG_DATA_HOME=/home/user/.local/share",
	)
	return cmd
}

func (sm *SessionManager) enqueueRLMEnsure(root string) {
	if sm.rlmIndex != nil {
		sm.rlmIndex.Ensure(root)
	}
}

func (sm *SessionManager) enqueueRLMChanges(changes []FileChange) {
	if sm.rlmIndex == nil || len(changes) == 0 {
		return
	}
	roots := map[string]bool{}
	for _, c := range changes {
		root := sm.rlmRootForURI(c.URI)
		if root != "" {
			roots[root] = true
		}
	}
	for root := range roots {
		sm.rlmIndex.Update(root)
	}
}

func (sm *SessionManager) rlmRootForURI(uri string) string {
	path := uriToPath(uri)
	if path == "" {
		return ""
	}
	if sm.multiProject && sm.pm != nil {
		var best string
		for _, p := range sm.pm.List() {
			root := normalizeRoot(p.Root)
			if path == root || strings.HasPrefix(path, root+string(os.PathSeparator)) {
				if len(root) > len(best) {
					best = root
				}
			}
		}
		if best != "" {
			return best
		}
		if root := deriveProjectRootForPath(path); root != "" {
			return root
		}
	}
	return sm.rootDir()
}

func deriveProjectRootForPath(path string) string {
	dir := path
	if info, err := os.Stat(path); err != nil || !info.IsDir() {
		dir = filepath.Dir(path)
	}
	markers := []string{
		"Configuration.xml",
		".bsl-language-server.json",
		filepath.Join("src", "cf", "Configuration.xml"),
		"DT-INF",
		".project",
		".git",
	}
	for {
		for _, marker := range markers {
			if _, err := os.Stat(filepath.Join(dir, marker)); err == nil {
				return dir
			}
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return ""
		}
		dir = parent
	}
}

func uriToPath(uri string) string {
	if strings.HasPrefix(uri, "file://") {
		u, err := url.Parse(uri)
		if err == nil {
			return filepath.Clean(u.Path)
		}
		return filepath.Clean(strings.TrimPrefix(uri, "file://"))
	}
	return filepath.Clean(uri)
}

func envString(name, def string) string {
	if v := os.Getenv(name); v != "" {
		return v
	}
	return def
}

func envBool(name string, def bool) bool {
	v := strings.ToLower(strings.TrimSpace(os.Getenv(name)))
	if v == "" {
		return def
	}
	return v == "1" || v == "true" || v == "yes" || v == "on"
}

func envDuration(name string, def time.Duration) time.Duration {
	v := os.Getenv(name)
	if v == "" {
		return def
	}
	d, err := time.ParseDuration(v)
	if err != nil {
		log.Printf("Invalid %s=%q: %v; using %s", name, v, err, def)
		return def
	}
	return d
}

func trimLog(b []byte, max int) string {
	s := strings.TrimSpace(string(b))
	if len(s) <= max {
		return s
	}
	return fmt.Sprintf("%s... [truncated %d bytes]", s[:max], len(s)-max)
}
