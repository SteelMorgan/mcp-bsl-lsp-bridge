package lsp

import (
	"encoding/json"
	"fmt"
	"os"
)

// LoadLSPConfig loads the LSP configuration from a JSON file
func LoadLSPConfig(path string) (*LSPServerConfig, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("failed to open config file: %w", err)
	}
	defer file.Close()

	var config LSPServerConfig
	decoder := json.NewDecoder(file)
	if err := decoder.Decode(&config); err != nil {
		return nil, fmt.Errorf("failed to parse config file: %w", err)
	}

	// Compute extension to language mapping if not provided
	if config.ExtensionLanguageMap == nil {
		config.ExtensionLanguageMap = make(map[string]string)
		for language, serverConfig := range config.LanguageServers {
			for _, ext := range serverConfig.Filetypes {
				config.ExtensionLanguageMap[ext] = language
			}
		}
	}

	// Compute language to extensions mapping if not provided
	if config.LanguageExtensionMap == nil {
		config.LanguageExtensionMap = make(map[string][]string)
		for language, serverConfig := range config.LanguageServers {
			config.LanguageExtensionMap[language] = serverConfig.Filetypes
		}
	}

	return &config, nil
}

func (c LSPServerConfig) FindServerConfig(language string) (*LanguageServerConfig, error) {
	for lang, serverConfig := range c.LanguageServers {
		if lang == language {
			return &serverConfig, nil
		}
	}

	return nil, fmt.Errorf("failed to find langauge config for '%s'", language)
}