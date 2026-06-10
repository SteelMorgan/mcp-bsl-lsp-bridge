package main

import (
	"encoding/json"
	"testing"
)

// newTestSM builds a SessionManager wired for multi-project mode but with no
// underlying LSP process. Workspace-folder notifications will fail (stdin is
// nil) and are best-effort, so the registry API stays testable in isolation.
func newTestSM(t *testing.T) *SessionManager {
	t.Helper()
	sm := NewSessionManager("noop", nil, "/projects")
	sm.multiProject = true
	sm.pm = NewProjectManager("") // no persistence
	return sm
}

func callProject(t *testing.T, sm *SessionManager, method string, params map[string]interface{}) map[string]interface{} {
	t.Helper()
	var raw json.RawMessage
	if params != nil {
		b, err := json.Marshal(params)
		if err != nil {
			t.Fatalf("marshal params: %v", err)
		}
		raw = b
	}
	res, err := sm.handleProjectAPI(method, raw)
	if err != nil {
		t.Fatalf("%s: %v", method, err)
	}
	m, ok := res.(map[string]interface{})
	if !ok {
		t.Fatalf("%s: result is not a map: %T", method, res)
	}
	return m
}

func TestProjectAPI_AddListCloseStatus(t *testing.T) {
	sm := newTestSM(t)

	add := callProject(t, sm, "project/add", map[string]interface{}{"root": "/projects/projA"})
	if add["added"] != true {
		t.Errorf("project/add should report added=true, got %v", add["added"])
	}
	if add["root"] != "/projects/projA" {
		t.Errorf("unexpected root: %v", add["root"])
	}
	// A fresh add starts in the indexing state.
	if add["state"] != string(ProjectIndexing) {
		t.Errorf("new project should be indexing, got %v", add["state"])
	}
	// project/add records last_used.
	if got := sm.pm.LastProject(); got != "/projects/projA" {
		t.Errorf("last_project = %q, want /projects/projA", got)
	}

	// Re-add is idempotent.
	add2 := callProject(t, sm, "project/add", map[string]interface{}{"root": "/projects/projA"})
	if add2["added"] != false {
		t.Errorf("re-add should report added=false, got %v", add2["added"])
	}

	callProject(t, sm, "project/add", map[string]interface{}{"root": "/projects/projB"})

	list := callProject(t, sm, "project/list", nil)
	projects, ok := list["projects"].([]map[string]interface{})
	if !ok {
		t.Fatalf("project/list projects has wrong type: %T", list["projects"])
	}
	if len(projects) != 2 {
		t.Fatalf("expected 2 projects, got %d", len(projects))
	}

	// project/status for a known root.
	st := callProject(t, sm, "project/status", map[string]interface{}{"root": "/projects/projA"})
	if st["found"] != true {
		t.Errorf("status of known project should be found=true, got %v", st["found"])
	}

	// project/status for unknown root.
	stUnknown := callProject(t, sm, "project/status", map[string]interface{}{"root": "/projects/ghost"})
	if stUnknown["found"] != false {
		t.Errorf("status of unknown project should be found=false, got %v", stUnknown["found"])
	}

	// Close removes it.
	cl := callProject(t, sm, "project/close", map[string]interface{}{"root": "/projects/projB"})
	if cl["closed"] != true {
		t.Errorf("close should report closed=true, got %v", cl["closed"])
	}
	if len(sm.pm.List()) != 1 {
		t.Errorf("expected 1 project after close, got %d", len(sm.pm.List()))
	}
}

func TestProjectAPI_OverlapRejected(t *testing.T) {
	sm := newTestSM(t)
	callProject(t, sm, "project/add", map[string]interface{}{"root": "/projects/projA"})

	if _, err := sm.handleProjectAPI("project/add", json.RawMessage(`{"root":"/projects/projA/sub"}`)); err == nil {
		t.Error("nested project root should be rejected")
	}
}

func TestProjectAPI_DisabledWhenNoManager(t *testing.T) {
	sm := NewSessionManager("noop", nil, "/projects") // pm == nil
	if _, err := sm.handleProjectAPI("project/list", nil); err == nil {
		t.Error("project API should error when multi-project mode is disabled")
	}
}

func TestProjectAPI_MarkReadyVisibleInStatus(t *testing.T) {
	sm := newTestSM(t)
	callProject(t, sm, "project/add", map[string]interface{}{"root": "/projects/projA"})
	sm.pm.MarkReady("/projects/projA")
	st := callProject(t, sm, "project/status", map[string]interface{}{"root": "/projects/projA"})
	if st["state"] != string(ProjectReady) {
		t.Errorf("project should be ready after MarkReady, got %v", st["state"])
	}
}
