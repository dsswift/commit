// Package config handles loading user and repository configuration.
package config

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/dsswift/commit/internal/assert"
	"github.com/dsswift/commit/pkg/types"
)

const (
	// ConfigDir is the directory name for commit tool config.
	ConfigDir = ".commit-tool"
	// EnvFile is the name of the environment config file.
	EnvFile = ".env"
)

// ValidProviders is the list of supported LLM providers.
var ValidProviders = []string{"anthropic", "openai", "grok", "gemini", "azure-foundry"}

// ConfigPath returns the full path to the user's config directory.
func ConfigPath() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("failed to get home directory: %w", err)
	}
	return filepath.Join(home, ConfigDir), nil
}

// EnsureConfigDir creates the config directory if it doesn't exist.
func EnsureConfigDir() error {
	path, err := ConfigPath()
	if err != nil {
		return err
	}

	if err := os.MkdirAll(path, 0700); err != nil {
		return fmt.Errorf("failed to create config directory: %w", err)
	}

	// Create logs directory too
	logsPath := filepath.Join(path, "logs", "executions")
	if err := os.MkdirAll(logsPath, 0700); err != nil {
		return fmt.Errorf("failed to create logs directory: %w", err)
	}

	return nil
}

// LoadUserConfig loads the user configuration from ~/.commit-tool/.env.
func LoadUserConfig() (*types.UserConfig, error) {
	configPath, err := ConfigPath()
	if err != nil {
		return nil, err
	}

	envPath := filepath.Join(configPath, EnvFile)

	// Check if config file exists
	if _, err := os.Stat(envPath); os.IsNotExist(err) {
		return nil, &ConfigNotFoundError{Path: envPath}
	}

	env, err := parseEnvFile(envPath)
	if err != nil {
		return nil, fmt.Errorf("failed to parse config file: %w", err)
	}

	config := &types.UserConfig{
		Provider: env["COMMIT_PROVIDER"],
		Model:    env["COMMIT_MODEL"],
		DryRun:   strings.ToLower(env["COMMIT_DRY_RUN"]) == "true",

		AnthropicAPIKey: env["ANTHROPIC_API_KEY"],
		OpenAIAPIKey:    env["OPENAI_API_KEY"],
		GrokAPIKey:      env["GROK_API_KEY"],
		GeminiAPIKey:    env["GEMINI_API_KEY"],

		AzureFoundryEndpoint:   env["AZURE_FOUNDRY_ENDPOINT"],
		AzureFoundryAPIKey:     env["AZURE_FOUNDRY_API_KEY"],
		AzureFoundryDeployment: env["AZURE_FOUNDRY_DEPLOYMENT"],
	}

	// Validate provider is set
	if config.Provider == "" {
		return nil, &ProviderNotConfiguredError{}
	}

	// Validate provider is supported
	validProvider := false
	for _, p := range ValidProviders {
		if p == config.Provider {
			validProvider = true
			break
		}
	}
	if !validProvider {
		return nil, &InvalidProviderError{Provider: config.Provider}
	}

	// Validate API key is set for the provider
	if err := validateAPIKey(config); err != nil {
		return nil, err
	}

	return config, nil
}

// validateAPIKey ensures the appropriate API key is set for the configured provider.
func validateAPIKey(config *types.UserConfig) error {
	switch config.Provider {
	case "anthropic":
		if config.AnthropicAPIKey == "" {
			return &MissingAPIKeyError{Provider: "anthropic", EnvVar: "ANTHROPIC_API_KEY"}
		}
	case "openai":
		if config.OpenAIAPIKey == "" {
			return &MissingAPIKeyError{Provider: "openai", EnvVar: "OPENAI_API_KEY"}
		}
	case "grok":
		if config.GrokAPIKey == "" {
			return &MissingAPIKeyError{Provider: "grok", EnvVar: "GROK_API_KEY"}
		}
	case "gemini":
		if config.GeminiAPIKey == "" {
			return &MissingAPIKeyError{Provider: "gemini", EnvVar: "GEMINI_API_KEY"}
		}
	case "azure-foundry":
		if config.AzureFoundryEndpoint == "" {
			return &MissingAPIKeyError{Provider: "azure-foundry", EnvVar: "AZURE_FOUNDRY_ENDPOINT"}
		}
		if config.AzureFoundryAPIKey == "" {
			return &MissingAPIKeyError{Provider: "azure-foundry", EnvVar: "AZURE_FOUNDRY_API_KEY"}
		}
		if config.AzureFoundryDeployment == "" {
			return &MissingAPIKeyError{Provider: "azure-foundry", EnvVar: "AZURE_FOUNDRY_DEPLOYMENT"}
		}
	}
	return nil
}

