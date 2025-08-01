package lsp

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"

	"rockerboo/mcp-lsp-bridge/logger"
	"rockerboo/mcp-lsp-bridge/security"
	"rockerboo/mcp-lsp-bridge/types"
)

// LoadLSPConfig loads the LSP configuration from a JSON file with security validation
func LoadLSPConfig(path string, allowedDirectories []string) (config *LSPServerConfig, err error) {
	// Validate path using secure path validation
	cleanPath, err := security.ValidateConfigPath(path, allowedDirectories)
	if err != nil {
		return nil, fmt.Errorf("config path validation failed: %w", err)
	}

	file, err := os.Open(cleanPath) // #nosec G304 - Path validated by security.ValidateConfigPath
	if err != nil {
		return nil, fmt.Errorf("failed to open config file: %w", err)
	}

	defer func() {
		if closeErr := file.Close(); closeErr != nil && err == nil {
			err = closeErr
		}
	}()

	decoder := json.NewDecoder(file)
	if err := decoder.Decode(&config); err != nil {
		return nil, fmt.Errorf("failed to parse config file: %w", err)
	}

	// Validate that we have the required mappings
	if config.ExtensionLanguageMap == nil {
		return nil, errors.New("extension_language_map is required in configuration")
	}

	if config.LanguageServerMap == nil {
		return nil, errors.New("language_server_map is required in configuration")
	}

	return config, nil
}

func (c *LSPServerConfig) GlobalConfig() GlobalConfig {
	return c.Global
}

func (c *LSPServerConfig) FindExtLanguage(ext string) (*types.Language, error) {
	language, exists := c.ExtensionLanguageMap[ext]

	if !exists {
		return nil, fmt.Errorf("no language found for %s", ext)
	}

	typesLanguage := types.Language(language)
	return &typesLanguage, nil
}

func (c LSPServerConfig) FindServerConfig(language string) (types.LanguageServerConfigProvider, error) {
	// Find which server handles this language
	for serverName, languages := range c.LanguageServerMap {
		for _, lang := range languages {
			if string(lang) == language {
				// Found the server, now get its config
				if serverConfig, exists := c.LanguageServers[serverName]; exists {
					return &serverConfig, nil
				}
				return nil, fmt.Errorf("server config not found for server '%s'", string(serverName))
			}
		}
	}

	return nil, fmt.Errorf("no server found for language '%s'", language)
}

// FindAllServerConfigs returns all server configurations that support the given language
func (c LSPServerConfig) FindAllServerConfigs(language string) ([]types.LanguageServerConfigProvider, []types.LanguageServer, error) {
	var configs []types.LanguageServerConfigProvider
	var servers []types.LanguageServer

	// Find all servers that handle this language
	for serverName, languages := range c.LanguageServerMap {
		for _, lang := range languages {
			if string(lang) == language {
				// Found a server, get its config
				if serverConfig, exists := c.LanguageServers[serverName]; exists {
					configs = append(configs, &serverConfig)
					servers = append(servers, serverName)
				}
			}
		}
	}

	if len(configs) == 0 {
		return nil, nil, fmt.Errorf("no servers found for language '%s'", language)
	}

	return configs, servers, nil
}

// ProjectRootMarker represents a project root identifier
type ProjectRootMarker struct {
	Filename string
	Language string
}

// GetProjectRootMarkers returns a list of common project root markers
func GetProjectRootMarkers() []ProjectRootMarker {
	return []ProjectRootMarker{
		{"go.mod", "go"},
		{"go.sum", "go"},
		{"package.json", "typescript"},
		{"yarn.lock", "typescript"},
		{"package-lock.json", "typescript"},
		{"tsconfig.json", "typescript"},
		{"Cargo.toml", "rust"},
		{"Cargo.lock", "rust"},
		{"pyproject.toml", "python"},
		{"setup.py", "python"},
		{"requirements.txt", "python"},
		{"Pipfile", "python"},
		{"poetry.lock", "python"},
		{"pom.xml", "java"},
		{"build.gradle", "java"},
		{"Gemfile", "ruby"},
		{"composer.json", "php"},
		{"CMakeLists.txt", "cpp"},
		{"Makefile", "c"},
		{"Dockerfile", "dockerfile"},
		{".gitignore", ""},
		{"README.md", ""},
	}
}

