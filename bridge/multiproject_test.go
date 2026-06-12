package bridge

import (
	"path/filepath"
	"testing"
)

func TestDeriveProjectRoot(t *testing.T) {
	// Virtual filesystem: only these paths "exist".
	present := map[string]bool{
		filepath.FromSlash("/projects/projA/Configuration.xml"):         true,
		filepath.FromSlash("/projects/projB/.bsl-language-server.json"): true,
		filepath.FromSlash("/projects/projC/src/cf/Configuration.xml"):  true,
	}
	exists := func(p string) bool { return present[filepath.Clean(p)] }

	cases := []struct {
		name string
		file string
		want string
	}{
		{
			name: "designer layout (Configuration.xml at root)",
			file: filepath.FromSlash("/projects/projA/CommonModules/Foo/Ext/Module.bsl"),
			want: filepath.FromSlash("/projects/projA"),
		},
		{
			name: "explicit bsl-language-server.json marker",
			file: filepath.FromSlash("/projects/projB/src/Module.bsl"),
			want: filepath.FromSlash("/projects/projB"),
		},
		{
			name: "EDT layout (src/cf marker)",
			file: filepath.FromSlash("/projects/projC/Module.bsl"),
			want: filepath.FromSlash("/projects/projC"),
		},
		{
			name: "no marker anywhere",
			file: filepath.FromSlash("/tmp/loose/Module.bsl"),
			want: "",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := deriveProjectRoot(tc.file, exists)
			if got != tc.want {
				t.Errorf("deriveProjectRoot(%q) = %q, want %q", tc.file, got, tc.want)
			}
		})
	}
}

func TestProjectCacheRememberForget(t *testing.T) {
	b := &MCPLSPBridge{}

	root := filepath.FromSlash("/projects/projA")
	b.rememberProject(root)
	if !b.knownProjects[filepath.Clean(root)] {
		t.Fatal("rememberProject did not record the root")
	}
	if b.lastTouchedRoot != filepath.Clean(root) {
		t.Errorf("lastTouchedRoot = %q, want %q", b.lastTouchedRoot, filepath.Clean(root))
	}

	b.forgetProject(root)
	if b.knownProjects[filepath.Clean(root)] {
		t.Error("forgetProject did not remove the root")
	}
	if b.lastTouchedRoot != "" {
		t.Errorf("lastTouchedRoot should be cleared, got %q", b.lastTouchedRoot)
	}
}

// ensureProjectForURI must be a no-op (no panic) when no multi-project client is
// connected — the common single-project case.
func TestEnsureProjectForURI_NoClientIsNoop(t *testing.T) {
	b := &MCPLSPBridge{clients: nil}
	b.ensureProjectForURI("file:///projects/projA/Module.bsl") // must not panic
	if len(b.knownProjects) != 0 {
		t.Errorf("expected no projects cached, got %d", len(b.knownProjects))
	}
}
