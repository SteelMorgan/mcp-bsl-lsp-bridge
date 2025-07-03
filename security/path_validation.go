package security

import (
	"errors"
	"fmt"
	"path/filepath"
	"strings"
)

// getCleanAbsPath validates and returns a clean absolute path
func getCleanAbsPath(path string) (string, error) {
	if path == "" || path == "." {
		return "", errors.New("path cannot be empty or current directory")
	}

	cleanPath := filepath.Clean(path)
	absPath, err := filepath.Abs(cleanPath)
	if err != nil {
		return "", fmt.Errorf("invalid file path: %w", err)
	}
	return absPath, nil
}

func GetCleanAbsPath(path string) (string, error) {
	return getCleanAbsPath(path)
}

// IsWithinAllowedDirectory checks if a path is within an allowed base directory
func IsWithinAllowedDirectory(path, baseDir string) bool {
	// Convert paths to absolute and clean them first
	absBase, _ := filepath.Abs(baseDir)
	absPath, _ := filepath.Abs(path)

	// Normalize paths by cleaning them
	cleanBase := filepath.Clean(absBase)
	cleanPath := filepath.Clean(absPath)

	// Exact match
	if cleanPath == cleanBase {
		return true
	}

	// Check if path starts with base directory (normal case - path is within base)
	if strings.HasPrefix(cleanPath, cleanBase+string(filepath.Separator)) {
		return true
	}

	// Note: We explicitly do NOT allow parent directories to be considered "within" child directories
	// as this would be a security vulnerability

	return false
}

// isWithinAllowedDirectory is an internal helper function
func isWithinAllowedDirectory(path, baseDir string) bool {
	return IsWithinAllowedDirectory(path, baseDir)
}

// ValidateConfigPath validates a configuration file path against allowed directories
func ValidateConfigPath(path string, allowedDirectories []string) (string, error) {
	// Clean and validate the path
	cleanPath, err := getCleanAbsPath(path)
	if err != nil {
		return "", fmt.Errorf("invalid config path: %w", err)
	}

	// Add current directory to allowed directories if not present
	if !contains(allowedDirectories, ".") {
		allowedDirectories = append(allowedDirectories, ".")
	}

	// Check against allowed directories
	for _, allowedDir := range allowedDirectories {
		if isWithinAllowedDirectory(cleanPath, allowedDir) {
			return cleanPath, nil
		}
	}

	return "", fmt.Errorf("file path is not allowed: %s", cleanPath)
}

// contains checks if a slice contains a specific string
func contains(slice []string, item string) bool {
	for _, s := range slice {
		if s == item {
			return true
		}
	}
	return false
}

// GetConfigAllowedDirectories returns the list of directories where config files are allowed
func GetConfigAllowedDirectories(configDir, workingDir string) []string {
	allowedDirs := []string{}

	// Add config directory if provided
	if configDir != "" {
		allowedDirs = append(allowedDirs, configDir)
	}

	// Add current working directory
	if workingDir != "" {
		allowedDirs = append(allowedDirs, workingDir)
	}

	// Add common config locations
	allowedDirs = append(allowedDirs, ".")

	return allowedDirs
}