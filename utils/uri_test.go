package utils

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestNormalizeURI(t *testing.T) {
	tmp := t.TempDir()
	absFile := filepath.Join(tmp, "file.go")
	absURI, err := PathToFileURI(absFile)
	if err != nil {
		t.Fatalf("PathToFileURI failed: %v", err)
	}

	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "already normalized file URI",
			input:    absURI,
			expected: absURI,
		},
		{
			name:     "http URI unchanged",
			input:    "https://example.com/file",
			expected: "https://example.com/file",
		},
		{
			name:     "absolute path",
			input:    absFile,
			expected: absURI,
		},
		{
			name:     "relative path becomes absolute",
			input:    "file.go",
			expected: mustFileURI("file.go"),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := NormalizeURI(tt.input)
			if result != tt.expected {
				t.Errorf("NormalizeURI(%q) = %q, want %q", tt.input, result, tt.expected)
			}
		})
	}
}

func TestURIToFilePath(t *testing.T) {
	tmp := t.TempDir()
	absFile := filepath.Join(tmp, "file.go")
	absURI, err := PathToFileURI(absFile)
	if err != nil {
		t.Fatalf("PathToFileURI failed: %v", err)
	}

	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "file URI",
			input:    absURI,
			expected: absFile,
		},
		{
			name:     "already a file path",
			input:    absFile,
			expected: absFile,
		},
		{
			name:     "http URI unchanged",
			input:    "https://example.com/file",
			expected: "https://example.com/file",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := URIToFilePath(tt.input)
			if result != tt.expected {
				t.Errorf("URIToFilePath(%q) = %q, want %q", tt.input, result, tt.expected)
			}
		})
	}
}

func TestFilePathToURI(t *testing.T) {
	tmp := t.TempDir()
	absFile := filepath.Join(tmp, "file.go")
	absURI, err := PathToFileURI(absFile)
	if err != nil {
		t.Fatalf("PathToFileURI failed: %v", err)
	}

	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "absolute path",
			input:    absFile,
			expected: absURI,
		},
		{
			name:     "already a URI",
			input:    absURI,
			expected: absURI,
		},
		{
			name:     "relative path becomes absolute",
			input:    "file.go",
			expected: mustFileURI("file.go"),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := FilePathToURI(tt.input)
			if result != tt.expected {
				t.Errorf("FilePathToURI(%q) = %q, want %q", tt.input, result, tt.expected)
			}
		})
	}
}

func TestRoundTrip(t *testing.T) {
	tmp := t.TempDir()
	testPaths := []string{
		filepath.Join(tmp, "file.go"),
		filepath.Join(tmp, "test.txt"),
		filepath.Join(tmp, "app.log"),
	}

	for _, path := range testPaths {
		t.Run(path, func(t *testing.T) {
			// Convert to URI and back
			uri := FilePathToURI(path)
			resultPath := URIToFilePath(uri)

			// Compare absolute cleaned paths to avoid platform differences.
			wantAbs, _ := filepath.Abs(path)
			gotAbs, _ := filepath.Abs(resultPath)
			wantAbs = filepath.Clean(wantAbs)
			gotAbs = filepath.Clean(gotAbs)
			if gotAbs != wantAbs {
				t.Errorf("Round trip failed: %s -> %s -> %s (want %s)", path, uri, resultPath, wantAbs)
			}

			// Normalize the URI
			normalizedURI := NormalizeURI(path)
			if !strings.HasPrefix(normalizedURI, "file://") {
				t.Errorf("NormalizeURI(%s) = %s, should start with file://", path, normalizedURI)
			}
		})
	}
}

// mustAbs is a helper that calls filepath.Abs and panics on error (for tests only)
func mustAbs(path string) string {
	abs, err := filepath.Abs(path)
	if err != nil {
		panic(err)
	}
	return abs
}

func mustFileURI(path string) string {
	abs := mustAbs(path)
	u, err := PathToFileURI(abs)
	if err != nil {
		panic(err)
	}
	return u
}

func TestFileURIToPath_WithSpaces(t *testing.T) {
	// Ensures percent-encoded spaces are decoded properly.
	tmp := t.TempDir()
	p := filepath.Join(tmp, "dir with space", "file.go")
	requireDir := filepath.Dir(p)
	if err := os.MkdirAll(requireDir, 0750); err != nil {
		t.Fatalf("MkdirAll failed: %v", err)
	}

	u, err := PathToFileURI(p)
	if err != nil {
		t.Fatalf("PathToFileURI failed: %v", err)
	}

	got, err := FileURIToPath(u)
	if err != nil {
		t.Fatalf("FileURIToPath failed: %v", err)
	}

	want := filepath.Clean(p)
	got = filepath.Clean(got)
	if got != want {
		t.Fatalf("FileURIToPath(%q) = %q, want %q", u, got, want)
	}
}
