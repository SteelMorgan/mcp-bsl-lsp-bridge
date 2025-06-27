package lsp

import (
	"os"
	"path/filepath"
	"slices"
	"testing"
)

// Helper function to create a temporary project directory with specific files
func createTempProjectWithFiles(t *testing.T, files map[string]string) string {
	t.Helper()

	// Create a temporary directory
	tempDir, err := os.MkdirTemp("", "project-language-detection-*")
	if err != nil {
		t.Fatalf("Failed to create temp directory: %v", err)
	}

	// Create files in the temp directory
	for filename, content := range files {
		filePath := filepath.Join(tempDir, filename)

		err := os.WriteFile(filePath, []byte(content), 0600)
		if err != nil {
			t.Fatalf("Failed to create file %s: %v", filename, err)
		}
	}

	return tempDir
}

// TestDetectProjectLanguages tests the project language detection method
func TestDetectProjectLanguages(t *testing.T) {
	// Create a test configuration
	config := &LSPServerConfig{
		LanguageServers: map[Language]LanguageServerConfig{
			"go": {
				Filetypes: []string{".go"},
			},
			"python": {
				Filetypes: []string{".py"},
			},
			"typescript": {
				Filetypes: []string{".ts", ".js"},
			},
		},
		ExtensionLanguageMap: map[string]Language{
			".go": "go",
			".py": "python",
			".ts": "typescript",
			".js": "typescript",
		},
	}

	testCases := []struct {
		name          string
		projectFiles  map[string]string
		expectedLangs []string
	}{
		{
			name: "Go Project",
			projectFiles: map[string]string{
				"go.mod":  "module example.com/myproject\n",
				"main.go": "package main\n\nfunc main() {}\n",
			},
			expectedLangs: []string{"go"},
		},
		{
			name: "Python Project",
			projectFiles: map[string]string{
				"pyproject.toml": "[tool.poetry]\nname = \"myproject\"\n",
				"main.py":        "def main():\n    pass\n",
			},
			expectedLangs: []string{"python"},
		},
		{
			name: "Mixed Project",
			projectFiles: map[string]string{
				"go.mod":    "module example.com/myproject\n",
				"main.go":   "package main\n\nfunc main() {}\n",
				"index.ts":  "const x = 42;\n",
				"script.py": "def hello():\n    print('world')\n",
			},
			expectedLangs: []string{"go", "typescript", "python"},
		},
		{
			name: "No Recognized Languages",
			projectFiles: map[string]string{
				"README.md":   "# My Project\n",
				"config.json": "{\"key\": \"value\"}\n",
			},
			expectedLangs: []string{},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			// Create a temporary project with the specified files
			tempDir := createTempProjectWithFiles(t, tc.projectFiles)
			defer cleanupDir(t, tempDir)

			// Detect project languages
			languages, err := config.DetectProjectLanguages(tempDir)

			// Check for expected error or languages
			if len(tc.expectedLangs) == 0 {
				if err == nil {
					t.Errorf("Expected an error for project with no recognized languages, got none")
				}
			} else {
				if err != nil {
					t.Fatalf("Unexpected error: %v", err)
				}

				// Check if detected languages match expected
				if len(languages) != len(tc.expectedLangs) {
					t.Errorf("Expected %d languages, got %d", len(tc.expectedLangs), len(languages))
				}

				// Check each expected language is in the detected languages
				for _, expectedLang := range tc.expectedLangs {
					found := slices.Contains(languages, expectedLang)
					if !found {
						t.Errorf("Expected language %s not found in detected languages", expectedLang)
					}
				}
			}
		})
	}
}

// TestDetectPrimaryProjectLanguage tests the primary language detection method
func TestDetectPrimaryProjectLanguage(t *testing.T) {
	// Create a test configuration
	config := &LSPServerConfig{
		LanguageServers: map[Language]LanguageServerConfig{
			"go": {
				Filetypes: []string{".go"},
			},
			"python": {
				Filetypes: []string{".py"},
			},
			"typescript": {
				Filetypes: []string{".ts", ".js"},
			},
		},
		ExtensionLanguageMap: map[string]Language{
			".go": "go",
			".py": "python",
			".ts": "typescript",
			".js": "typescript",
		},
	}

	testCases := []struct {
		name            string
		projectFiles    map[string]string
		expectedPrimary string
	}{
		{
			name: "Go Project",
			projectFiles: map[string]string{
				"go.mod":  "module example.com/myproject\n",
				"main.go": "package main\n\nfunc main() {}\n",
			},
			expectedPrimary: "go",
		},
		{
			name: "Mixed Project with Precedence",
			projectFiles: map[string]string{
				"go.mod":    "module example.com/myproject\n",
				"main.go":   "package main\n\nfunc main() {}\n",
				"index.ts":  "const x = 42;\n",
				"script.py": "def hello():\n    print('world')\n",
			},
			expectedPrimary: "go",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			// Create a temporary project with the specified files
			tempDir := createTempProjectWithFiles(t, tc.projectFiles)
			defer cleanupDir(t, tempDir)

			// Detect primary project language
			primaryLang, err := config.DetectPrimaryProjectLanguage(tempDir)

			// Check for expected primary language
			if err != nil {
				t.Fatalf("Unexpected error: %v", err)
			}

			if primaryLang != tc.expectedPrimary {
				t.Errorf("Expected primary language %s, got %s", tc.expectedPrimary, primaryLang)
			}
		})
	}
}

func cleanupDir(t *testing.T, dir string) func() {
	return func() {
		if err := os.RemoveAll(dir); err != nil {
			t.Logf("Failed to remove temp dir: %v", err)
		}
	}
}
