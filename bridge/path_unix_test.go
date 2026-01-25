//go:build !windows

package bridge

import (
	"path/filepath"
	"testing"

	"rockerboo/mcp-lsp-bridge/security"
)

func TestUnixSpecificPaths(t *testing.T) {
	tests := []struct {
		name    string
		path    string
		want    string
		wantErr bool
	}{
		{
			name:    "absolute path",
			path:    "/path/to/mcp-lsp-bridge",
			want:    "/path/to/mcp-lsp-bridge",
			wantErr: false,
		},
		{
			name:    "root path",
			path:    "/",
			want:    "/",
			wantErr: false,
		},
		{
			name:    "path with spaces",
			path:    "/home/user name/with spaces",
			want:    "/home/user name/with spaces",
			wantErr: false,
		},
		{
			name:    "path with special chars",
			path:    "/home/user/special-chars_123",
			want:    "/home/user/special-chars_123",
			wantErr: false,
		},
		{
			name:    "tmp directory",
			path:    "/tmp/test",
			want:    "/tmp/test",
			wantErr: false,
		},
		{
			name:    "relative path with forward slashes",
			path:    "subdir/file.txt",
			want:    "", // Will be current dir + path
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := security.GetCleanAbsPath(tt.path)
			if (err != nil) != tt.wantErr {
				t.Errorf("security.GetCleanAbsPath() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			if !tt.wantErr {
				if tt.want != "" && got != tt.want {
					t.Errorf("security.GetCleanAbsPath() = %v, want %v", got, tt.want)
				}
				// Verify it's a valid Unix absolute path
				if !filepath.IsAbs(got) {
					t.Errorf("Expected absolute path, got %v", got)
				}
				// Unix paths should start with /
				if got[0] != '/' {
					t.Errorf("Expected Unix path to start with /, got %v", got)
				}
			}
		})
	}
}

func TestUnixIsWithinAllowedDirectory(t *testing.T) {
	tests := []struct {
		name    string
		path    string
		baseDir string
		allowed bool
	}{
		{
			name:    "subdirectory",
			path:    "/path/to/mcp-lsp-bridge/lsp",
			baseDir: "/path/to/mcp-lsp-bridge",
			allowed: true,
		},
		{
			name:    "different root directory",
			path:    "/etc/config",
			baseDir: "/path/to",
			allowed: false,
		},
		{
			name:    "case sensitive (Unix)",
			path:    "/PATH/TO",
			baseDir: "/path/to",
			allowed: false,
		},
		{
			name:    "parent directory escape attempt",
			path:    "/path/to/mcp-lsp-bridge/../..",
			baseDir: "/path/to/mcp-lsp-bridge",
			allowed: false,
		},
		{
			name:    "symlink-like path (resolved by filepath.Abs)",
			path:    "/path/to/mcp-lsp-bridge/./lsp",
			baseDir: "/path/to/mcp-lsp-bridge",
			allowed: true,
		},
		{
			name:    "root directory",
			path:    "/",
			baseDir: "/home",
			allowed: false,
		},
		{
			name:    "tmp subdirectory",
			path:    "/tmp/project/subdir",
			baseDir: "/tmp/project",
			allowed: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := security.IsWithinAllowedDirectory(tt.path, tt.baseDir)
			if result != tt.allowed {
				t.Errorf("security.IsWithinAllowedDirectory(%s, %s) = %v, want %v",
					tt.path, tt.baseDir, result, tt.allowed)
			}
		})
	}
}

func TestUnixPermissions(t *testing.T) {
	// Test that we can handle paths even if they don't exist
	tests := []struct {
		name    string
		path    string
		baseDir string
		allowed bool
	}{
		{
			name:    "non-existent subdirectory",
			path:    "/path/to/nonexistent/subdir",
			baseDir: "/path/to",
			allowed: true,
		},
		{
			name:    "non-existent parent attempt",
			path:    "/path/to/../../../etc",
			baseDir: "/path/to",
			allowed: false, // /etc is NOT within /path/to
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := security.IsWithinAllowedDirectory(tt.path, tt.baseDir)
			if result != tt.allowed {
				t.Errorf("security.IsWithinAllowedDirectory(%s, %s) = %v, want %v",
					tt.path, tt.baseDir, result, tt.allowed)
			}
		})
	}
}

func TestUnixSymlinks(t *testing.T) {
	// Create a temporary directory for symlink testing
	tempDir := t.TempDir()

	// Create subdirectories
	allowedDir := filepath.Join(tempDir, "allowed")
	outsideDir := filepath.Join(tempDir, "outside")

	tests := []struct {
		name    string
		path    string
		baseDir string
		allowed bool
	}{
		{
			name:    "within temp allowed dir",
			path:    filepath.Join(allowedDir, "subdir"),
			baseDir: allowedDir,
			allowed: true,
		},
		{
			name:    "outside temp dir",
			path:    outsideDir,
			baseDir: allowedDir,
			allowed: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := security.IsWithinAllowedDirectory(tt.path, tt.baseDir)
			if result != tt.allowed {
				t.Errorf("security.IsWithinAllowedDirectory(%s, %s) = %v, want %v",
					tt.path, tt.baseDir, result, tt.allowed)
			}
		})
	}
}
