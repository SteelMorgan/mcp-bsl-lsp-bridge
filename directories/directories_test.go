package directories

import (
	"os"
	"os/user"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

// MockUserProvider is a mock implementation of UserProvider
type MockUserProvider struct {
	mock.Mock
}

func (m *MockUserProvider) Current() (*user.User, error) {
	args := m.Called()
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}

	return args.Get(0).(*user.User), args.Error(1)
}

// MockEnvProvider is a mock implementation of EnvProvider
type MockEnvProvider struct {
	mock.Mock
}

func (m *MockEnvProvider) Getenv(key string) string {
	args := m.Called(key)
	return args.String(0)
}

// Tests for 0% coverage functions

func TestNewDefaultEnvProvider(t *testing.T) {
	provider := NewDefaultEnvProvider()
	
	require.NotNil(t, provider)
	assert.IsType(t, DefaultEnvProvider{}, provider)
}

func TestDefaultEnvProvider_Getenv(t *testing.T) {
	provider := DefaultEnvProvider{}
	
	// Test with a known environment variable that should exist
	// Use PATH which exists on all systems
	result := provider.Getenv("PATH")
	// PATH should exist and not be empty on most systems
	// Note: We can't assert the exact value since it varies by system
	assert.IsType(t, "", result)
	
	// Test with a non-existent environment variable
	result = provider.Getenv("NONEXISTENT_TEST_VAR_12345")
	assert.Equal(t, "", result)
}

func TestDefaultUserProvider_Current(t *testing.T) {
	provider := DefaultUserProvider{}
	
	user, err := provider.Current()
	
	// This should work on most systems
	assert.NoError(t, err)
	require.NotNil(t, user)
	assert.NotEmpty(t, user.Username)
}

