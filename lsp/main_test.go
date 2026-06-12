//go:build !windows
// +build !windows

package lsp

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

// TestMain builds the bundled mock-lsp-server fixture and prepends it to PATH so
// tests that spawn the "mock-lsp-server" command can find it without requiring
// the external rockerBOO/mock-lsp-server tool to be installed. Since running
// `go test` already requires the Go toolchain, building the fixture here always
// has `go` available.
func TestMain(m *testing.M) {
	cleanup, err := buildMockLSPServerOnPath()
	if err != nil {
		fmt.Fprintf(os.Stderr, "failed to build mock-lsp-server fixture: %v\n", err)
		os.Exit(1)
	}
	defer cleanup()

	os.Exit(m.Run())
}

func buildMockLSPServerOnPath() (func(), error) {
	tmpDir, err := os.MkdirTemp("", "mock-lsp-bin")
	if err != nil {
		return nil, err
	}
	cleanup := func() { _ = os.RemoveAll(tmpDir) }

	bin := filepath.Join(tmpDir, "mock-lsp-server")
	cmd := exec.Command("go", "build", "-o", bin, "rockerboo/mcp-lsp-bridge/cmd/mock-lsp-server")
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		cleanup()
		return nil, fmt.Errorf("go build mock-lsp-server: %w", err)
	}

	if err := os.Setenv("PATH", tmpDir+string(os.PathListSeparator)+os.Getenv("PATH")); err != nil {
		cleanup()
		return nil, err
	}

	return cleanup, nil
}
