package utils

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// DockerPathMapper handles path conversion between host system and Docker container
type DockerPathMapper struct {
	hostRoot      string // D:\My Projects\Projects 1C
	containerRoot string // /projects
	enabled       bool   // true if working in Docker mode
}

// NewDockerPathMapper creates a new DockerPathMapper instance
func NewDockerPathMapper(hostRoot, containerRoot string) (*DockerPathMapper, error) {
	if hostRoot == "" {
		return nil, errors.New("host root path cannot be empty")
	}
	if containerRoot == "" {
		return nil, errors.New("container root path cannot be empty")
	}

	// Clean and normalize paths
	// For Docker mode, don't use filepath.Abs as it may be cross-platform (Windows path on Linux)
	cleanHostRoot := filepath.Clean(hostRoot)
	// Convert to forward slashes for consistency
	cleanHostRoot = filepath.ToSlash(cleanHostRoot)

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
		filePath = strings.TrimPrefix(hostPath, "file://")
		// Handle Windows file:// URI format (file:///C:/path)
		if strings.HasPrefix(filePath, "/") && len(filePath) > 3 && filePath[2] == ':' {
			filePath = filePath[1:] // Remove leading slash for Windows paths
		}
	} else {
		filePath = hostPath
	}

	// Clean and normalize the input path
	// Don't use filepath.Abs for cross-platform paths (Windows path on Linux)
	// First replace backslashes with forward slashes for cross-platform compatibility
	cleanPath := strings.ReplaceAll(filePath, "\\", "/")
	cleanPath = filepath.Clean(cleanPath)
	
	// Normalize host root path separators
	normalizedHostRoot := strings.ReplaceAll(dpm.hostRoot, "\\", "/")

	// Check if path is within the host root directory
	if !strings.HasPrefix(cleanPath, normalizedHostRoot) {
		return "", fmt.Errorf("path %s is outside mounted directory %s", cleanPath, normalizedHostRoot)
	}

	// Replace host root with container root
	relativePath := strings.TrimPrefix(cleanPath, normalizedHostRoot)
	relativePath = strings.TrimPrefix(relativePath, "/")

	// Build container path
	var containerPath string
	if relativePath == "" {
		containerPath = dpm.containerRoot
	} else {
		containerPath = filepath.Join(dpm.containerRoot, relativePath)
	}

	// Normalize the final path
	containerPath = filepath.Clean(containerPath)

	// Return as URI if input was URI
	if isURI {
		return "file://" + strings.ReplaceAll(containerPath, "\\", "/"), nil
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

	// Clean and normalize the input path
	cleanPath := filepath.Clean(containerPath)

	// Check if path is within the container root directory
	if !strings.HasPrefix(cleanPath, dpm.containerRoot) {
		return "", fmt.Errorf("path %s is outside container root %s", cleanPath, dpm.containerRoot)
	}

	// Replace container root with host root
	relativePath := strings.TrimPrefix(cleanPath, dpm.containerRoot)
	relativePath = strings.TrimPrefix(relativePath, "/")

	// Build host path
	var hostPath string
	if relativePath == "" {
		hostPath = dpm.hostRoot
	} else {
		hostPath = filepath.Join(dpm.hostRoot, relativePath)
	}

	// Normalize the final path
	hostPath = filepath.Clean(hostPath)

	return hostPath, nil
}

// ValidatePath checks if a host path is within the allowed directory
func (dpm *DockerPathMapper) ValidatePath(hostPath string) error {
	if !dpm.enabled {
		return nil // No validation if disabled
	}

	cleanPath, err := filepath.Abs(hostPath)
	if err != nil {
		return fmt.Errorf("invalid path: %w", err)
	}

	// Check if path is within host root
	cleanPath = filepath.ToSlash(cleanPath)
	normalizedHostRoot := filepath.ToSlash(dpm.hostRoot)

	if !strings.HasPrefix(cleanPath, normalizedHostRoot) {
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
		filePath = strings.TrimPrefix(uri, "file://")
		// Handle Windows file:// URI format (file:///C:/path)
		if strings.HasPrefix(filePath, "/") && len(filePath) > 3 && filePath[2] == ':' {
			filePath = filePath[1:] // Remove leading slash for Windows paths
		}
	} else {
		filePath = uri
	}

	// Convert host path to container path
	containerPath, err := dpm.HostToContainer(filePath)
	if err != nil {
		return "", err
	}

	// Return as file:// URI with proper Unix path separators
	return "file://" + strings.ReplaceAll(containerPath, "\\", "/"), nil
}

// ConvertURI converts a file:// URI from host to container format
func (dpm *DockerPathMapper) ConvertURI(uri string) (string, error) {
	return dpm.NormalizeURI(uri)
}
