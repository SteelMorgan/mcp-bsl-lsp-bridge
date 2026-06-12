package bridge

// Multi-project routing for the bridge side.
//
// In multi-project mode the daemon serves several 1C configurations behind one
// BSL LS process and routes each document to the right ServerContext by URI
// prefix. The bridge's job is therefore narrow:
//   - derive a project root from a file path (deriveProjectRoot),
//   - auto-register that project with the daemon the first time it is touched,
//   - expose explicit project_add / project_close / project_list operations.
//
// File-tool signatures do not change: the routing is by URI and happens inside
// BSL LS. Auto-add is best-effort — if it fails, the file call still proceeds
// (the daemon may simply return empty results until the project is indexed).

import (
	"os"
	"path/filepath"
	"time"

	"rockerboo/mcp-lsp-bridge/logger"
	"rockerboo/mcp-lsp-bridge/lsp"
	"rockerboo/mcp-lsp-bridge/utils"
)

// mpProbeInterval throttles how often we re-probe the daemon for the
// multi-project flag while it still reads false (single-project hot path).
const mpProbeInterval = 30 * time.Second

// Compile-time guarantee that the session adapter implements the multi-project
// API the bridge relies on. A signature drift in either package fails the build.
var _ projectController = (*lsp.SessionAdapter)(nil)

// projectController is implemented by clients that support the daemon's
// multi-project API (currently *lsp.SessionAdapter).
type projectController interface {
	MultiProjectEnabled() bool
	ProjectAdd(root string) (map[string]interface{}, error)
	ProjectClose(root string) (map[string]interface{}, error)
	ProjectList() (map[string]interface{}, error)
}

// projectMarkers are files/dirs that mark the root of a 1C project (EDT or
// designer layout, or an explicit BSL LS config / VCS root).
var projectMarkers = []string{
	"Configuration.xml",
	".bsl-language-server.json",
	"src/cf/Configuration.xml",
	"DT-INF",   // EDT project metadata
	".project", // EDT/Eclipse project file
	".git",     // VCS root (fallback)
}

// deriveProjectRoot walks up from the file's directory looking for a project
// marker and returns the first directory that contains one. Returns "" when no
// marker is found. The exists callback is injected for testability.
func deriveProjectRoot(filePath string, exists func(string) bool) string {
	dir := filepath.Dir(filePath)
	for {
		for _, marker := range projectMarkers {
			if exists(filepath.Join(dir, marker)) {
				return dir
			}
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return "" // reached filesystem root without a marker
		}
		dir = parent
	}
}

// fileExists reports whether a path exists on the real filesystem.
func fileExists(p string) bool {
	_, err := os.Stat(p)
	return err == nil
}

// DeriveProjectRoot resolves the project root for a file URI/path on the real
// filesystem (server namespace). Returns "" if it cannot be determined.
func (b *MCPLSPBridge) DeriveProjectRoot(uri string) string {
	filePath := utils.URIToFilePath(uri)
	if b.HasPathMapper() && b.pathMapper != nil {
		if mapped, err := b.pathMapper.HostToContainer(filePath); err == nil {
			filePath = mapped
		}
	}
	return deriveProjectRoot(filePath, fileExists)
}

// projectControllerClient returns the first connected client that supports the
// multi-project API, or nil.
func (b *MCPLSPBridge) projectControllerClient() projectController {
	b.mu.RLock()
	defer b.mu.RUnlock()
	for _, c := range b.clients {
		if pc, ok := c.(projectController); ok {
			return pc
		}
	}
	return nil
}

// MultiProjectEnabled reports whether the connected daemon is in multi-project
// mode. Safe to call when no client supports it (returns false). The result is
// cached: once confirmed true it sticks; while false it is re-probed at most
// every mpProbeInterval so the single-project hot path stays cheap.
func (b *MCPLSPBridge) MultiProjectEnabled() bool {
	pc := b.projectControllerClient()
	if pc == nil {
		return false
	}
	return b.multiProjectActive(pc)
}

