package utils

import (
	"errors"
	"fmt"
	"os"
	"path"
	"strings"
)

// DockerPathMapper handles path conversion between host system and Docker container
type DockerPathMapper struct {
	hostRoot      string // D:/My Projects/Projects 1C (normalized with forward slashes)
	containerRoot string // /projects
	enabled       bool   // true if working in Docker mode
}

// IsWindowsAbsPath checks if a path is a Windows absolute path (e.g., C:\... or C:/...)
// This works correctly regardless of the runtime OS (Linux or Windows)
func IsWindowsAbsPath(p string) bool {
	if len(p) < 2 {
		return false
	}
	// Check for drive letter pattern: X: or X:/ or X:\
	letter := p[0]
	if (letter >= 'A' && letter <= 'Z') || (letter >= 'a' && letter <= 'z') {
		if p[1] == ':' {
			return true
		}
	}
	return false
}

// normalizePathSeparators converts all backslashes to forward slashes
// and cleans the path (removes redundant slashes, resolves . and ..)
func normalizePathSeparators(p string) string {
	// Convert all backslashes to forward slashes
	normalized := strings.ReplaceAll(p, "\\", "/")
	// Use path.Clean which always uses forward slashes
	return path.Clean(normalized)
}

// pathsEqualFold compares two paths case-insensitively (for Windows path compatibility)
// Both paths should already be normalized with forward slashes
func pathsEqualFold(a, b string) bool {
	return strings.EqualFold(a, b)
}

// hasPrefixFold checks if path has prefix using case-insensitive comparison
// Both paths should already be normalized with forward slashes
func hasPrefixFold(p, prefix string) bool {
	if len(p) < len(prefix) {
		return false
	}
	return strings.EqualFold(p[:len(prefix)], prefix)
}

// NewDockerPathMapper creates a new DockerPathMapper instance
func NewDockerPathMapper(hostRoot, containerRoot string) (*DockerPathMapper, error) {
	if hostRoot == "" {
		return nil, errors.New("host root path cannot be empty")
	}
	if containerRoot == "" {
		return nil, errors.New("container root path cannot be empty")
	}

	// Normalize host root path - convert backslashes to forward slashes
	// This works correctly on both Linux and Windows
	cleanHostRoot := normalizePathSeparators(hostRoot)

	// For container paths, use simple string cleaning to avoid Windows path issues
	cleanContainerRoot := strings.TrimSuffix(containerRoot, "/")
	if !strings.HasPrefix(cleanContainerRoot, "/") {
		return nil, errors.New("container root must be an absolute path starting with /")
	}

	return &DockerPathMapper{
		hostRoot:      cleanHostRoot,
		containerRoot: cleanContainerRoot,
		enabled:       true,
	}, nil
}

// NewDockerPathMapperFromEnv creates a DockerPathMapper from environment variables
func NewDockerPathMapperFromEnv() (*DockerPathMapper, error) {
	// Try different environment variable names for host root
	hostRoot := os.Getenv("HOST_PROJECTS_ROOT")
	if hostRoot == "" {
		hostRoot = os.Getenv("PROJECTS_HOST_ROOT")
	}

	containerRoot := os.Getenv("PROJECTS_ROOT")
	if containerRoot == "" {
		containerRoot = "/projects" // Default container root
	}

	// If no host root is specified, return disabled mapper
	if hostRoot == "" {
		return &DockerPathMapper{
			hostRoot:      "",
			containerRoot: containerRoot,
			enabled:       false,
		}, nil
	}

	return NewDockerPathMapper(hostRoot, containerRoot)
}

// IsEnabled returns true if the path mapper is enabled (Docker mode)
func (dpm *DockerPathMapper) IsEnabled() bool {
	return dpm.enabled
}

// HostRoot returns the host root path
func (dpm *DockerPathMapper) HostRoot() string {
	return dpm.hostRoot
}

// ContainerRoot returns the container root path
func (dpm *DockerPathMapper) ContainerRoot() string {
	return dpm.containerRoot
}

