package main

// ProjectManager keeps the registry of warm 1C projects (workspace folders) for
// the multi-project mode. It is the Go-side source of truth for which projects
// are currently registered with BSL LS, their indexing state, and which one was
// used last (persisted to state.json so it can be pre-warmed after a restart).
//
// The manager is pure state + persistence: it does NOT talk to BSL LS. The
// daemon drives the actual workspace/didChangeWorkspaceFolders calls and updates
// the manager accordingly. This keeps it unit-testable without a running server.

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
)

// ProjectState is the lifecycle state of a registered project.
type ProjectState string

const (
	ProjectIndexing ProjectState = "indexing"
	ProjectReady    ProjectState = "ready"
	ProjectClosing  ProjectState = "closing"
)

const stateFileVersion = 1

// Project is a single registered workspace.
type Project struct {
	Root  string
	State ProjectState
}

// ProjectSnapshot is an immutable view returned by List/Status.
type ProjectSnapshot struct {
	Root     string       `json:"root"`
	State    ProjectState `json:"state"`
	LastUsed bool         `json:"last_used"`
}

// persistedState is the on-disk shape of state.json.
type persistedState struct {
	Version     int      `json:"version"`
	LastProject string   `json:"last_project,omitempty"`
	Active      []string `json:"active"`
}

// ProjectManager is safe for concurrent use.
type ProjectManager struct {
	mu        sync.RWMutex
	projects  map[string]*Project
	lastUsed  string   // normalized root of the last-used project
	persisted []string // "active" list read from state.json on Load (not auto-warmed)
	statePath string
}

// NewProjectManager creates a manager that persists to statePath (may be empty
// to disable persistence, e.g. in tests that don't exercise it).
func NewProjectManager(statePath string) *ProjectManager {
	return &ProjectManager{
		projects:  make(map[string]*Project),
		statePath: statePath,
	}
}

// normalizeRoot canonicalizes a workspace root path for use as a map key.
func normalizeRoot(root string) string {
	r := filepath.Clean(strings.TrimSpace(root))
	// filepath.Clean already strips a trailing separator (except for root "/").
	return r
}

// isAncestorOrEqual reports whether a is an ancestor of, or equal to, b.
// Uses separator-aware prefix matching so "/p/projA" is NOT an ancestor of
// "/p/projAB".
func isAncestorOrEqual(a, b string) bool {
	if a == b {
		return true
	}
	return strings.HasPrefix(b, a+string(filepath.Separator))
}

// Add registers root. Returns (project, added, error). If the project already
// exists it is returned with added=false. Overlapping/nested roots are rejected
// (invariant: routing by URI prefix requires non-nested roots).
func (pm *ProjectManager) Add(root string) (*Project, bool, error) {
	r := normalizeRoot(root)
	if r == "" || r == "." {
		return nil, false, fmt.Errorf("empty project root")
	}

	pm.mu.Lock()
	defer pm.mu.Unlock()

	if p, ok := pm.projects[r]; ok {
		return p, false, nil
	}

	for existing := range pm.projects {
		if isAncestorOrEqual(existing, r) || isAncestorOrEqual(r, existing) {
			return nil, false, fmt.Errorf("project root %q overlaps existing root %q (nested roots are not allowed)", r, existing)
		}
	}

	p := &Project{Root: r, State: ProjectIndexing}
	pm.projects[r] = p
	pm.persistLocked()
	return p, true, nil
}

// setState transitions an existing project; no-op if unknown.
func (pm *ProjectManager) setState(root string, state ProjectState) {
	r := normalizeRoot(root)
	pm.mu.Lock()
	defer pm.mu.Unlock()
	if p, ok := pm.projects[r]; ok {
		p.State = state
	}
}

// MarkReady marks a project as fully indexed and ready.
func (pm *ProjectManager) MarkReady(root string) { pm.setState(root, ProjectReady) }

