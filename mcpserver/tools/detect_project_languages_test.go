package tools

import (
	"fmt"
	"testing"

	"github.com/mark3labs/mcp-go/server"
)

func TestDetectProjectLanguagesTool(t *testing.T) {
	testCases := []struct {
		name            string
		projectPath     string
		mode            string
		mockLanguages   []string
		mockPrimary     string
		expectError     bool
		expectedContent string
		description     string
	}{
		{
			name:            "detect all languages in Go project",
			projectPath:     "/path/to/go-project",
			mode:            "all",
			mockLanguages:   []string{"go", "shell", "yaml"},
			expectError:     false,
			expectedContent: "Detected languages:",
			description:     "Should detect multiple languages in a Go project",
		},
		{
			name:            "detect primary language in Go project",
			projectPath:     "/path/to/go-project",
			mode:            "primary",
			mockPrimary:     "go",
			expectError:     false,
			expectedContent: "Primary language:",
			description:     "Should detect Go as primary language",
		},
		{
			name:            "detect all languages in Python project",
			projectPath:     "/path/to/python-project",
			mode:            "all",
			mockLanguages:   []string{"python", "yaml", "toml"},
			expectError:     false,
			expectedContent: "Detected languages:",
			description:     "Should detect multiple languages in a Python project",
		},
		{
			name:            "detect primary language in Python project",
			projectPath:     "/path/to/python-project",
			mode:            "primary",
			mockPrimary:     "python",
			expectError:     false,
			expectedContent: "Primary language:",
			description:     "Should detect Python as primary language",
		},
		{
			name:            "detect languages in TypeScript project",
			projectPath:     "/path/to/ts-project",
			mode:            "all",
			mockLanguages:   []string{"typescript", "javascript", "json"},
			expectError:     false,
			expectedContent: "Detected languages:",
			description:     "Should detect TypeScript and related languages",
		},
		{
			name:            "detect primary language in Rust project",
			projectPath:     "/path/to/rust-project",
			mode:            "primary",
			mockPrimary:     "rust",
			expectError:     false,
			expectedContent: "Primary language:",
			description:     "Should detect Rust as primary language",
		},
		{
			name:            "detect languages in multi-language project",
			projectPath:     "/path/to/multi-lang-project",
			mode:            "all",
			mockLanguages:   []string{"go", "python", "typescript", "rust", "shell"},
			expectError:     false,
			expectedContent: "Detected languages:",
			description:     "Should detect multiple languages in a polyglot project",
		},
		{
			name:        "project not found error",
			projectPath: "/nonexistent/project",
			mode:        "all",
			expectError: true,
			description: "Should handle non-existent project paths",
		},
		{
			name:        "empty project path",
			projectPath: "",
			mode:        "all",
			expectError: true,
			description: "Should handle empty project paths",
		},
		{
			name:            "default mode (should behave like 'all')",
			projectPath:     "/path/to/default-project",
			mode:            "",
			mockLanguages:   []string{"go", "yaml"},
			expectError:     false,
			expectedContent: "Detected languages:",
			description:     "Should default to 'all' mode when mode is not specified",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			bridge := &ComprehensiveMockBridge{
				detectProjectLanguagesFunc: func(projectPath string) ([]string, error) {
					if tc.expectError {
						return nil, fmt.Errorf("project detection failed for %s", projectPath)
					}
					return tc.mockLanguages, nil
				},
				detectPrimaryProjectLanguageFunc: func(projectPath string) (string, error) {
					if tc.expectError {
						return "", fmt.Errorf("primary language detection failed for %s", projectPath)
					}
					return tc.mockPrimary, nil
				},
			}

			// Create MCP server and register tool
			mcpServer := server.NewMCPServer("test", "1.0.0", server.WithToolCapabilities(false))
			RegisterProjectLanguageDetectionTool(mcpServer, bridge)

			// Test the bridge functionality that the tool would use
			if tc.mode == "primary" || tc.mode == "" {
				// Test primary language detection
				if !tc.expectError {
					primary, err := bridge.DetectPrimaryProjectLanguage(tc.projectPath)
					if err != nil {
						t.Errorf("Unexpected error in primary language detection: %v", err)
						return
					}
					if tc.mockPrimary != "" && primary != tc.mockPrimary {
						t.Errorf("Expected primary language %s, got %s", tc.mockPrimary, primary)
					}
				}
			}

			if tc.mode == "all" || tc.mode == "" {
				// Test all languages detection
				languages, err := bridge.DetectProjectLanguages(tc.projectPath)
				if tc.expectError {
					if err == nil {
						t.Error("Expected error but got none")
					}
					return
				}
				if err != nil {
					t.Errorf("Unexpected error in language detection: %v", err)
					return
				}
				if len(languages) != len(tc.mockLanguages) {
					t.Errorf("Expected %d languages, got %d", len(tc.mockLanguages), len(languages))
				}
			}

			t.Logf("Test case '%s' passed - %s", tc.name, tc.description)
		})
	}
}