// parseEnvFile parses a .env file into a map.
func parseEnvFile(path string) (map[string]string, error) {
	assert.NotEmptyString(path, "env file path cannot be empty")

	file, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	env := make(map[string]string)
	scanner := bufio.NewScanner(file)

	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())

		// Skip empty lines and comments
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}

		// Parse KEY=value
		parts := strings.SplitN(line, "=", 2)
		if len(parts) != 2 {
			continue
		}

		key := strings.TrimSpace(parts[0])
		value := strings.TrimSpace(parts[1])

		// Remove surrounding quotes if present
		if len(value) >= 2 {
			if (value[0] == '"' && value[len(value)-1] == '"') ||
				(value[0] == '\'' && value[len(value)-1] == '\'') {
				value = value[1 : len(value)-1]
			}
		}

		env[key] = value
	}

	if err := scanner.Err(); err != nil {
		return nil, err
	}

	return env, nil
}

// CreateDefaultConfig creates a default .env template file.
func CreateDefaultConfig() error {
	configPath, err := ConfigPath()
	if err != nil {
		return err
	}

	if err := EnsureConfigDir(); err != nil {
		return err
	}

	envPath := filepath.Join(configPath, EnvFile)

	// Don't overwrite existing config
	if _, err := os.Stat(envPath); err == nil {
		return nil
	}

	template := `# Commit Tool Configuration
# Documentation: https://github.com/dsswift/commit#configuration

# ═══════════════════════════════════════════════════════════════════════════════
# PROVIDER SELECTION (required)
# ═══════════════════════════════════════════════════════════════════════════════
# Choose one: anthropic | openai | grok | gemini | azure-foundry
COMMIT_PROVIDER=

# ═══════════════════════════════════════════════════════════════════════════════
# PUBLIC CLOUD API KEYS (use one matching your provider)
# ═══════════════════════════════════════════════════════════════════════════════
ANTHROPIC_API_KEY=
OPENAI_API_KEY=
GROK_API_KEY=
GEMINI_API_KEY=

# ═══════════════════════════════════════════════════════════════════════════════
# AZURE AI FOUNDRY (private cloud - optional)
# ═══════════════════════════════════════════════════════════════════════════════
# For organizations using Azure-hosted models
AZURE_FOUNDRY_ENDPOINT=
AZURE_FOUNDRY_API_KEY=
AZURE_FOUNDRY_DEPLOYMENT=

# ═══════════════════════════════════════════════════════════════════════════════
# OPTIONAL SETTINGS
# ═══════════════════════════════════════════════════════════════════════════════
# Override default model for your provider
# COMMIT_MODEL=claude-3-5-sonnet

# Always preview without committing (useful for testing)
# COMMIT_DRY_RUN=true
`

	if err := os.WriteFile(envPath, []byte(template), 0600); err != nil {
		return fmt.Errorf("failed to write config file: %w", err)
	}

	return nil
}

// Error types for configuration issues.

// ConfigNotFoundError indicates the config file doesn't exist.
type ConfigNotFoundError struct {
	Path string
}

func (e *ConfigNotFoundError) Error() string {
	return fmt.Sprintf("config file not found: %s", e.Path)
}

// ProviderNotConfiguredError indicates no provider is set.
type ProviderNotConfiguredError struct{}

func (e *ProviderNotConfiguredError) Error() string {
	return "no provider configured. Set COMMIT_PROVIDER in ~/.commit-tool/.env"
}

// InvalidProviderError indicates an unsupported provider.
type InvalidProviderError struct {
	Provider string
}

func (e *InvalidProviderError) Error() string {
	return fmt.Sprintf("invalid provider %q. Supported: %v", e.Provider, ValidProviders)
}

// MissingAPIKeyError indicates a required API key is missing.
type MissingAPIKeyError struct {
	Provider string
	EnvVar   string
}

func (e *MissingAPIKeyError) Error() string {
	return fmt.Sprintf("missing API key for provider %q. Set %s in ~/.commit-tool/.env", e.Provider, e.EnvVar)
}
