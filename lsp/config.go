package lsp

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
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

	// Compute extension to language mapping if not provided
	if config.ExtensionLanguageMap == nil {
		config.ExtensionLanguageMap = make(map[string]types.Language)
		for language, serverConfig := range config.LanguageServers {
			for _, ext := range serverConfig.Filetypes {
				config.ExtensionLanguageMap[ext] = language
			}
		}
	}

	// Compute language to extensions mapping if not provided
	if config.LanguageExtensionMap == nil {
		config.LanguageExtensionMap = make(map[types.Language][]string)
		for language, serverConfig := range config.LanguageServers {
			config.LanguageExtensionMap[language] = serverConfig.Filetypes
		}
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
	for lang, serverConfig := range c.LanguageServers {
		if lang == types.Language(language) {
			return &serverConfig, nil
		}
	}

	return nil, fmt.Errorf("failed to find langauge config for '%s'", language)
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

func (c LSPServerConfig) GetLanguageServers() map[types.Language]types.LanguageServerConfigProvider {
	result := make(map[types.Language]types.LanguageServerConfigProvider)
	for lang, config := range c.LanguageServers {
		result[types.Language(lang)] = &config
	}
	return result
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