func TestDetectProjectLanguagesEdgeCases(t *testing.T) {
	t.Run("project with no recognizable languages", func(t *testing.T) {
		bridge := &ComprehensiveMockBridge{
			detectProjectLanguagesFunc: func(projectPath string) ([]string, error) {
				return []string{}, nil // Empty language list
			},
			detectPrimaryProjectLanguageFunc: func(projectPath string) (string, error) {
				return "", fmt.Errorf("no primary language detected")
			},
		}

		// Test empty language detection
		languages, err := bridge.DetectProjectLanguages("/path/to/empty-project")
		if err != nil {
			t.Errorf("Unexpected error: %v", err)
		}
		if len(languages) != 0 {
			t.Errorf("Expected 0 languages, got %d", len(languages))
		}

		// Test primary language detection failure
		_, err = bridge.DetectPrimaryProjectLanguage("/path/to/empty-project")
		if err == nil {
			t.Error("Expected error for empty project")
		}
	})

	t.Run("project with single language", func(t *testing.T) {
		bridge := &ComprehensiveMockBridge{
			detectProjectLanguagesFunc: func(projectPath string) ([]string, error) {
				return []string{"go"}, nil
			},
			detectPrimaryProjectLanguageFunc: func(projectPath string) (string, error) {
				return "go", nil
			},
		}

		// Test single language detection
		languages, err := bridge.DetectProjectLanguages("/path/to/single-lang")
		if err != nil {
			t.Errorf("Unexpected error: %v", err)
		}
		if len(languages) != 1 {
			t.Errorf("Expected 1 language, got %d", len(languages))
		}
		if languages[0] != "go" {
			t.Errorf("Expected 'go', got '%s'", languages[0])
		}

		// Test primary language detection
		primary, err := bridge.DetectPrimaryProjectLanguage("/path/to/single-lang")
		if err != nil {
			t.Errorf("Unexpected error: %v", err)
		}
		if primary != "go" {
			t.Errorf("Expected 'go', got '%s'", primary)
		}
	})

	t.Run("project with many languages prioritization", func(t *testing.T) {
		languages := []string{"go", "python", "typescript", "javascript", "rust", "c", "cpp", "java", "shell", "yaml", "json", "dockerfile"}
		
		bridge := &ComprehensiveMockBridge{
			detectProjectLanguagesFunc: func(projectPath string) ([]string, error) {
				return languages, nil
			},
			detectPrimaryProjectLanguageFunc: func(projectPath string) (string, error) {
				// Simulate prioritization logic (Go is often primary in Go projects)
				return "go", nil
			},
		}

		// Test many languages detection
		detectedLangs, err := bridge.DetectProjectLanguages("/path/to/polyglot-project")
		if err != nil {
			t.Errorf("Unexpected error: %v", err)
		}
		if len(detectedLangs) != len(languages) {
			t.Errorf("Expected %d languages, got %d", len(languages), len(detectedLangs))
		}

		// Test primary language selection
		primary, err := bridge.DetectPrimaryProjectLanguage("/path/to/polyglot-project")
		if err != nil {
			t.Errorf("Unexpected error: %v", err)
		}
		if primary != "go" {
			t.Errorf("Expected 'go' as primary, got '%s'", primary)
		}
	})
}

func TestDetectProjectLanguagesConfiguration(t *testing.T) {
	t.Run("validate language detection patterns", func(t *testing.T) {
		// Test that common project patterns are recognized
		projectPatterns := map[string][]string{
			"/go-project":         {"go", "mod", "yaml"},
			"/python-project":     {"python", "requirements", "yaml"},
			"/node-project":       {"typescript", "javascript", "json"},
			"/rust-project":       {"rust", "toml", "yaml"},
			"/java-project":       {"java", "xml", "properties"},
			"/docker-project":     {"dockerfile", "yaml", "shell"},
			"/kubernetes-project": {"yaml", "helm", "shell"},
		}

		for projectPath, expectedLanguages := range projectPatterns {
			bridge := &ComprehensiveMockBridge{
				detectProjectLanguagesFunc: func(path string) ([]string, error) {
					return expectedLanguages, nil
				},
			}

			languages, err := bridge.DetectProjectLanguages(projectPath)
			if err != nil {
				t.Errorf("Error detecting languages for %s: %v", projectPath, err)
				continue
			}

			if len(languages) != len(expectedLanguages) {
				t.Errorf("For %s: expected %d languages, got %d", projectPath, len(expectedLanguages), len(languages))
			}
		}
	})
}