// Package directories provides cross-platform directory resolution
// for applications based on user context and system conventions.
package directories

import (
	"fmt"
	"os"
	"os/user"
	"path/filepath"
	"runtime"
)

// EnvProvider provides access to environment variables.
type EnvProvider interface {
	Getenv(key string) string
}

// DefaultEnvProvider is a concrete implementation of EnvProvider using os.Getenv.
type DefaultEnvProvider struct{}

func NewDefaultEnvProvider() DefaultEnvProvider {
	return DefaultEnvProvider{}
}

func (d DefaultEnvProvider) Getenv(key string) string {
	return os.Getenv(key)
}

// UserProvider provides access to the current user's information.
type UserProvider interface {
	Current() (*user.User, error)
}

// DefaultUserProvider is a concrete implementation of UserProvider using user.Current().
type DefaultUserProvider struct{}

func (d DefaultUserProvider) Current() (*user.User, error) {
	return user.Current()
}

// DirectoryResolver handles directory resolution logic for applications
type DirectoryResolver struct {
	appName         string
	userProvider    UserProvider
	envProvider     EnvProvider
	shouldEnsureDir bool
}

// NewDirectoryResolver creates a new directory resolver with specified providers.
// Use NewDefaultDirectoryResolver for typical application usage.
func NewDirectoryResolver(appName string, userProvider UserProvider, envProvider EnvProvider, shouldEnsureDir bool) *DirectoryResolver {
	return &DirectoryResolver{
		appName:         appName,
		userProvider:    userProvider,
		envProvider:     envProvider,
		shouldEnsureDir: shouldEnsureDir,
	}
}

// isRoot checks if the current user is root (UID 0 on Unix systems)
func (dr *DirectoryResolver) isRoot() (bool, error) {
	u, err := dr.userProvider.Current()
	if err != nil {
		return false, fmt.Errorf("failed to get current user: %w", err)
	}

	return u.Uid == "0", nil
}

// maybeEnsureDir creates the directory if it doesn't exist and returns the path
func (dr *DirectoryResolver) maybeEnsureDir(dir string) (string, error) {
	if !dr.shouldEnsureDir {
		return dir, nil
	}

	if err := os.MkdirAll(dir, 0755); err != nil {
		return "", fmt.Errorf("failed to create directory %s: %w", dir, err)
	}

	return dir, nil
}

// GetLogDirectory returns the appropriate log directory based on user context
// For root: /var/log/{appName}
// For regular users: ~/.local/share/{appName} (Unix) or %LOCALAPPDATA%\{appName}\logs (Windows)
func (dr *DirectoryResolver) GetLogDirectory() (string, error) {
	isR, err := dr.isRoot()
	if err != nil {
		return "", fmt.Errorf("failed to check if user is root: %w", err)
	}

	if isR {
		return dr.maybeEnsureDir(filepath.Join("/", "var", "log", dr.appName))
	}

	return dr.getUserLogDirectory()
}

// getUserLogDirectory gets the user-specific log directory following platform conventions
func (dr *DirectoryResolver) getUserLogDirectory() (string, error) {
	u, err := dr.userProvider.Current()
	if err != nil {
		return "", fmt.Errorf("failed to get current user: %w", err)
	}

	if runtime.GOOS == "windows" {
		// Windows: use %LOCALAPPDATA%
		baseDir := dr.envProvider.Getenv("LOCALAPPDATA")
		if baseDir == "" {
			baseDir = filepath.Join(u.HomeDir, "AppData", "Local")
		}

		return dr.maybeEnsureDir(filepath.Join(baseDir, dr.appName, "logs"))
	}

	// Unix-like systems: follow XDG Base Directory Specification
	xdgDataHome := dr.envProvider.Getenv("XDG_DATA_HOME")
	if xdgDataHome == "" {
		xdgDataHome = filepath.Join(u.HomeDir, ".local", "share")
	}

	return dr.maybeEnsureDir(filepath.Join(xdgDataHome, dr.appName, "logs"))
}