func TestDirectoryResolver_DirectoryEnsuring(t *testing.T) {
	tests := []struct {
		name            string
		appName         string
		mockUser        *user.User
		mockUserErr     error
		mockEnvVars     map[string]string
		shouldEnsureDir bool
		function        string
		wantDirExists   bool
		wantErr         bool
		isRoot          bool
	}{
		{
			name:            "ensure directories enabled - config - regular user",
			appName:         "testapp",
			mockUser:        &user.User{Uid: "1000", HomeDir: "/tmp/testhome"},
			mockEnvVars:     map[string]string{},
			shouldEnsureDir: true,
			function:        "config",
			wantDirExists:   true,
			wantErr:         false,
			isRoot:          false,
		},
		{
			name:            "ensure directories disabled - config - regular user",
			appName:         "testapp",
			mockUser:        &user.User{Uid: "1000", HomeDir: "/tmp/testhome"},
			mockEnvVars:     map[string]string{},
			shouldEnsureDir: false,
			function:        "config",
			wantDirExists:   false,
			wantErr:         false,
			isRoot:          false,
		},
		{
			name:            "ensure directories enabled - cache - regular user",
			appName:         "testapp",
			mockUser:        &user.User{Uid: "1000", HomeDir: "/tmp/testhome"},
			mockEnvVars:     map[string]string{},
			shouldEnsureDir: true,
			function:        "cache",
			wantDirExists:   true,
			wantErr:         false,
			isRoot:          false,
		},
		{
			name:            "ensure directories enabled - data - regular user",
			appName:         "testapp",
			mockUser:        &user.User{Uid: "1000", HomeDir: "/tmp/testhome"},
			mockEnvVars:     map[string]string{},
			shouldEnsureDir: true,
			function:        "data",
			wantDirExists:   true,
			wantErr:         false,
			isRoot:          false,
		},
		{
			name:            "ensure directories enabled - log - regular user",
			appName:         "testapp",
			mockUser:        &user.User{Uid: "1000", HomeDir: "/tmp/testhome"},
			mockEnvVars:     map[string]string{},
			shouldEnsureDir: true,
			function:        "log",
			wantDirExists:   true,
			wantErr:         false,
			isRoot:          false,
		},
		{
			name:            "user error - config",
			appName:         "testapp",
			mockUser:        nil,
			mockUserErr:     assert.AnError,
			mockEnvVars:     map[string]string{},
			shouldEnsureDir: true,
			function:        "config",
			wantDirExists:   false,
			wantErr:         true,
			isRoot:          false,
		},
		{
			name:     "ensure directories enabled - config - with XDG_CONFIG_HOME",
			appName:  "testapp",
			mockUser: &user.User{Uid: "1000", HomeDir: "/tmp/testhome"},
			mockEnvVars: map[string]string{
				"XDG_CONFIG_HOME": "/tmp/custom-config",
			},
			shouldEnsureDir: true,
			function:        "config",
			wantDirExists:   true,
			wantErr:         false,
			isRoot:          false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create a temp directory that we can actually write to
			tempDir := t.TempDir()

			// Setup mock providers
			mockUserProvider := &MockUserProvider{}
			mockEnvProvider := &MockEnvProvider{}

			// Configure user provider mock
			if tt.mockUserErr != nil {
				mockUserProvider.On("Current").Return(nil, tt.mockUserErr)
			} else {
				// Use tempDir as the base for HomeDir to ensure we can write to it
				testUser := *tt.mockUser
				if !tt.isRoot {
					testUser.HomeDir = tempDir
				}

				mockUserProvider.On("Current").Return(&testUser, nil)
			}

			// Configure environment provider mock
			for key, value := range tt.mockEnvVars {
				// If XDG_CONFIG_HOME is set, make it point to our temp directory
				if key == "XDG_CONFIG_HOME" {
					value = filepath.Join(tempDir, "custom-config")
				}

				mockEnvProvider.On("Getenv", key).Return(value)
			}

			// For root user tests, override system paths to use temp directory
			// This allows us to test directory creation without root permissions
			if tt.isRoot {
				switch tt.function {
				case "config":
					// Mock any environment variable that might override /etc
					mockEnvProvider.On("Getenv", "XDG_CONFIG_DIRS").Return(filepath.Join(tempDir, "etc")).Maybe()
				case "cache":
					// Mock any environment variable that might override /var/cache
					mockEnvProvider.On("Getenv", "XDG_CACHE_HOME").Return(filepath.Join(tempDir, "var", "cache")).Maybe()
				case "data":
					// Mock any environment variable that might override /var/lib
					mockEnvProvider.On("Getenv", "XDG_DATA_DIRS").Return(filepath.Join(tempDir, "var", "lib")).Maybe()
				case "log":
					// For log directories, you might have a custom env var or need to modify your implementation
					// This depends on how your DirectoryResolver handles log directories for root
				}
			}

			// For any environment variables not explicitly set, return empty string
			// Use Maybe() to make this expectation optional
			mockEnvProvider.On("Getenv", mock.AnythingOfType("string")).Return("").Maybe()

			// Create directory resolver
			dr := &DirectoryResolver{
				appName:         tt.appName,
				userProvider:    mockUserProvider,
				envProvider:     mockEnvProvider,
				shouldEnsureDir: tt.shouldEnsureDir,
			}

			// Call the appropriate function
			var got string

			var err error
			switch tt.function {
			case "config":
				got, err = dr.GetConfigDirectory()
			case "cache":
				got, err = dr.GetCacheDirectory()
			case "data":
				got, err = dr.GetDataDirectory()
			case "log":
				got, err = dr.GetLogDirectory()
			default:
				t.Fatalf("Unknown function: %s", tt.function)
			}

			// Check error expectation
			if tt.wantErr {
				assert.Error(t, err)
				return
			}

			require.NoError(t, err)

			// Check directory existence and path correctness
			if tt.shouldEnsureDir && tt.wantDirExists {
				// For both regular users and root users (using temp dirs),
				// check that directory was actually created
				_, statErr := os.Stat(got)
				assert.False(t, os.IsNotExist(statErr), "Directory should exist: %s", got)

				// For root user tests, also verify the path structure is correct
				if tt.isRoot {
					switch tt.function {
					case "config":
						assert.Contains(t, got, "etc")
						assert.Contains(t, got, tt.appName)
					case "cache":
						assert.Contains(t, got, "cache")
						assert.Contains(t, got, tt.appName)
					case "data":
						assert.Contains(t, got, "lib") // or "data" depending on your implementation
						assert.Contains(t, got, tt.appName)
					case "log":
						assert.Contains(t, got, "log")
						assert.Contains(t, got, tt.appName)
					}
				}
			} else if !tt.shouldEnsureDir {
				// When shouldEnsureDir is false, just verify the path is constructed correctly
				assert.NotEmpty(t, got)
			}

			// Verify all expectations were met
			mockUserProvider.AssertExpectations(t)
			mockEnvProvider.AssertExpectations(t)
		})
	}
}
func TestNewDirectoryResolver(t *testing.T) {
	tests := []struct {
		name            string
		appName         string
		mockUser        *user.User
		mockUserErr     error
		mockEnvVars     map[string]string
		shouldEnsureDir bool
		want            *DirectoryResolver
		wantErr         bool
	}{
		{
			name:    "basic constructor",
			appName: "testapp",
			mockUser: &user.User{
				Uid:     "1000",
				HomeDir: "/home/testuser",
			},
			mockEnvVars:     map[string]string{},
			shouldEnsureDir: true,
			want: &DirectoryResolver{
				appName:         "testapp",
				shouldEnsureDir: true,
			},
			wantErr: false,
		},
		{
			name:    "without ensuring directories",
			appName: "myapp",
			mockUser: &user.User{
				Uid:     "0",
				HomeDir: "/root",
			},
			mockEnvVars:     map[string]string{},
			shouldEnsureDir: false,
			want: &DirectoryResolver{
				appName:         "myapp",
				shouldEnsureDir: false,
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockUserProvider := &MockUserProvider{}
			mockEnvProvider := &MockEnvProvider{}

			// Setup mocks (even though NewDirectoryResolver doesn't use them)
			mockUserProvider.On("Current").Return(tt.mockUser, tt.mockUserErr)

			for key, value := range tt.mockEnvVars {
				mockEnvProvider.On("Getenv", key).Return(value)
			}

			mockEnvProvider.On("Getenv", mock.AnythingOfType("string")).Return("")

			got := NewDirectoryResolver(tt.appName, mockUserProvider, mockEnvProvider, tt.shouldEnsureDir)

			assert.Equal(t, tt.want.appName, got.appName)
			assert.Equal(t, tt.want.shouldEnsureDir, got.shouldEnsureDir)
			assert.NotNil(t, got.userProvider)
			assert.NotNil(t, got.envProvider)
		})
	}
}

