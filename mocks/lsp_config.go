package mocks

import (
	"rockerboo/mcp-lsp-bridge/types"

	"github.com/stretchr/testify/mock"
)

type MockLSPServerConfig struct {
	mock.Mock
}

func (m *MockLSPServerConfig) DetectProjectLanguages(projectPath string) ([]types.Language, error) {
	args := m.Called(projectPath)
	return args.Get(0).([]types.Language), args.Error(1)
}

func (m *MockLSPServerConfig) DetectPrimaryProjectLanguage(projectPath string) (*types.Language, error) {
	args := m.Called(projectPath)
	return args.Get(0).(*types.Language), args.Error(1)
}

func (m *MockLSPServerConfig) FindServerConfig(language string) (types.LanguageServerConfigProvider, error) {
	args := m.Called(language)
	return args.Get(0).(types.LanguageServerConfigProvider), args.Error(1)
}

func (m *MockLSPServerConfig) FindAllServerConfigs(language string) ([]types.LanguageServerConfigProvider, []types.LanguageServer, error) {
	args := m.Called(language)
	return args.Get(0).([]types.LanguageServerConfigProvider), args.Get(1).([]types.LanguageServer), args.Error(2)
}

func (m *MockLSPServerConfig) GetGlobalConfig() types.GlobalConfig {
	args := m.Called()
	return args.Get(0).(types.GlobalConfig)
}

func (m *MockLSPServerConfig) GetLanguageServers() map[types.LanguageServer]types.LanguageServerConfigProvider {
	args := m.Called()
	return args.Get(0).(map[types.LanguageServer]types.LanguageServerConfigProvider)
}

func (m *MockLSPServerConfig) FindExtLanguage(ext string) (*types.Language, error) {
	args := m.Called(ext)
	return args.Get(0).(*types.Language), args.Error(1)
}

func (m *MockLSPServerConfig) GetServerNameFromLanguage(language types.Language) types.LanguageServer {
	args := m.Called(language)
	return args.Get(0).(types.LanguageServer)
}
