package utils

import (
	"os"
	"testing"
)

func TestNewDockerPathMapper(t *testing.T) {
	tests := []struct {
		name          string
		hostRoot      string
		containerRoot string
		expectError   bool
	}{
		{
			name:          "valid paths",
			hostRoot:      "D:/My Projects/Projects 1C",
			containerRoot: "/projects",
			expectError:   false,
		},
		{
			name:          "empty host root",
			hostRoot:      "",
			containerRoot: "/projects",
			expectError:   true,
		},
		{
			name:          "empty container root",
			hostRoot:      "D:/My Projects/Projects 1C",
			containerRoot: "",
			expectError:   true,
		},
		{
			name:          "relative container root",
			hostRoot:      "D:/My Projects/Projects 1C",
			containerRoot: "projects",
			expectError:   true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mapper, err := NewDockerPathMapper(tt.hostRoot, tt.containerRoot)
			if tt.expectError {
				if err == nil {
					t.Errorf("Expected error but got none")
				}
				return
			}
			if err != nil {
				t.Errorf("Unexpected error: %v", err)
				return
			}
			if mapper == nil {
				t.Errorf("Expected mapper but got nil")
			}
		})
	}
}

func TestNewDockerPathMapperFromEnv(t *testing.T) {
	// Save original environment
	originalHostRoot := os.Getenv("HOST_PROJECTS_ROOT")
	originalProjectsHostRoot := os.Getenv("PROJECTS_HOST_ROOT")
	originalProjectsRoot := os.Getenv("PROJECTS_ROOT")

	// Clean up after test
	defer func() {
		if originalHostRoot != "" {
			os.Setenv("HOST_PROJECTS_ROOT", originalHostRoot)
		} else {
			os.Unsetenv("HOST_PROJECTS_ROOT")
		}
		if originalProjectsHostRoot != "" {
			os.Setenv("PROJECTS_HOST_ROOT", originalProjectsHostRoot)
		} else {
			os.Unsetenv("PROJECTS_HOST_ROOT")
		}
		if originalProjectsRoot != "" {
			os.Setenv("PROJECTS_ROOT", originalProjectsRoot)
		} else {
			os.Unsetenv("PROJECTS_ROOT")
		}
	}()

	tests := []struct {
		name           string
		hostRootEnv    string
		projectsRootEnv string
		expectEnabled  bool
	}{
		{
			name:           "enabled with HOST_PROJECTS_ROOT",
			hostRootEnv:    "D:/My Projects/Projects 1C",
			projectsRootEnv: "/projects",
			expectEnabled:  true,
		},
		{
			name:           "enabled with PROJECTS_HOST_ROOT",
			hostRootEnv:    "",
			projectsRootEnv: "/projects",
			expectEnabled:  false,
		},
		{
			name:           "disabled without host root",
			hostRootEnv:    "",
			projectsRootEnv: "/projects",
			expectEnabled:  false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Set environment variables
			if tt.hostRootEnv != "" {
				os.Setenv("HOST_PROJECTS_ROOT", tt.hostRootEnv)
			} else {
				os.Unsetenv("HOST_PROJECTS_ROOT")
			}
			os.Unsetenv("PROJECTS_HOST_ROOT")
			if tt.projectsRootEnv != "" {
				os.Setenv("PROJECTS_ROOT", tt.projectsRootEnv)
			} else {
				os.Unsetenv("PROJECTS_ROOT")
			}

			mapper, err := NewDockerPathMapperFromEnv()
			if err != nil {
				t.Errorf("Unexpected error: %v", err)
				return
			}

			if mapper.IsEnabled() != tt.expectEnabled {
				t.Errorf("Expected enabled=%v, got %v", tt.expectEnabled, mapper.IsEnabled())
			}
		})
	}
}

