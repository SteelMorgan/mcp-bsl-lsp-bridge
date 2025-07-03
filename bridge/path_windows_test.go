//go:build windows

package bridge

import (
	"path/filepath"
	"testing"

	"rockerboo/mcp-lsp-bridge/security"
)

func TestWindowsSpecificPaths(t *testing.T) {
	tests := []struct {
		name    string
		path    string
		want    string
		wantErr bool
	}{
		{
			name:    "C drive path",
			path:    `C:\Users\rockerboo\code\mcp-lsp-bridge`,
			want:    `C:\Users\rockerboo\code\mcp-lsp-bridge`,
			wantErr: false,
		},
		{
			name:    "D drive path",
			path:    `D:\Projects\test`,
			want:    `D:\Projects\test`,
			wantErr: false,
		},
		{
			name:    "UNC path",
			path:    `\\server\share\folder`,
			want:    `\\server\share\folder`,
			wantErr: false,
		},
		{
			name:    "path with forward slashes",
			path:    `C:/Users/rockerboo/code`,
			want:    `C:\Users\rockerboo\code`,
			wantErr: false,
		},
		{
			name:    "path with mixed separators",
			path:    `C:\Users/rockerboo\code`,
			want:    `C:\Users\rockerboo\code`,
			wantErr: false,
		},
		{
			name:    "relative path with backslashes",
			path:    `subdir\file.txt`,
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
				// Verify it's a valid Windows absolute path
				if !filepath.IsAbs(got) {
					t.Errorf("Expected absolute path, got %v", got)
				}
			}
		})
	}
}

func TestWindowsIsWithinAllowedDirectory(t *testing.T) {
	tests := []struct {
		name    string
		path    string
		baseDir string
		allowed bool
	}{
		{
			name:    "same drive subdirectory",
			path:    `C:\Users\rockerboo\code\mcp-lsp-bridge\lsp`,
			baseDir: `C:\Users\rockerboo\code\mcp-lsp-bridge`,
			allowed: true,
		},
		{
			name:    "different drive",
			path:    `D:\Users\rockerboo\code`,
			baseDir: `C:\Users\rockerboo\code`,
			allowed: false,
		},
		{
			name:    "UNC path subdirectory",
			path:    `\\server\share\project\subdir`,
			baseDir: `\\server\share\project`,
			allowed: true,
		},
		{
			name:    "different UNC server",
			path:    `\\server2\share\project`,
			baseDir: `\\server1\share\project`,
			allowed: false,
		},
		{
			name:    "case insensitive (Windows)",
			path:    `C:\USERS\ROCKERBOO\CODE`,
			baseDir: `c:\users\rockerboo\code`,
			allowed: true,
		},
		{
			name:    "parent directory escape attempt",
			path:    `C:\Users\rockerboo\code\mcp-lsp-bridge\..\..`,
			baseDir: `C:\Users\rockerboo\code\mcp-lsp-bridge`,
			allowed: false,
		},
		{
			name:    "forward slash in Windows path",
			path:    `C:/Users/rockerboo/code/mcp-lsp-bridge/lsp`,
			baseDir: `C:\Users\rockerboo\code\mcp-lsp-bridge`,
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

func TestWindowsDriveLetters(t *testing.T) {
	tests := []struct {
		name     string
		path     string
		wantVol  string
	}{
		{
			name:    "C drive",
			path:    `C:\path\to\file`,
			wantVol: `C:`,
		},
		{
			name:    "D drive",
			path:    `D:\another\path`,
			wantVol: `D:`,
		},
		{
			name:    "UNC path",
			path:    `\\server\share\path`,
			wantVol: `\\server\share`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			vol := filepath.VolumeName(tt.path)
			if vol != tt.wantVol {
				t.Errorf("filepath.VolumeName(%s) = %v, want %v", tt.path, vol, tt.wantVol)
			}
		})
	}
}
