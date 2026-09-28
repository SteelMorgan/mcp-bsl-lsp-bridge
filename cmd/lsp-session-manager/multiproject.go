package main

// Multi-project wiring for the daemon.
//
// In multi-project mode (MULTI_PROJECT=1) the daemon keeps a single BSL LS
// process and registers several 1C configurations as workspace folders. BSL LS
// routes each document to the right ServerContext by URI prefix, so file-level
// requests need no project parameter. This file drives the LSP side
// (workspace/didChangeWorkspaceFolders) and keeps the ProjectManager registry
// in sync. The registry itself lives in projectmanager.go and is pure state.
//
// Warm-up attribution: BSL LS $/progress notifications do not carry a workspace
// name, so we serialize warm-ups (one project indexed at a time) and attribute
// the indexing window between "add P" and "indexing settled" to P. See the
// design doc, section 6.

import (
	"encoding/json"
	"fmt"
	"log"
	"path/filepath"
	"time"

	"rockerboo/mcp-lsp-bridge/utils"
)

// EnableMultiProject turns on multi-project mode and loads persisted state.
// Must be called before Start() so initialize() takes the multi-project path.
func (sm *SessionManager) EnableMultiProject(statePath string) error {
	sm.multiProject = true
	sm.pm = NewProjectManager(statePath)
	return sm.pm.Load()
}

// warmupBoot warms only the last-used project after a (re)start. Everything
// else is added lazily on first use by the bridge.
func (sm *SessionManager) warmupBoot() {
	last := sm.pm.LastProject()
	if last == "" {
		log.Println("multi-project: no last-used project to warm on boot")
		return
	}
	log.Printf("multi-project: warming last-used project %s on boot", last)
	if _, _, err := sm.pm.Add(last); err != nil {
		log.Printf("multi-project: cannot warm last project %s: %v", last, err)
		return
	}
	sm.warmupProject(last)
}

// warmupProject adds a project as a workspace folder and waits for its indexing
// to settle, then marks it ready. Serialized via warmupMu so that at most one
// project indexes at a time, which is what makes $/progress attributable.
func (sm *SessionManager) warmupProject(root string) {
	sm.warmupMu.Lock()
	defer sm.warmupMu.Unlock()

	sm.setWarming(root)
	defer sm.setWarming("")

	if err := sm.addWorkspaceFolderNotify(root); err != nil {
		log.Printf("warmup: failed to add workspace folder %s: %v", root, err)
		return
	}
	log.Printf("warmup: indexing project %s ...", root)
	sm.waitForIndexingIdle(10*time.Second, 30*time.Minute)
	sm.pm.MarkReady(root)
	sm.enqueueRLMEnsure(root)
	log.Printf("warmup: project %s is ready", root)
}

// waitForIndexingIdle blocks until BSL LS indexing has begun and then settled.
// graceBegin bounds how long we wait for indexing to start (the server is async
// and may emit no progress at all for a tiny project); maxWait bounds the total.
func (sm *SessionManager) waitForIndexingIdle(graceBegin, maxWait time.Duration) {
	start := time.Now()

	// Phase A: wait for indexing to actually begin.
	graceDeadline := start.Add(graceBegin)
	for time.Now().Before(graceDeadline) {
		if sm.IsIndexing() {
			break
		}
		time.Sleep(250 * time.Millisecond)
	}

	// Phase B: wait until indexing is inactive and has been quiet for a moment.
	deadline := start.Add(maxWait)
	for time.Now().Before(deadline) {
		sm.indexingMu.RLock()
		active := sm.indexingActive
		last := sm.indexingLastUpdate
		sm.indexingMu.RUnlock()
		if !active && (last.IsZero() || time.Since(last) > 2*time.Second) {
			return
		}
		time.Sleep(500 * time.Millisecond)
	}
	log.Printf("warmup: indexing wait timed out after %s", maxWait)
}