func TestHostToContainer(t *testing.T) {
	mapper, err := NewDockerPathMapper("D:/My Projects/Projects 1C", "/projects")
	if err != nil {
		t.Fatalf("Failed to create mapper: %v", err)
	}

	tests := []struct {
		name        string
		hostPath    string
		expected    string
		expectError bool
	}{
		{
			name:        "root directory",
			hostPath:    "D:/My Projects/Projects 1C",
			expected:    "/projects",
			expectError: false,
		},
		{
			name:        "subdirectory",
			hostPath:    "D:/My Projects/Projects 1C/temp",
			expected:    "/projects/temp",
			expectError: false,
		},
		{
			name:        "file path",
			hostPath:    "D:/My Projects/Projects 1C/temp/file.bsl",
			expected:    "/projects/temp/file.bsl",
			expectError: false,
		},
		{
			name:        "path with spaces",
			hostPath:    "D:/My Projects/Projects 1C/GBIG Portfolio asset management",
			expected:    "/projects/GBIG Portfolio asset management",
			expectError: false,
		},
		{
			name:        "path outside root",
			hostPath:    "D:/Other/Projects",
			expected:    "",
			expectError: true,
		},
		{
			name:        "empty path",
			hostPath:    "",
			expected:    "",
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := mapper.HostToContainer(tt.hostPath)
			if tt.expectError {
				if err == nil {
					t.Errorf("Expected error but got none")
				}
				return
			}
			if err != nil {
				t.Errorf("Unexpected error: %v", err)
				return
			}
			if result != tt.expected {
				t.Errorf("Expected %s, got %s", tt.expected, result)
			}
		})
	}
}

func TestContainerToHost(t *testing.T) {
	mapper, err := NewDockerPathMapper("D:/My Projects/Projects 1C", "/projects")
	if err != nil {
		t.Fatalf("Failed to create mapper: %v", err)
	}

	tests := []struct {
		name          string
		containerPath string
		expected      string
		expectError   bool
	}{
		{
			name:          "root directory",
			containerPath: "/projects",
			expected:      "D:/My Projects/Projects 1C",
			expectError:   false,
		},
		{
			name:          "subdirectory",
			containerPath: "/projects/temp",
			expected:      "D:/My Projects/Projects 1C/temp",
			expectError:   false,
		},
		{
			name:          "file path",
			containerPath: "/projects/temp/file.bsl",
			expected:      "D:/My Projects/Projects 1C/temp/file.bsl",
			expectError:   false,
		},
		{
			name:          "path outside root",
			containerPath: "/other/projects",
			expected:      "",
			expectError:   true,
		},
		{
			name:          "empty path",
			containerPath: "",
			expected:      "",
			expectError:   true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := mapper.ContainerToHost(tt.containerPath)
			if tt.expectError {
				if err == nil {
					t.Errorf("Expected error but got none")
				}
				return
			}
			if err != nil {
				t.Errorf("Unexpected error: %v", err)
				return
			}
			if result != tt.expected {
				t.Errorf("Expected %s, got %s", tt.expected, result)
			}
		})
	}
}

func TestDockerPathMapperNormalizeURI(t *testing.T) {
	mapper, err := NewDockerPathMapper("D:/My Projects/Projects 1C", "/projects")
	if err != nil {
		t.Fatalf("Failed to create mapper: %v", err)
	}

	tests := []struct {
		name        string
		uri         string
		expected    string
		expectError bool
	}{
		{
			name:        "file URI with subdirectory",
			uri:         "file:///D:/My Projects/Projects 1C/temp",
			expected:    "file:///projects/temp",
			expectError: false,
		},
		{
			name:        "file URI with file",
			uri:         "file:///D:/My Projects/Projects 1C/temp/file.bsl",
			expected:    "file:///projects/temp/file.bsl",
			expectError: false,
		},
		{
			name:        "file URI with Windows path format",
			uri:         "file:///D:/My Projects/Projects 1C/temp",
			expected:    "file:///projects/temp",
			expectError: false,
		},
		{
			name:        "path without file:// prefix",
			uri:         "D:/My Projects/Projects 1C/temp",
			expected:    "file:///projects/temp",
			expectError: false,
		},
		{
			name:        "path outside root",
			uri:         "file:///D:/Other/Projects",
			expected:    "",
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := mapper.NormalizeURI(tt.uri)
			if tt.expectError {
				if err == nil {
					t.Errorf("Expected error but got none")
				}
				return
			}
			if err != nil {
				t.Errorf("Unexpected error: %v", err)
				return
			}
			if result != tt.expected {
				t.Errorf("Expected %s, got %s", tt.expected, result)
			}
		})
	}
}

func TestValidatePath(t *testing.T) {
	mapper, err := NewDockerPathMapper("D:/My Projects/Projects 1C", "/projects")
	if err != nil {
		t.Fatalf("Failed to create mapper: %v", err)
	}

	tests := []struct {
		name        string
		path        string
		expectError bool
	}{
		{
			name:        "valid path",
			path:        "D:/My Projects/Projects 1C/temp",
			expectError: false,
		},
		{
			name:        "root path",
			path:        "D:/My Projects/Projects 1C",
			expectError: false,
		},
		{
			name:        "path outside root",
			path:        "D:/Other/Projects",
			expectError: true,
		},
		{
			name:        "relative path",
			path:        "temp",
			expectError: false, // Will be converted to absolute
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := mapper.ValidatePath(tt.path)
			if tt.expectError {
				if err == nil {
					t.Errorf("Expected error but got none")
				}
			} else {
				if err != nil {
					t.Errorf("Unexpected error: %v", err)
				}
			}
		})
	}
}

