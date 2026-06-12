package main

import (
	"path/filepath"
	"testing"
)

func TestProjectManager_AddListAndState(t *testing.T) {
	pm := NewProjectManager("")

	p, added, err := pm.Add("/projects/projA")
	if err != nil || !added {
		t.Fatalf("Add A: added=%v err=%v", added, err)
	}
	if p.State != ProjectIndexing {
		t.Errorf("new project should be indexing, got %s", p.State)
	}

	// Re-add is idempotent.
	if _, added, _ := pm.Add("/projects/projA"); added {
		t.Error("re-Add should report added=false")
	}

	if _, _, err := pm.Add("/projects/projB"); err != nil {
		t.Fatalf("Add B: %v", err)
	}

	pm.MarkReady("/projects/projA")
	if snap, ok := pm.Get("/projects/projA"); !ok || snap.State != ProjectReady {
		t.Errorf("A should be ready, got %+v ok=%v", snap, ok)
	}

	list := pm.List()
	if len(list) != 2 {
		t.Fatalf("expected 2 projects, got %d", len(list))
	}
	if list[0].Root != "/projects/projA" || list[1].Root != "/projects/projB" {
		t.Errorf("list not sorted by root: %+v", list)
	}

	if !pm.Close("/projects/projB") {
		t.Error("Close B should return true")
	}
	if pm.Close("/projects/projB") {
		t.Error("second Close B should return false")
	}
	if len(pm.List()) != 1 {
		t.Errorf("expected 1 project after close, got %d", len(pm.List()))
	}
}

func TestProjectManager_OverlapRejected(t *testing.T) {
	pm := NewProjectManager("")

	if _, _, err := pm.Add("/projects/projA"); err != nil {
		t.Fatalf("Add A: %v", err)
	}

	// Nested under an existing root.
	if _, _, err := pm.Add("/projects/projA/sub"); err == nil {
		t.Error("nested root under existing should be rejected")
	}

	// Existing root nested under a new (ancestor) root.
	if _, _, err := pm.Add("/projects"); err == nil {
		t.Error("ancestor root of existing should be rejected")
	}

	// Sibling with shared name prefix must be allowed (not a real nesting).
	if _, _, err := pm.Add("/projects/projAB"); err != nil {
		t.Errorf("sibling projAB should be allowed, got %v", err)
	}
}

func TestProjectManager_Normalization(t *testing.T) {
	pm := NewProjectManager("")
	if _, added, err := pm.Add("/projects/projA/"); err != nil || !added {
		t.Fatalf("Add with trailing slash: added=%v err=%v", added, err)
	}
	// Same project via non-normalized path is idempotent.
	if _, added, _ := pm.Add("/projects/./projA"); added {
		t.Error("normalized duplicate should not be added again")
	}
}

func TestProjectManager_PersistRoundTrip(t *testing.T) {
	statePath := filepath.Join(t.TempDir(), "sub", "state.json")

	pm := NewProjectManager(statePath)
	if _, _, err := pm.Add("/projects/projA"); err != nil {
		t.Fatalf("Add A: %v", err)
	}
	if _, _, err := pm.Add("/projects/projB"); err != nil {
		t.Fatalf("Add B: %v", err)
	}
	pm.SetLastUsed("/projects/projA")

	// New manager loads persisted state.
	pm2 := NewProjectManager(statePath)
	if err := pm2.Load(); err != nil {
		t.Fatalf("Load: %v", err)
	}
	if got := pm2.LastProject(); got != "/projects/projA" {
		t.Errorf("LastProject after load = %q, want /projects/projA", got)
	}
	active := pm2.PersistedActive()
	if len(active) != 2 {
		t.Fatalf("PersistedActive = %v, want 2 entries", active)
	}
	// Loaded manager has nothing warm yet (server restarted).
	if len(pm2.List()) != 0 {
		t.Errorf("loaded manager should have no warm projects, got %d", len(pm2.List()))
	}
}

func TestProjectManager_LoadMissingFileIsOK(t *testing.T) {
	pm := NewProjectManager(filepath.Join(t.TempDir(), "nope.json"))
	if err := pm.Load(); err != nil {
		t.Errorf("Load of missing file should not error, got %v", err)
	}
	if pm.LastProject() != "" {
		t.Error("LastProject should be empty on fresh start")
	}
}