// addWorkspaceFolderNotify registers root with BSL LS at runtime.
func (sm *SessionManager) addWorkspaceFolderNotify(root string) error {
	return sm.sendWorkspaceFoldersChange([]string{root}, nil)
}

// removeWorkspaceFolderNotify unregisters root from BSL LS at runtime.
func (sm *SessionManager) removeWorkspaceFolderNotify(root string) error {
	return sm.sendWorkspaceFoldersChange(nil, []string{root})
}

// sendWorkspaceFoldersChange sends a workspace/didChangeWorkspaceFolders LSP
// notification with the given added/removed roots.
func (sm *SessionManager) sendWorkspaceFoldersChange(added, removed []string) error {
	toFolders := func(roots []string) []map[string]string {
		out := make([]map[string]string, 0, len(roots))
		for _, r := range roots {
			out = append(out, map[string]string{
				"uri":  utils.FilePathToURI(r),
				"name": filepath.Base(r),
			})
		}
		return out
	}
	params := map[string]interface{}{
		"event": map[string]interface{}{
			"added":   toFolders(added),
			"removed": toFolders(removed),
		},
	}
	return sm.sendNotification("workspace/didChangeWorkspaceFolders", params)
}

// setWarming records the project currently being warmed (diagnostics only).
func (sm *SessionManager) setWarming(root string) {
	sm.warmingMu.Lock()
	sm.warming = root
	sm.warmingMu.Unlock()
}

// handleProjectAPI services the project/* daemon API methods.
func (sm *SessionManager) handleProjectAPI(method string, params json.RawMessage) (interface{}, error) {
	if sm.pm == nil {
		return nil, fmt.Errorf("multi-project mode is disabled (set MULTI_PROJECT=1)")
	}

	var req struct {
		Root string `json:"root"`
	}
	if len(params) > 0 {
		_ = json.Unmarshal(params, &req)
	}

	switch method {
	case "project/add":
		if req.Root == "" {
			return nil, fmt.Errorf("project/add requires 'root'")
		}
		p, added, err := sm.pm.Add(req.Root)
		if err != nil {
			return nil, err
		}
		sm.pm.SetLastUsed(p.Root)
		if added {
			go sm.warmupProject(p.Root)
		}
		snap, _ := sm.pm.Get(p.Root)
		return projectResult(snap, map[string]interface{}{"added": added}), nil

	case "project/close":
		if req.Root == "" {
			return nil, fmt.Errorf("project/close requires 'root'")
		}
		// Best-effort: tell BSL LS to drop the folder before forgetting it.
		if err := sm.removeWorkspaceFolderNotify(req.Root); err != nil {
			log.Printf("project/close: failed to notify server for %s: %v", req.Root, err)
		}
		closed := sm.pm.Close(req.Root)
		return map[string]interface{}{"root": normalizeRoot(req.Root), "closed": closed}, nil

	case "project/status":
		if req.Root != "" {
			snap, ok := sm.pm.Get(req.Root)
			if !ok {
				return map[string]interface{}{"root": normalizeRoot(req.Root), "found": false}, nil
			}
			return projectResult(snap, map[string]interface{}{"found": true}), nil
		}
		fallthrough

	case "project/list":
		list := sm.pm.List()
		out := make([]map[string]interface{}, 0, len(list))
		for _, p := range list {
			out = append(out, projectResult(p, nil))
		}
		return map[string]interface{}{
			"projects":     out,
			"last_project": sm.pm.LastProject(),
		}, nil
	}

	return nil, fmt.Errorf("unknown project method: %s", method)
}

// projectResult builds a JSON-friendly project record, merging extra fields.
func projectResult(snap ProjectSnapshot, extra map[string]interface{}) map[string]interface{} {
	m := map[string]interface{}{
		"root":      snap.Root,
		"state":     string(snap.State),
		"last_used": snap.LastUsed,
	}
	for k, v := range extra {
		m[k] = v
	}
	return m
}