// DetectProjectLanguages scans a directory for project root markers and file extensions
// to determine all languages used in the project, returning them in priority order
func (c LSPServerConfig) DetectProjectLanguages(projectPath string) ([]types.Language, error) {
	if projectPath == "" {
		return nil, errors.New("project path cannot be empty")
	}

	logger.Info(fmt.Sprintf("Detecting project languages in '%s'", projectPath))

	// Check if directory exists
	if _, err := os.Stat(projectPath); os.IsNotExist(err) {
		return nil, fmt.Errorf("project directory does not exist: %s", projectPath)
	}

	languageScores := make(map[string]int)
	rootMarkers := GetProjectRootMarkers()

	// Step 1: Check for project root markers (highest priority)
	for _, marker := range rootMarkers {
		markerPath := filepath.Join(projectPath, marker.Filename)
		if _, err := os.Stat(markerPath); err == nil && marker.Language != "" {
			languageScores[marker.Language] += 100 // High priority for root markers
		}
	}

	// Step 2: Scan files for language detection based on extensions
	err := filepath.Walk(projectPath, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		// Skip hidden directories and common ignore patterns
		if info.IsDir() {
			name := info.Name()
			if strings.HasPrefix(name, ".") && name != "." {
				return filepath.SkipDir
			}

			if name == "node_modules" || name == "target" || name == "build" || name == "dist" {
				return filepath.SkipDir
			}
		}

		if !info.IsDir() {
			ext := filepath.Ext(path)
			if language, found := c.ExtensionLanguageMap[ext]; found {
				languageScores[string(language)] += 1
			}
		}

		return nil
	})

	if err != nil {
		return nil, fmt.Errorf("error walking project directory: %w", err)
	}

	// Step 3: Sort languages by score (descending)
	type languageScore struct {
		language string
		score    int
	}

	var sortedLanguages []languageScore
	for lang, score := range languageScores {
		sortedLanguages = append(sortedLanguages, languageScore{lang, score})
	}

	// Simple sorting by score (descending)
	for i := range sortedLanguages {
		for j := i + 1; j < len(sortedLanguages); j++ {
			if sortedLanguages[j].score > sortedLanguages[i].score {
				sortedLanguages[i], sortedLanguages[j] = sortedLanguages[j], sortedLanguages[i]
			}
		}
	}

	// Extract just the language names
	var result []types.Language
	for _, ls := range sortedLanguages {
		result = append(result, types.Language(ls.language))
	}

	if len(result) == 0 {
		return nil, errors.New("no recognizable project languages found")
	}

	return result, nil
}

func (c LSPServerConfig) GetGlobalConfig() types.GlobalConfig {
	return types.GlobalConfig(c.Global)
}

func (c LSPServerConfig) GetLanguageServers() map[types.LanguageServer]types.LanguageServerConfigProvider {
	result := make(map[types.LanguageServer]types.LanguageServerConfigProvider)
	// Build a server -> server config mapping
	for serverName, serverConfig := range c.LanguageServers {
		result[serverName] = &serverConfig
	}
	return result
}

// GetServerNameFromLanguage returns the server name for a given language
func (c LSPServerConfig) GetServerNameFromLanguage(language types.Language) types.LanguageServer {
	for serverName, supportedLanguages := range c.LanguageServerMap {
		if slices.Contains(supportedLanguages, language) {
			return serverName
		}
	}
	return "" // Handle not found case
}

// DetectPrimaryProjectLanguage returns the most likely primary language for a project
func (c LSPServerConfig) DetectPrimaryProjectLanguage(projectPath string) (*types.Language, error) {
	languages, err := c.DetectProjectLanguages(projectPath)
	if err != nil {
		return nil, err
	}

	if len(languages) == 0 {
		return nil, errors.New("no project language detected")
	}

	language := types.Language(languages[0])

	return &language, nil
}