// multiProjectActive is the cached probe behind MultiProjectEnabled.
func (b *MCPLSPBridge) multiProjectActive(pc projectController) bool {
	b.projectMu.Lock()
	if b.mpEnabled {
		b.projectMu.Unlock()
		return true
	}
	if !b.mpCheckedAt.IsZero() && time.Since(b.mpCheckedAt) < mpProbeInterval {
		b.projectMu.Unlock()
		return false
	}
	b.projectMu.Unlock()

	enabled := pc.MultiProjectEnabled()

	b.projectMu.Lock()
	b.mpCheckedAt = time.Now()
	if enabled {
		b.mpEnabled = true
	}
	b.projectMu.Unlock()
	return enabled
}

// ProjectAdd explicitly registers/warms a project.
func (b *MCPLSPBridge) ProjectAdd(root string) (map[string]interface{}, error) {
	pc := b.projectControllerClient()
	if pc == nil {
		return nil, errNoMultiProject
	}
	res, err := pc.ProjectAdd(root)
	if err == nil {
		b.rememberProject(root)
	}
	return res, err
}

// ProjectClose explicitly unregisters a project and frees its memory.
func (b *MCPLSPBridge) ProjectClose(root string) (map[string]interface{}, error) {
	pc := b.projectControllerClient()
	if pc == nil {
		return nil, errNoMultiProject
	}
	res, err := pc.ProjectClose(root)
	if err == nil {
		b.forgetProject(root)
	}
	return res, err
}

// ProjectList returns all registered projects.
func (b *MCPLSPBridge) ProjectList() (map[string]interface{}, error) {
	pc := b.projectControllerClient()
	if pc == nil {
		return nil, errNoMultiProject
	}
	return pc.ProjectList()
}

// ensureProjectForURI auto-registers the project owning uri with the daemon. It
// is cheap on the hot path: the daemon is contacted only when the active
// project root changes or the project has not been seen yet. Best-effort: any
// error is logged and swallowed so the file operation can still proceed.
func (b *MCPLSPBridge) ensureProjectForURI(uri string) {
	pc := b.projectControllerClient()
	if pc == nil || !b.multiProjectActive(pc) {
		return
	}

	root := b.DeriveProjectRoot(uri)
	if root == "" {
		return
	}
	root = filepath.Clean(root)

	b.projectMu.Lock()
	if b.knownProjects == nil {
		b.knownProjects = make(map[string]bool)
	}
	known := b.knownProjects[root]
	sameAsLast := b.lastTouchedRoot == root
	b.lastTouchedRoot = root
	if !known {
		b.knownProjects[root] = true
	}
	b.projectMu.Unlock()

	// Already the active project and already registered: nothing to do.
	if known && sameAsLast {
		return
	}

	// Add (idempotent server-side) — also updates the daemon's last_used so the
	// right project is warmed after a restart.
	if _, err := pc.ProjectAdd(root); err != nil {
		logger.Warn("multi-project: auto-add of " + root + " failed: " + err.Error())
		// Drop from the cache so a later call retries.
		b.forgetProject(root)
	}
}

func (b *MCPLSPBridge) rememberProject(root string) {
	root = filepath.Clean(root)
	b.projectMu.Lock()
	defer b.projectMu.Unlock()
	if b.knownProjects == nil {
		b.knownProjects = make(map[string]bool)
	}
	b.knownProjects[root] = true
	b.lastTouchedRoot = root
}

func (b *MCPLSPBridge) forgetProject(root string) {
	root = filepath.Clean(root)
	b.projectMu.Lock()
	defer b.projectMu.Unlock()
	delete(b.knownProjects, root)
	if b.lastTouchedRoot == root {
		b.lastTouchedRoot = ""
	}
}

// errNoMultiProject is returned by project operations when no connected client
// supports the multi-project API.
var errNoMultiProject = multiProjectError("multi-project mode is not available (no session-mode client, or MULTI_PROJECT=0)")

type multiProjectError string

func (e multiProjectError) Error() string { return string(e) }