// MarkIndexing marks a project as (re)indexing.
func (pm *ProjectManager) MarkIndexing(root string) { pm.setState(root, ProjectIndexing) }

// Close removes a project from the registry. Returns true if it existed.
func (pm *ProjectManager) Close(root string) bool {
	r := normalizeRoot(root)
	pm.mu.Lock()
	defer pm.mu.Unlock()
	if _, ok := pm.projects[r]; !ok {
		return false
	}
	delete(pm.projects, r)
	if pm.lastUsed == r {
		pm.lastUsed = ""
	}
	pm.persistLocked()
	return true
}

// Get returns a snapshot of one project.
func (pm *ProjectManager) Get(root string) (ProjectSnapshot, bool) {
	r := normalizeRoot(root)
	pm.mu.RLock()
	defer pm.mu.RUnlock()
	p, ok := pm.projects[r]
	if !ok {
		return ProjectSnapshot{}, false
	}
	return ProjectSnapshot{Root: p.Root, State: p.State, LastUsed: pm.lastUsed == r}, true
}

// List returns snapshots of all registered projects, sorted by root.
func (pm *ProjectManager) List() []ProjectSnapshot {
	pm.mu.RLock()
	defer pm.mu.RUnlock()
	out := make([]ProjectSnapshot, 0, len(pm.projects))
	for _, p := range pm.projects {
		out = append(out, ProjectSnapshot{Root: p.Root, State: p.State, LastUsed: pm.lastUsed == p.Root})
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Root < out[j].Root })
	return out
}

// SetLastUsed records root as the last-used project (used for boot warm-up) and
// persists. The project does not need to be registered yet.
func (pm *ProjectManager) SetLastUsed(root string) {
	r := normalizeRoot(root)
	pm.mu.Lock()
	defer pm.mu.Unlock()
	if pm.lastUsed == r {
		return
	}
	pm.lastUsed = r
	pm.persistLocked()
}

// LastProject returns the last-used project root (from state.json or runtime).
func (pm *ProjectManager) LastProject() string {
	pm.mu.RLock()
	defer pm.mu.RUnlock()
	return pm.lastUsed
}

// PersistedActive returns the "active" list read from state.json on Load. These
// are NOT auto-warmed; the boot policy decides what to warm (default: last only).
func (pm *ProjectManager) PersistedActive() []string {
	pm.mu.RLock()
	defer pm.mu.RUnlock()
	return append([]string(nil), pm.persisted...)
}

// Load reads state.json. A missing file is not an error (fresh start).
func (pm *ProjectManager) Load() error {
	if pm.statePath == "" {
		return nil
	}
	data, err := os.ReadFile(pm.statePath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return fmt.Errorf("read state file: %w", err)
	}

	var st persistedState
	if err := json.Unmarshal(data, &st); err != nil {
		return fmt.Errorf("parse state file: %w", err)
	}

	pm.mu.Lock()
	defer pm.mu.Unlock()
	if st.LastProject != "" {
		pm.lastUsed = normalizeRoot(st.LastProject)
	}
	pm.persisted = pm.persisted[:0]
	for _, r := range st.Active {
		pm.persisted = append(pm.persisted, normalizeRoot(r))
	}
	return nil
}

// persistLocked atomically writes state.json. Caller must hold pm.mu.
func (pm *ProjectManager) persistLocked() {
	if pm.statePath == "" {
		return
	}

	active := make([]string, 0, len(pm.projects))
	for r := range pm.projects {
		active = append(active, r)
	}
	sort.Strings(active)

	st := persistedState{Version: stateFileVersion, LastProject: pm.lastUsed, Active: active}
	data, err := json.MarshalIndent(st, "", "  ")
	if err != nil {
		return
	}

	dir := filepath.Dir(pm.statePath)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return
	}
	tmp := pm.statePath + ".tmp"
	if err := os.WriteFile(tmp, data, 0o644); err != nil {
		return
	}
	_ = os.Rename(tmp, pm.statePath)
}