func TestDisabledMapper(t *testing.T) {
	// Create a disabled mapper
	mapper := &DockerPathMapper{
		hostRoot:      "",
		containerRoot: "/projects",
		enabled:       false,
	}

	// Test that disabled mapper returns paths as-is
	hostPath := "D:/My Projects/Projects 1C/temp"
	result, err := mapper.HostToContainer(hostPath)
	if err != nil {
		t.Errorf("Unexpected error: %v", err)
	}
	if result != hostPath {
		t.Errorf("Expected %s, got %s", hostPath, result)
	}

	// Test URI normalization
	uri := "file:///D:/My Projects/Projects 1C/temp"
	result, err = mapper.NormalizeURI(uri)
	if err != nil {
		t.Errorf("Unexpected error: %v", err)
	}
	if result != uri {
		t.Errorf("Expected %s, got %s", uri, result)
	}

	// Test validation
	err = mapper.ValidatePath(hostPath)
	if err != nil {
		t.Errorf("Unexpected error: %v", err)
	}
}

func TestPathWithCyrillic(t *testing.T) {
	// Test with paths containing Cyrillic characters
	mapper, err := NewDockerPathMapper("D:/Мои Проекты/Проекты 1C", "/projects")
	if err != nil {
		t.Fatalf("Failed to create mapper: %v", err)
	}

	hostPath := "D:/Мои Проекты/Проекты 1C/тест"
	expected := "/projects/тест"

	result, err := mapper.HostToContainer(hostPath)
	if err != nil {
		t.Errorf("Unexpected error: %v", err)
	}
	if result != expected {
		t.Errorf("Expected %s, got %s", expected, result)
	}
}

func TestCrossPlatformPaths(t *testing.T) {
	// Test with different path separators
	mapper, err := NewDockerPathMapper("D:/My Projects/Projects 1C", "/projects")
	if err != nil {
		t.Fatalf("Failed to create mapper: %v", err)
	}

	tests := []struct {
		name     string
		hostPath string
		expected string
	}{
		{
			name:     "forward slashes",
			hostPath: "D:/My Projects/Projects 1C/temp",
			expected: "/projects/temp",
		},
		{
			name:     "backslashes",
			hostPath: "D:\\My Projects\\Projects 1C\\temp",
			expected: "/projects/temp",
		},
		{
			name:     "mixed separators",
			hostPath: "D:/My Projects\\Projects 1C/temp",
			expected: "/projects/temp",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := mapper.HostToContainer(tt.hostPath)
			if err != nil {
				t.Errorf("Unexpected error: %v", err)
				return
			}
			if result != tt.expected {
				t.Errorf("Expected %s, got %s", tt.expected, result)
			}
		})
	}
}

// TestIsWindowsAbsPath tests the IsWindowsAbsPath helper function
func TestIsWindowsAbsPath(t *testing.T) {
	tests := []struct {
		path     string
		expected bool
	}{
		{"D:/path", true},
		{"D:\\path", true},
		{"D:", true},
		{"d:/path", true},
		{"d:\\path", true},
		{"C:/", true},
		{"/path", false},
		{"path", false},
		{"./path", false},
		{"", false},
		{"1:/path", false}, // number is not a valid drive letter
	}

	for _, tt := range tests {
		t.Run(tt.path, func(t *testing.T) {
			result := IsWindowsAbsPath(tt.path)
			if result != tt.expected {
				t.Errorf("IsWindowsAbsPath(%q) = %v, expected %v", tt.path, result, tt.expected)
			}
		})
	}
}