// HostToContainer converts a host path to container path
// Handles Windows paths (D:\..., D:/...) correctly even when running on Linux
func (dpm *DockerPathMapper) HostToContainer(hostPath string) (string, error) {
	if !dpm.enabled {
		return hostPath, nil // Return as-is if disabled
	}

	if hostPath == "" {
		return "", errors.New("host path cannot be empty")
	}

	// Handle file:// URI
	isURI := strings.HasPrefix(hostPath, "file://")
	var filePath string
	if isURI {
		p, err := FileURIToPath(hostPath)
		if err != nil {
			return "", err
		}
		filePath = p
	} else {
		filePath = hostPath
	}

	// Normalize the input path - convert backslashes to forward slashes
	cleanPath := normalizePathSeparators(filePath)

	// hostRoot is already normalized in NewDockerPathMapper
	normalizedHostRoot := dpm.hostRoot

	// Check if path is within the host root directory (case-insensitive for Windows paths)
	if !hasPrefixFold(cleanPath, normalizedHostRoot) {
		return "", fmt.Errorf("path %s is outside mounted directory %s", cleanPath, normalizedHostRoot)
	}

	// Extract relative path (preserve original case for the relative portion)
	relativePath := cleanPath[len(normalizedHostRoot):]
	relativePath = strings.TrimPrefix(relativePath, "/")

	// Build container path
	var containerPath string
	if relativePath == "" {
		containerPath = dpm.containerRoot
	} else {
		containerPath = path.Join(dpm.containerRoot, relativePath)
	}

	// Normalize the final path
	containerPath = path.Clean(containerPath)

	// Return as URI if input was URI
	if isURI {
		return "file://" + containerPath, nil
	}
	return containerPath, nil
}

// ContainerToHost converts a container path to host path
func (dpm *DockerPathMapper) ContainerToHost(containerPath string) (string, error) {
	if !dpm.enabled {
		return containerPath, nil // Return as-is if disabled
	}

	if containerPath == "" {
		return "", errors.New("container path cannot be empty")
	}

	// Clean and normalize the input path (container is always slash-based)
	cleanPath := normalizePathSeparators(containerPath)

	// Check if path is within the container root directory
	if !strings.HasPrefix(cleanPath, dpm.containerRoot) {
		return "", fmt.Errorf("path %s is outside container root %s", cleanPath, dpm.containerRoot)
	}

	// Replace container root with host root
	relativePath := strings.TrimPrefix(cleanPath, dpm.containerRoot)
	relativePath = strings.TrimPrefix(relativePath, "/")

	// Build host path (keep forward slashes - the caller can convert if needed)
	var hostPath string
	if relativePath == "" {
		hostPath = dpm.hostRoot
	} else {
		hostPath = path.Join(dpm.hostRoot, relativePath)
	}

	// Normalize the final path
	hostPath = path.Clean(hostPath)

	return hostPath, nil
}

// ValidatePath checks if a host path is within the allowed directory
func (dpm *DockerPathMapper) ValidatePath(hostPath string) error {
	if !dpm.enabled {
		return nil // No validation if disabled
	}

	// Normalize the path first
	cleanPath := normalizePathSeparators(hostPath)

	// Check if path is absolute (works for both Windows and Unix paths)
	isAbsolute := strings.HasPrefix(cleanPath, "/") || IsWindowsAbsPath(cleanPath)

	// In Docker mode, treat relative paths as relative to hostRoot
	if !isAbsolute {
		cleanPath = path.Join(dpm.hostRoot, cleanPath)
		cleanPath = path.Clean(cleanPath)
	}

	// Check if path is within host root (case-insensitive for Windows paths)
	if !hasPrefixFold(cleanPath, dpm.hostRoot) {
		return fmt.Errorf("path is outside mounted directory: %s", hostPath)
	}

	return nil
}

// NormalizeURI normalizes a file:// URI for container usage
func (dpm *DockerPathMapper) NormalizeURI(uri string) (string, error) {
	if !dpm.enabled {
		return uri, nil // Return as-is if disabled
	}

	// Extract path from file:// URI
	var filePath string
	if strings.HasPrefix(uri, "file://") {
		p, err := FileURIToPath(uri)
		if err != nil {
			return "", err
		}
		filePath = p
	} else {
		filePath = uri
	}

	// Convert host path to container path
	containerPath, err := dpm.HostToContainer(filePath)
	if err != nil {
		return "", err
	}

	// If HostToContainer already returned a URI, return it
	if strings.HasPrefix(containerPath, "file://") {
		return containerPath, nil
	}

	// Return as file:// URI with proper Unix path separators
	return "file://" + containerPath, nil
}

// ConvertURI converts a file:// URI from host to container format
func (dpm *DockerPathMapper) ConvertURI(uri string) (string, error) {
	return dpm.NormalizeURI(uri)
}