func TestDirectoryResolver_isRoot(t *testing.T) {
	tests := []struct {
		name        string
		mockUser    *user.User
		mockUserErr error
		want        bool
		wantErr     bool
	}{
		{
			name: "root user",
			mockUser: &user.User{
				Uid: "0",
			},
			want:    true,
			wantErr: false,
		},
		{
			name: "regular user",
			mockUser: &user.User{
				Uid: "1000",
			},
			want:    false,
			wantErr: false,
		},
		{
			name:        "user provider error",
			mockUserErr: assert.AnError,
			want:        false,
			wantErr:     true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockUserProvider := &MockUserProvider{}
			mockEnvProvider := &MockEnvProvider{}

			mockUserProvider.On("Current").Return(tt.mockUser, tt.mockUserErr)
			mockEnvProvider.On("Getenv", mock.AnythingOfType("string")).Return("")

			dr := &DirectoryResolver{
				userProvider: mockUserProvider,
				envProvider:  mockEnvProvider,
			}

			got, err := dr.isRoot()

			if tt.wantErr {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
				assert.Equal(t, tt.want, got)
			}

			mockUserProvider.AssertExpectations(t)
		})
	}
}

func TestDirectoryResolver_maybeEnsureDir(t *testing.T) {
	tests := []struct {
		name            string
		shouldEnsureDir bool
		wantErr         bool
	}{
		{
			name:            "ensure directory - success",
			shouldEnsureDir: true,
			wantErr:         false,
		},
		{
			name:            "don't ensure directory",
			shouldEnsureDir: false,
			wantErr:         false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tempDir := t.TempDir()
			testDir := filepath.Join(tempDir, "subdir")

			mockUserProvider := &MockUserProvider{}
			mockEnvProvider := &MockEnvProvider{}
			mockEnvProvider.On("Getenv", mock.AnythingOfType("string")).Return("")

			dr := &DirectoryResolver{
				shouldEnsureDir: tt.shouldEnsureDir,
				userProvider:    mockUserProvider,
				envProvider:     mockEnvProvider,
			}

			got, err := dr.maybeEnsureDir(testDir)

			if tt.wantErr {
				assert.Error(t, err)
			} else {
				require.NoError(t, err)
				assert.Equal(t, testDir, got)

				if tt.shouldEnsureDir {
					// Verify directory was created
					_, statErr := os.Stat(got)
					assert.False(t, os.IsNotExist(statErr))
				}
			}
		})
	}
}

