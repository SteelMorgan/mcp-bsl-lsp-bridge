package tools

import (
	"os"
	"path/filepath"
	"testing"

	"rockerboo/mcp-lsp-bridge/mocks"
	"rockerboo/mcp-lsp-bridge/utils"

	"github.com/myleshyson/lsprotocol-go/protocol"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestResolveFileContext tests the core file resolution functionality
func TestResolveFileContext(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "file_context_test")
	require.NoError(t, err)
	defer func() {
		require.NoError(t, os.RemoveAll(tempDir))
	}()

	// Create minimal test structure
	cmdDir := filepath.Join(tempDir, "cmd")
	require.NoError(t, os.MkdirAll(cmdDir, 0750))

	// Create test files
	mainFile := filepath.Join(tempDir, "main.go")
	cmdFile := filepath.Join(cmdDir, "auth.go")
	require.NoError(t, os.WriteFile(mainFile, []byte("// test"), 0600))
	require.NoError(t, os.WriteFile(cmdFile, []byte("// test"), 0600))

	testCases := []struct {
		name           string
		fileContext    string
		expectResolved bool
		expectedPath   string
	}{
		{
			name:           "exact file in root",
			fileContext:    "main.go",
			expectResolved: true,
			expectedPath:   mainFile,
		},
		{
			name:           "file in subdirectory",
			fileContext:    "auth.go",
			expectResolved: true,
			expectedPath:   cmdFile,
		},
		{
			name:           "extension inference",
			fileContext:    "auth",
			expectResolved: true,
			expectedPath:   cmdFile,
		},
		{
			name:           "file not found",
			fileContext:    "nonexistent.py",
			expectResolved: false,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			bridge := &mocks.MockBridge{}

			result, err := ResolveFileContext(bridge, tc.fileContext, tempDir)
			require.NoError(t, err)

			if tc.expectResolved {
				assert.Equal(t, tc.expectedPath, result.ResolvedPath)
				assert.Empty(t, result.ErrorMessage)
			} else {
				assert.Empty(t, result.ResolvedPath)
				assert.NotEmpty(t, result.ErrorMessage)
			}
		})
	}
}

// TestFilterSymbolsByFileContext tests the integration with symbol filtering
func TestFilterSymbolsByFileContext(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "symbol_filter_test")
	require.NoError(t, err)
	defer func() {
		require.NoError(t, os.RemoveAll(tempDir))
	}()

	testFile := filepath.Join(tempDir, "test.go")
	require.NoError(t, os.WriteFile(testFile, []byte("// test"), 0600))

	symbols := []SymbolMatch{
		{Name: "TestFunction", Location: protocol.Location{Uri: protocol.DocumentUri(utils.FilePathToURI(testFile))}},
		{Name: "OtherFunction", Location: protocol.Location{Uri: protocol.DocumentUri("file:///other/other.go")}},
	}

	bridge := &mocks.MockBridge{}
	bridge.On("AllowedDirectories").Return([]string{tempDir})

	// Test existing file filters correctly
	result, err := filterSymbolsByFileContext(bridge, symbols, "test.go")
	require.NoError(t, err)
	assert.Len(t, result, 1)

	// Test empty context returns all
	result, err = filterSymbolsByFileContext(bridge, symbols, "")
	require.NoError(t, err)
	assert.Len(t, result, 2)

	bridge.AssertExpectations(t)
}
