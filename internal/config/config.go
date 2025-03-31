package config

import (
	"os"
	"strconv"

	"github.com/joho/godotenv"
)

// Config holds all configuration values
type Config struct {
	SolanaRPCEndpoint string
	TelegramToken     string
	USDCThreshold     float64 // Threshold for USDC transfers to trigger alerts (e.g., 1000 USDC)
}

// Load reads configuration from environment variables
func Load() (*Config, error) {
	// Load .env file if it exists
	_ = godotenv.Load()

	// Parse USDC threshold from environment variable
	threshold := 1000.0
	if thresholdStr := os.Getenv("USDC_THRESHOLD"); thresholdStr != "" {
		if val, err := strconv.ParseFloat(thresholdStr, 64); err == nil {
			threshold = val
		}
	}

	// Set default values
	config := &Config{
		SolanaRPCEndpoint: getEnv("SOLANA_RPC_ENDPOINT", "https://api.mainnet-beta.solana.com"),
		TelegramToken:     os.Getenv("TELEGRAM_BOT_TOKEN"),
		USDCThreshold:     threshold,
	}

	return config, nil
}

// getEnv gets an environment variable or returns a default value
func getEnv(key, defaultValue string) string {
	value := os.Getenv(key)
	if value == "" {
		return defaultValue
	}
	return value
}
