package config

import (
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestLoad(t *testing.T) {
	tests := []struct {
		name           string
		envVars        map[string]string
		expectedConfig *Config
	}{
		{
			name:    "Default values",
			envVars: map[string]string{},
			expectedConfig: &Config{
				SolanaRPCEndpoint: "https://api.mainnet-beta.solana.com",
				TelegramToken:     "",
			},
		},
		{
			name: "Custom values",
			envVars: map[string]string{
				"SOLANA_RPC_ENDPOINT": "https://custom-endpoint.com",
				"TELEGRAM_BOT_TOKEN":  "test-token",
			},
			expectedConfig: &Config{
				SolanaRPCEndpoint: "https://custom-endpoint.com",
				TelegramToken:     "test-token",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Clear environment variables
			os.Clearenv()

			// Set test environment variables
			for key, value := range tt.envVars {
				os.Setenv(key, value)
			}

			// Load configuration
			config, err := Load()

			// Assert results
			assert.NoError(t, err)
			assert.Equal(t, tt.expectedConfig.SolanaRPCEndpoint, config.SolanaRPCEndpoint)
			assert.Equal(t, tt.expectedConfig.TelegramToken, config.TelegramToken)
		})
	}
}

func TestGetEnv(t *testing.T) {
	tests := []struct {
		name       string
		key        string
		defaultVal string
		envValue   string
		expected   string
	}{
		{
			name:       "Returns default when env var is not set",
			key:        "TEST_KEY",
			defaultVal: "default",
			envValue:   "",
			expected:   "default",
		},
		{
			name:       "Returns env var when set",
			key:        "TEST_KEY",
			defaultVal: "default",
			envValue:   "custom",
			expected:   "custom",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Clear environment variables
			os.Clearenv()

			// Set test environment variable if needed
			if tt.envValue != "" {
				os.Setenv(tt.key, tt.envValue)
			}

			// Test getEnv function
			result := getEnv(tt.key, tt.defaultVal)
			assert.Equal(t, tt.expected, result)
		})
	}
}
