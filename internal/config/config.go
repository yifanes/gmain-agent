package config

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
)

const (
	DefaultConfigDir  = ".claude-code"
	DefaultConfigFile = "config.json"
)

// AuthType represents the type of authentication
type AuthType string

const (
	AuthTypeAPIKey AuthType = "api_key"
	AuthTypeBearer AuthType = "bearer"
)

// Config represents the application configuration
type Config struct {
	// API settings
	APIKey    string   `json:"api_key,omitempty"`
	AuthToken string   `json:"auth_token,omitempty"`
	AuthType  AuthType `json:"auth_type,omitempty"`
	BaseURL   string   `json:"base_url,omitempty"`
	Model     string   `json:"model,omitempty"`

	// UI settings
	MaxTokens   int  `json:"max_tokens,omitempty"`
	ColorOutput bool `json:"color_output,omitempty"`

	// Session settings
	AutoSaveSession bool   `json:"auto_save_session,omitempty"`
	SessionDir      string `json:"session_dir,omitempty"`
}

// GetAuthCredential returns the authentication credential and type
func (c *Config) GetAuthCredential() (string, AuthType) {
	if c.AuthToken != "" {
		return c.AuthToken, AuthTypeBearer
	}
	return c.APIKey, AuthTypeAPIKey
}

// DefaultConfig returns the default configuration
func DefaultConfig() *Config {
	return &Config{
		Model:           "claude-sonnet-4-20250514",
		MaxTokens:       8192,
		ColorOutput:     true,
		AutoSaveSession: true,
	}
}

// LoadConfig loads configuration from file and environment
func LoadConfig() (*Config, error) {
	cfg := DefaultConfig()

	// Try to load from config file
	configPath, err := getConfigPath()
	if err == nil {
		if data, err := os.ReadFile(configPath); err == nil {
			if err := json.Unmarshal(data, cfg); err != nil {
				return nil, fmt.Errorf("failed to parse config file: %w", err)
			}
		}
	}

	// Override with environment variables
	// ANTHROPIC_AUTH_TOKEN takes precedence (Bearer token for proxies/custom endpoints)
	if authToken := os.Getenv("ANTHROPIC_AUTH_TOKEN"); authToken != "" {
		cfg.AuthToken = authToken
		cfg.AuthType = AuthTypeBearer
	}

	// ANTHROPIC_API_KEY is the standard API key
	if apiKey := os.Getenv("ANTHROPIC_API_KEY"); apiKey != "" {
		cfg.APIKey = apiKey
		// Only set auth type if not already set by AUTH_TOKEN
		if cfg.AuthType == "" {
			cfg.AuthType = AuthTypeAPIKey
		}
	}

	// ANTHROPIC_BASE_URL for custom API endpoints
	if baseURL := os.Getenv("ANTHROPIC_BASE_URL"); baseURL != "" {
		cfg.BaseURL = baseURL
	}

	if model := os.Getenv("CLAUDE_MODEL"); model != "" {
		cfg.Model = model
	}

	return cfg, nil
}

// SaveConfig saves the configuration to file
func SaveConfig(cfg *Config) error {
	configPath, err := getConfigPath()
	if err != nil {
		return err
	}

	// Create directory if needed
	dir := filepath.Dir(configPath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("failed to create config directory: %w", err)
	}

	// Marshal config
	data, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal config: %w", err)
	}

	// Write file
	if err := os.WriteFile(configPath, data, 0600); err != nil {
		return fmt.Errorf("failed to write config file: %w", err)
	}

	return nil
}

// getConfigPath returns the path to the config file
func getConfigPath() (string, error) {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("failed to get home directory: %w", err)
	}

	return filepath.Join(homeDir, DefaultConfigDir, DefaultConfigFile), nil
}

// GetConfigDir returns the config directory path
func GetConfigDir() (string, error) {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("failed to get home directory: %w", err)
	}

	return filepath.Join(homeDir, DefaultConfigDir), nil
}

// Validate validates the configuration
func (c *Config) Validate() error {
	if c.APIKey == "" && c.AuthToken == "" {
		return fmt.Errorf("API key or auth token is required. Set ANTHROPIC_API_KEY or ANTHROPIC_AUTH_TOKEN environment variable, or configure in ~/.claude-code/config.json")
	}

	if c.MaxTokens <= 0 {
		c.MaxTokens = 8192
	}

	return nil
}