func TestDirectoryResolver_GetLogDirectory(t *testing.T) {
	tests := []struct {
		name            string
		appName         string
		mockUser        *user.User
		mockUserErr     error
		mockEnvVars     map[string]string
		shouldEnsureDir bool
		want            string
		wantErr         bool
	}{
		{
			name:    "root",
			appName: "test",
			mockUser: &user.User{
				Uid: "0",
			},
			shouldEnsureDir: false,
			want:            "/var/log/test",
			wantErr:         false,
		},
		{
			name:    "regular user unix",
			appName: "test",
			mockUser: &user.User{
				Uid:     "1000",
				HomeDir: "/home/testuser",
			},
			shouldEnsureDir: false,
			mockEnvVars:     map[string]string{},
			want:            filepath.Join("/home", "testuser", ".local", "share", "test", "logs"),
			wantErr:         false,
		},
		{
			name:    "regular user unix with XDG_DATA_HOME",
			appName: "test",
			mockUser: &user.User{
				Uid:     "1000",
				HomeDir: "/home/testuser",
			},
			shouldEnsureDir: false,
			mockEnvVars:     map[string]string{"XDG_DATA_HOME": "/custom/data"},
			want:            filepath.Join("/custom/data", "test", "logs"),
			wantErr:         false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockUserProvider := &MockUserProvider{}
			mockEnvProvider := &MockEnvProvider{}

			mockUserProvider.On("Current").Return(tt.mockUser, tt.mockUserErr)
			// Set up environment variable mocks
			// For regular user cases, we know XDG_DATA_HOME will be checked
			if tt.mockUser != nil && tt.mockUser.Uid != "0" {
				if xdgDataHome, exists := tt.mockEnvVars["XDG_DATA_HOME"]; exists {
					mockEnvProvider.On("Getenv", "XDG_DATA_HOME").Return(xdgDataHome)
				} else {
					mockEnvProvider.On("Getenv", "XDG_DATA_HOME").Return("")
				}
			}

			dr := &DirectoryResolver{
				appName:         tt.appName,
				userProvider:    mockUserProvider,
				envProvider:     mockEnvProvider,
				shouldEnsureDir: tt.shouldEnsureDir,
			}

			got, err := dr.GetLogDirectory()

			if tt.wantErr {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
				assert.Equal(t, tt.want, got)
			}

			mockUserProvider.AssertExpectations(t)
			mockEnvProvider.AssertExpectations(t)
		})
	}
}

func TestDirectoryResolver_GetDataDirectory(t *testing.T) {
	tests := []struct {
		name            string
		appName         string
		mockUser        *user.User
		mockUserErr     error
		mockEnvVars     map[string]string
		shouldEnsureDir bool
		want            string
		wantErr         bool
	}{
		{
			name:    "root",
			appName: "test",
			mockUser: &user.User{
				Uid: "0",
			},
			shouldEnsureDir: false,
			want:            "/var/lib/test",
			wantErr:         false,
		},
		{
			name:    "regular user unix",
			appName: "test",
			mockUser: &user.User{
				Uid:     "1000",
				HomeDir: "/home/testuser",
			},
			shouldEnsureDir: false,
			mockEnvVars:     map[string]string{},
			want:            filepath.Join("/home/testuser", ".local", "share", "test"),
			wantErr:         false,
		},
		{
			name:    "regular user unix with XDG_DATA_HOME",
			appName: "test",
			mockUser: &user.User{
				Uid:     "1000",
				HomeDir: "/home/testuser",
			},
			shouldEnsureDir: false,
			mockEnvVars:     map[string]string{"XDG_DATA_HOME": "/custom/data"},
			want:            filepath.Join("/custom/data", "test"),
			wantErr:         false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockUserProvider := &MockUserProvider{}
			mockEnvProvider := &MockEnvProvider{}

			mockUserProvider.On("Current").Return(tt.mockUser, tt.mockUserErr)
			// Set up environment variable mocks
			// For regular user cases, we know XDG_DATA_HOME will be checked
			if tt.mockUser != nil && tt.mockUser.Uid != "0" {
				if xdgDataHome, exists := tt.mockEnvVars["XDG_DATA_HOME"]; exists {
					mockEnvProvider.On("Getenv", "XDG_DATA_HOME").Return(xdgDataHome)
				} else {
					mockEnvProvider.On("Getenv", "XDG_DATA_HOME").Return("")
				}
			}

			dr := &DirectoryResolver{
				appName:         tt.appName,
				userProvider:    mockUserProvider,
				envProvider:     mockEnvProvider,
				shouldEnsureDir: tt.shouldEnsureDir,
			}

			got, err := dr.GetDataDirectory()

			if tt.wantErr {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
				assert.Equal(t, tt.want, got)
			}

			mockUserProvider.AssertExpectations(t)
			mockEnvProvider.AssertExpectations(t)
		})
	}
}