// GetDataDirectory returns appropriate data directory for the user
// For root: /var/lib/{appName}
// For regular users: ~/.local/share/{appName} (Unix) or %LOCALAPPDATA%\{appName} (Windows)
func (dr *DirectoryResolver) GetDataDirectory() (string, error) {
	isR, err := dr.isRoot()
	if err != nil {
		return "", fmt.Errorf("failed to check if user is root: %w", err)
	}

	if isR {
		return dr.maybeEnsureDir(filepath.Join("/", "var", "lib", dr.appName))
	}

	u, err := dr.userProvider.Current()
	if err != nil {
		return "", fmt.Errorf("failed to get current user: %w", err)
	}

	if runtime.GOOS == "windows" {
		baseDir := dr.envProvider.Getenv("LOCALAPPDATA")
		if baseDir == "" {
			baseDir = filepath.Join(u.HomeDir, "AppData", "Local")
		}

		return dr.maybeEnsureDir(filepath.Join(baseDir, dr.appName))
	}

	xdgDataHome := dr.envProvider.Getenv("XDG_DATA_HOME")
	if xdgDataHome == "" {
		xdgDataHome = filepath.Join(u.HomeDir, ".local", "share")
	}

	return dr.maybeEnsureDir(filepath.Join(xdgDataHome, dr.appName))
}

// GetCacheDirectory returns appropriate cache directory for the user
// For root: /var/cache/{appName}
// For regular users: ~/.cache/{appName} (Unix) or %TEMP%\{appName} (Windows)
func (dr *DirectoryResolver) GetCacheDirectory() (string, error) {
	isR, err := dr.isRoot()
	if err != nil {
		return "", fmt.Errorf("failed to check if user is root: %w", err)
	}

	if isR {
		return dr.maybeEnsureDir(filepath.Join("/", "var", "cache", dr.appName))
	}

	u, err := dr.userProvider.Current()
	if err != nil {
		return "", fmt.Errorf("failed to get current user: %w", err)
	}

	if runtime.GOOS == "windows" {
		baseDir := dr.envProvider.Getenv("TEMP")
		if baseDir == "" {
			baseDir = filepath.Join(u.HomeDir, "AppData", "Local", "Temp")
		}

		return dr.maybeEnsureDir(filepath.Join(baseDir, dr.appName))
	}

	xdgCacheHome := dr.envProvider.Getenv("XDG_CACHE_HOME")
	if xdgCacheHome == "" {
		xdgCacheHome = filepath.Join(u.HomeDir, ".cache")
	}

	return dr.maybeEnsureDir(filepath.Join(xdgCacheHome, dr.appName))
}

// GetConfigDirectory returns appropriate configuration directory for the user
// For root: /etc/{appName}
// For regular users: ~/.config/{appName} (Unix) or %APPDATA%\{appName} (Windows)
func (dr *DirectoryResolver) GetConfigDirectory() (string, error) {
	isR, err := dr.isRoot()
	if err != nil {
		return "", fmt.Errorf("failed to check if user is root: %w", err)
	}

	if isR {
		return dr.maybeEnsureDir(filepath.Join("/", "etc", dr.appName))
	}

	u, err := dr.userProvider.Current()
	if err != nil {
		return "", fmt.Errorf("failed to get current user: %w", err)
	}

	if runtime.GOOS == "windows" {
		configDir := dr.envProvider.Getenv("APPDATA")
		if configDir == "" {
			configDir = filepath.Join(u.HomeDir, "AppData", "Roaming")
		}

		return dr.maybeEnsureDir(filepath.Join(configDir, dr.appName))
	}

	// Unix-like systems
	xdgConfigHome := dr.envProvider.Getenv("XDG_CONFIG_HOME")
	if xdgConfigHome == "" {
		xdgConfigHome = filepath.Join(u.HomeDir, ".config")
	}

	return dr.maybeEnsureDir(filepath.Join(xdgConfigHome, dr.appName))
}
