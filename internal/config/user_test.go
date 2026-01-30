package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestParseEnvFile(t *testing.T) {
	// Create a temp env file
	tmpDir, err := os.MkdirTemp("", "config-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	envContent := `# Comment line
COMMIT_PROVIDER=anthropic
ANTHROPIC_API_KEY=sk-ant-test123

# Another comment
COMMIT_MODEL=claude-3-5-sonnet
EMPTY_VALUE=
QUOTED_VALUE="quoted string"
SINGLE_QUOTED='single quoted'
`

	envPath := filepath.Join(tmpDir, ".env")
	if err := os.WriteFile(envPath, []byte(envContent), 0600); err != nil {
		t.Fatalf("failed to write env file: %v", err)
	}

	env, err := parseEnvFile(envPath)
	if err != nil {
		t.Fatalf("parseEnvFile failed: %v", err)
	}

	tests := []struct {
		key      string
		expected string
	}{
		{"COMMIT_PROVIDER", "anthropic"},
		{"ANTHROPIC_API_KEY", "sk-ant-test123"},
		{"COMMIT_MODEL", "claude-3-5-sonnet"},
		{"EMPTY_VALUE", ""},
		{"QUOTED_VALUE", "quoted string"},
		{"SINGLE_QUOTED", "single quoted"},
	}

	for _, tt := range tests {
		t.Run(tt.key, func(t *testing.T) {
			if env[tt.key] != tt.expected {
				t.Errorf("expected %s=%q, got %q", tt.key, tt.expected, env[tt.key])
			}
		})
	}
}

func TestLoadUserConfig_MissingFile(t *testing.T) {
	// Save original home and restore after test
	origHome := os.Getenv("HOME")
	tmpDir, _ := os.MkdirTemp("", "config-test-*")
	defer os.RemoveAll(tmpDir)
	os.Setenv("HOME", tmpDir)
	defer os.Setenv("HOME", origHome)

	_, err := LoadUserConfig()
	if err == nil {
		t.Error("expected error for missing config file")
	}

	if _, ok := err.(*ConfigNotFoundError); !ok {
		t.Errorf("expected ConfigNotFoundError, got %T: %v", err, err)
	}
}

func TestLoadUserConfig_MissingProvider(t *testing.T) {
	origHome := os.Getenv("HOME")
	tmpDir, _ := os.MkdirTemp("", "config-test-*")
	defer os.RemoveAll(tmpDir)
	os.Setenv("HOME", tmpDir)
	defer os.Setenv("HOME", origHome)

	// Create config dir and file without provider
	configDir := filepath.Join(tmpDir, ConfigDir)
	os.MkdirAll(configDir, 0700)
	os.WriteFile(filepath.Join(configDir, EnvFile), []byte("ANTHROPIC_API_KEY=test"), 0600)

	_, err := LoadUserConfig()
	if err == nil {
		t.Error("expected error for missing provider")
	}

	if _, ok := err.(*ProviderNotConfiguredError); !ok {
		t.Errorf("expected ProviderNotConfiguredError, got %T: %v", err, err)
	}
}

func TestLoadUserConfig_InvalidProvider(t *testing.T) {
	origHome := os.Getenv("HOME")
	tmpDir, _ := os.MkdirTemp("", "config-test-*")
	defer os.RemoveAll(tmpDir)
	os.Setenv("HOME", tmpDir)
	defer os.Setenv("HOME", origHome)

	configDir := filepath.Join(tmpDir, ConfigDir)
	os.MkdirAll(configDir, 0700)
	os.WriteFile(filepath.Join(configDir, EnvFile), []byte("COMMIT_PROVIDER=invalid"), 0600)

	_, err := LoadUserConfig()
	if err == nil {
		t.Error("expected error for invalid provider")
	}

	if e, ok := err.(*InvalidProviderError); !ok {
		t.Errorf("expected InvalidProviderError, got %T: %v", err, err)
	} else if e.Provider != "invalid" {
		t.Errorf("expected provider 'invalid', got %q", e.Provider)
	}
}

func TestLoadUserConfig_MissingAPIKey(t *testing.T) {
	origHome := os.Getenv("HOME")
	tmpDir, _ := os.MkdirTemp("", "config-test-*")
	defer os.RemoveAll(tmpDir)
	os.Setenv("HOME", tmpDir)
	defer os.Setenv("HOME", origHome)

	configDir := filepath.Join(tmpDir, ConfigDir)
	os.MkdirAll(configDir, 0700)
	os.WriteFile(filepath.Join(configDir, EnvFile), []byte("COMMIT_PROVIDER=anthropic"), 0600)

	_, err := LoadUserConfig()
	if err == nil {
		t.Error("expected error for missing API key")
	}

	if e, ok := err.(*MissingAPIKeyError); !ok {
		t.Errorf("expected MissingAPIKeyError, got %T: %v", err, err)
	} else {
		if e.Provider != "anthropic" {
			t.Errorf("expected provider 'anthropic', got %q", e.Provider)
		}
		if e.EnvVar != "ANTHROPIC_API_KEY" {
			t.Errorf("expected env var 'ANTHROPIC_API_KEY', got %q", e.EnvVar)
		}
	}
}

func TestLoadUserConfig_ValidAnthropicConfig(t *testing.T) {
	origHome := os.Getenv("HOME")
	tmpDir, _ := os.MkdirTemp("", "config-test-*")
	defer os.RemoveAll(tmpDir)
	os.Setenv("HOME", tmpDir)
	defer os.Setenv("HOME", origHome)

	configDir := filepath.Join(tmpDir, ConfigDir)
	os.MkdirAll(configDir, 0700)
	envContent := `COMMIT_PROVIDER=anthropic
ANTHROPIC_API_KEY=sk-ant-test
COMMIT_MODEL=claude-3-haiku
COMMIT_DRY_RUN=true`
	os.WriteFile(filepath.Join(configDir, EnvFile), []byte(envContent), 0600)

	config, err := LoadUserConfig()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if config.Provider != "anthropic" {
		t.Errorf("expected provider 'anthropic', got %q", config.Provider)
	}
	if config.AnthropicAPIKey != "sk-ant-test" {
		t.Errorf("expected API key 'sk-ant-test', got %q", config.AnthropicAPIKey)
	}
	if config.Model != "claude-3-haiku" {
		t.Errorf("expected model 'claude-3-haiku', got %q", config.Model)
	}
	if !config.DryRun {
		t.Error("expected DryRun to be true")
	}
}

func TestLoadUserConfig_ValidAzureFoundryConfig(t *testing.T) {
	origHome := os.Getenv("HOME")
	tmpDir, _ := os.MkdirTemp("", "config-test-*")
	defer os.RemoveAll(tmpDir)
	os.Setenv("HOME", tmpDir)
	defer os.Setenv("HOME", origHome)

	configDir := filepath.Join(tmpDir, ConfigDir)
	os.MkdirAll(configDir, 0700)
	envContent := `COMMIT_PROVIDER=azure-foundry
AZURE_FOUNDRY_ENDPOINT=https://test.openai.azure.com
AZURE_FOUNDRY_API_KEY=test-key
AZURE_FOUNDRY_DEPLOYMENT=gpt-4`
	os.WriteFile(filepath.Join(configDir, EnvFile), []byte(envContent), 0600)

	config, err := LoadUserConfig()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if config.Provider != "azure-foundry" {
		t.Errorf("expected provider 'azure-foundry', got %q", config.Provider)
	}
	if config.AzureFoundryEndpoint != "https://test.openai.azure.com" {
		t.Errorf("unexpected endpoint: %q", config.AzureFoundryEndpoint)
	}
	if config.AzureFoundryAPIKey != "test-key" {
		t.Errorf("unexpected API key: %q", config.AzureFoundryAPIKey)
	}
	if config.AzureFoundryDeployment != "gpt-4" {
		t.Errorf("unexpected deployment: %q", config.AzureFoundryDeployment)
	}
}

func TestEnsureConfigDir(t *testing.T) {
	origHome := os.Getenv("HOME")
	tmpDir, _ := os.MkdirTemp("", "config-test-*")
	defer os.RemoveAll(tmpDir)
	os.Setenv("HOME", tmpDir)
	defer os.Setenv("HOME", origHome)

	err := EnsureConfigDir()
	if err != nil {
		t.Fatalf("EnsureConfigDir failed: %v", err)
	}

	// Check that directories were created
	configDir := filepath.Join(tmpDir, ConfigDir)
	if _, err := os.Stat(configDir); os.IsNotExist(err) {
		t.Error("config directory was not created")
	}

	logsDir := filepath.Join(configDir, "logs", "executions")
	if _, err := os.Stat(logsDir); os.IsNotExist(err) {
		t.Error("logs directory was not created")
	}
}

func TestCreateDefaultConfig(t *testing.T) {
	origHome := os.Getenv("HOME")
	tmpDir, _ := os.MkdirTemp("", "config-test-*")
	defer os.RemoveAll(tmpDir)
	os.Setenv("HOME", tmpDir)
	defer os.Setenv("HOME", origHome)

	err := CreateDefaultConfig()
	if err != nil {
		t.Fatalf("CreateDefaultConfig failed: %v", err)
	}

	// Check that the file was created
	envPath := filepath.Join(tmpDir, ConfigDir, EnvFile)
	content, err := os.ReadFile(envPath)
	if err != nil {
		t.Fatalf("failed to read created config: %v", err)
	}

	// Check for expected content
	if len(content) == 0 {
		t.Error("config file is empty")
	}

	contentStr := string(content)
	if !contains(contentStr, "COMMIT_PROVIDER") {
		t.Error("config file missing COMMIT_PROVIDER")
	}
	if !contains(contentStr, "ANTHROPIC_API_KEY") {
		t.Error("config file missing ANTHROPIC_API_KEY")
	}
}

func TestCreateDefaultConfig_DoesNotOverwrite(t *testing.T) {
	origHome := os.Getenv("HOME")
	tmpDir, _ := os.MkdirTemp("", "config-test-*")
	defer os.RemoveAll(tmpDir)
	os.Setenv("HOME", tmpDir)
	defer os.Setenv("HOME", origHome)

	// Create existing config
	configDir := filepath.Join(tmpDir, ConfigDir)
	os.MkdirAll(configDir, 0700)
	existingContent := "EXISTING=content"
	envPath := filepath.Join(configDir, EnvFile)
	os.WriteFile(envPath, []byte(existingContent), 0600)

	// Try to create default config
	err := CreateDefaultConfig()
	if err != nil {
		t.Fatalf("CreateDefaultConfig failed: %v", err)
	}

	// Verify content was not overwritten
	content, _ := os.ReadFile(envPath)
	if string(content) != existingContent {
		t.Errorf("config was overwritten: expected %q, got %q", existingContent, string(content))
	}
}

func TestErrorTypes(t *testing.T) {
	tests := []struct {
		name     string
		err      error
		expected string
	}{
		{
			name:     "ConfigNotFoundError",
			err:      &ConfigNotFoundError{Path: "/path/to/config"},
			expected: "config file not found: /path/to/config",
		},
		{
			name:     "ProviderNotConfiguredError",
			err:      &ProviderNotConfiguredError{},
			expected: "no provider configured. Set COMMIT_PROVIDER in ~/.commit-tool/.env",
		},
		{
			name:     "InvalidProviderError",
			err:      &InvalidProviderError{Provider: "bad"},
			expected: "invalid provider \"bad\". Supported: [anthropic openai grok gemini azure-foundry]",
		},
		{
			name:     "MissingAPIKeyError",
			err:      &MissingAPIKeyError{Provider: "openai", EnvVar: "OPENAI_API_KEY"},
			expected: "missing API key for provider \"openai\". Set OPENAI_API_KEY in ~/.commit-tool/.env",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.err.Error() != tt.expected {
				t.Errorf("expected %q, got %q", tt.expected, tt.err.Error())
			}
		})
	}
}

func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(s) > 0 && containsAt(s, substr, 0))
}

func containsAt(s, substr string, start int) bool {
	for i := start; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