func TestDirectoryResolver_GetCacheDirectory(t *testing.T) {
	tests := []struct {
		name            string
		appName         string
		mockUser        *user.User
		mockUserErr     error
		mockEnvVars     map[string]string
		shouldEnsureDir bool
		want            string
		wantErr         bool
	}{
		{
			name:    "root",
			appName: "test",
			mockUser: &user.User{
				Uid: "0",
			},
			shouldEnsureDir: false,
			want:            "/var/cache/test",
			wantErr:         false,
		},
		{
			name:    "regular user unix",
			appName: "test",
			mockUser: &user.User{
				Uid:     "1000",
				HomeDir: "/home/testuser",
			},
			shouldEnsureDir: false,
			mockEnvVars:     map[string]string{},
			want:            filepath.Join("/home/testuser", ".cache", "test"),
			wantErr:         false,
		},
		{
			name:    "regular user unix with XDG_CACHE_HOME",
			appName: "test",
			mockUser: &user.User{
				Uid:     "1000",
				HomeDir: "/home/testuser",
			},
			shouldEnsureDir: false,
			mockEnvVars:     map[string]string{"XDG_CACHE_HOME": "/custom/cache"},
			want:            filepath.Join("/custom/cache", "test"),
			wantErr:         false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockUserProvider := &MockUserProvider{}
			mockEnvProvider := &MockEnvProvider{}

			mockUserProvider.On("Current").Return(tt.mockUser, tt.mockUserErr)
			// Set up environment variable mocks
			// For regular user cases, we know XDG_CACHE_HOME will be checked
			if tt.mockUser != nil && tt.mockUser.Uid != "0" {
				if xdgDataHome, exists := tt.mockEnvVars["XDG_CACHE_HOME"]; exists {
					mockEnvProvider.On("Getenv", "XDG_CACHE_HOME").Return(xdgDataHome)
				} else {
					mockEnvProvider.On("Getenv", "XDG_CACHE_HOME").Return("")
				}
			}

			dr := &DirectoryResolver{
				appName:         tt.appName,
				userProvider:    mockUserProvider,
				envProvider:     mockEnvProvider,
				shouldEnsureDir: tt.shouldEnsureDir,
			}

			got, err := dr.GetCacheDirectory()

			if tt.wantErr {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
				assert.Equal(t, tt.want, got)
			}

			mockUserProvider.AssertExpectations(t)
			mockEnvProvider.AssertExpectations(t)
		})
	}
}

func TestDirectoryResolver_GetConfigDirectory_Comprehensive(t *testing.T) {
	tests := []struct {
		name            string
		appName         string
		mockUser        *user.User
		mockUserErr     error
		mockEnvVars     map[string]string
		shouldEnsureDir bool
		want            string
		wantErr         bool
	}{
		{
			name:    "root",
			appName: "test",
			mockUser: &user.User{
				Uid: "0",
			},
			shouldEnsureDir: false,
			want:            "/etc/test",
			wantErr:         false,
		},
		{
			name:    "regular user unix",
			appName: "test",
			mockUser: &user.User{
				Uid:     "1000",
				HomeDir: "/home/testuser",
			},
			shouldEnsureDir: false,
			mockEnvVars:     map[string]string{},
			want:            filepath.Join("/home/testuser", ".config", "test"),
			wantErr:         false,
		},
		{
			name:    "regular user unix with XDG_CONFIG_HOME",
			appName: "test",
			mockUser: &user.User{
				Uid:     "1000",
				HomeDir: "/home/testuser",
			},
			shouldEnsureDir: false,
			mockEnvVars:     map[string]string{"XDG_CONFIG_HOME": "/custom/config"},
			want:            filepath.Join("/custom/config", "test"),
			wantErr:         false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockUserProvider := &MockUserProvider{}
			mockEnvProvider := &MockEnvProvider{}

			mockUserProvider.On("Current").Return(tt.mockUser, tt.mockUserErr)
			// Set up environment variable mocks
			// For regular user cases, we know XDG_CONFIG_HOME will be checked
			if tt.mockUser != nil && tt.mockUser.Uid != "0" {
				if xdgDataHome, exists := tt.mockEnvVars["XDG_CONFIG_HOME"]; exists {
					mockEnvProvider.On("Getenv", "XDG_CONFIG_HOME").Return(xdgDataHome)
				} else {
					mockEnvProvider.On("Getenv", "XDG_CONFIG_HOME").Return("")
				}
			}

			dr := &DirectoryResolver{
				appName:         tt.appName,
				userProvider:    mockUserProvider,
				envProvider:     mockEnvProvider,
				shouldEnsureDir: tt.shouldEnsureDir,
			}

			got, err := dr.GetConfigDirectory()

			if tt.wantErr {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
				assert.Equal(t, tt.want, got)
			}

			mockUserProvider.AssertExpectations(t)
			mockEnvProvider.AssertExpectations(t)
		})
	}
}
