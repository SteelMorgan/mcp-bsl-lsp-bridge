package mocks

import (
	"rockerboo/mcp-lsp-bridge/lsp"

	"github.com/stretchr/testify/mock"
)

type MockLSPServerConfig struct {
	mock.Mock
}

func (m *MockLSPServerConfig) DetectProjectLanguages(projectPath string) ([]lsp.Language, error) {
	args := m.Called(projectPath)
	return args.Get(0).([]lsp.Language), args.Error(1)
}

func (m *MockLSPServerConfig) DetectPrimaryProjectLanguage(projectPath string) (*lsp.Language, error) {
	args := m.Called(projectPath)
	return args.Get(0).(*lsp.Language), args.Error(1)
}

func (m *MockLSPServerConfig) FindServerConfig(language string) (*lsp.LanguageServerConfig, error) {
	args := m.Called(language)
	return args.Get(0).(*lsp.LanguageServerConfig), args.Error(1)
}


func (m *MockLSPServerConfig) GetGlobalConfig() lsp.GlobalConfig {
	args := m.Called()
	return args.Get(0).(lsp.GlobalConfig)
}

func (m *MockLSPServerConfig) GetLanguageServers() map[lsp.Language]lsp.LanguageServerConfig {
	args := m.Called()
	return args.Get(0).(map[lsp.Language]lsp.LanguageServerConfig)
}

func (m *MockLSPServerConfig) FindExtLanguage(ext string) (*lsp.Language, error) {
	args := m.Called(ext)
	return args.Get(0).(*lsp.Language), args.Error(1)
}