// TestCaseInsensitiveMatching tests that path matching is case-insensitive for Windows paths
func TestCaseInsensitiveMatching(t *testing.T) {
	// Create mapper with mixed case host root
	mapper, err := NewDockerPathMapper("D:/My Projects/Projects 1C", "/projects")
	if err != nil {
		t.Fatalf("Failed to create mapper: %v", err)
	}

	tests := []struct {
		name        string
		hostPath    string
		expected    string
		expectError bool
	}{
		{
			name:        "exact case match",
			hostPath:    "D:/My Projects/Projects 1C/test",
			expected:    "/projects/test",
			expectError: false,
		},
		{
			name:        "lowercase drive letter",
			hostPath:    "d:/My Projects/Projects 1C/test",
			expected:    "/projects/test",
			expectError: false,
		},
		{
			name:        "uppercase path",
			hostPath:    "D:/MY PROJECTS/PROJECTS 1C/test",
			expected:    "/projects/test",
			expectError: false,
		},
		{
			name:        "mixed case",
			hostPath:    "d:/my projects/projects 1c/Test",
			expected:    "/projects/Test",
			expectError: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := mapper.HostToContainer(tt.hostPath)
			if tt.expectError {
				if err == nil {
					t.Errorf("Expected error but got none")
				}
				return
			}
			if err != nil {
				t.Errorf("Unexpected error: %v", err)
				return
			}
			if result != tt.expected {
				t.Errorf("Expected %s, got %s", tt.expected, result)
			}
		})
	}
}

// TestWindowsBackslashHostRoot tests that mapper works when host root has backslashes
func TestWindowsBackslashHostRoot(t *testing.T) {
	// Create mapper with Windows-style backslash host root
	mapper, err := NewDockerPathMapper("D:\\My Projects\\Projects 1C", "/projects")
	if err != nil {
		t.Fatalf("Failed to create mapper: %v", err)
	}

	tests := []struct {
		name        string
		hostPath    string
		expected    string
		expectError bool
	}{
		{
			name:        "backslash input",
			hostPath:    "D:\\My Projects\\Projects 1C\\test",
			expected:    "/projects/test",
			expectError: false,
		},
		{
			name:        "forward slash input",
			hostPath:    "D:/My Projects/Projects 1C/test",
			expected:    "/projects/test",
			expectError: false,
		},
		{
			name:        "file URI input",
			hostPath:    "file:///D:/My Projects/Projects 1C/test.bsl",
			expected:    "file:///projects/test.bsl",
			expectError: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := mapper.HostToContainer(tt.hostPath)
			if tt.expectError {
				if err == nil {
					t.Errorf("Expected error but got none")
				}
				return
			}
			if err != nil {
				t.Errorf("Unexpected error: %v", err)
				return
			}
			if result != tt.expected {
				t.Errorf("Expected %s, got %s", tt.expected, result)
			}
		})
	}
}

// TestRealWorldPaths tests paths that would actually be used by Cursor/Claude
func TestRealWorldPaths(t *testing.T) {
	// This is the exact scenario from the user's setup
	mapper, err := NewDockerPathMapper("D:\\My Projects\\FrameWork 1C\\mcp-lsp-bridge", "/projects")
	if err != nil {
		t.Fatalf("Failed to create mapper: %v", err)
	}

	tests := []struct {
		name        string
		hostPath    string
		expected    string
		expectError bool
	}{
		{
			name:        "test-workspace with backslashes",
			hostPath:    "D:\\My Projects\\FrameWork 1C\\mcp-lsp-bridge\\test-workspace",
			expected:    "/projects/test-workspace",
			expectError: false,
		},
		{
			name:        "test-workspace with forward slashes",
			hostPath:    "D:/My Projects/FrameWork 1C/mcp-lsp-bridge/test-workspace",
			expected:    "/projects/test-workspace",
			expectError: false,
		},
		{
			name:        "BSL file with backslashes",
			hostPath:    "D:\\My Projects\\FrameWork 1C\\mcp-lsp-bridge\\test-workspace\\Catalogs\\Test\\Module.bsl",
			expected:    "/projects/test-workspace/Catalogs/Test/Module.bsl",
			expectError: false,
		},
		{
			name:        "file URI format",
			hostPath:    "file:///D:/My Projects/FrameWork 1C/mcp-lsp-bridge/test-workspace/test.bsl",
			expected:    "file:///projects/test-workspace/test.bsl",
			expectError: false,
		},
		{
			name:        "lowercase drive in file URI",
			hostPath:    "file:///d:/My Projects/FrameWork 1C/mcp-lsp-bridge/test.bsl",
			expected:    "file:///projects/test.bsl",
			expectError: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := mapper.HostToContainer(tt.hostPath)
			if tt.expectError {
				if err == nil {
					t.Errorf("Expected error but got none")
				}
				return
			}
			if err != nil {
				t.Errorf("Unexpected error: %v", err)
				return
			}
			if result != tt.expected {
				t.Errorf("Expected %s, got %s", tt.expected, result)
			}
		})
	}
}
