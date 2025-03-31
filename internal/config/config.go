package config

import (
	"os"

	"github.com/joho/godotenv"
)

type Config struct {
	SolanaRPCEndpoint string
	TelegramToken     string
}

func Load() (*Config, error) {
	// Load .env file if it exists
	_ = godotenv.Load()

	// Set default values
	config := &Config{
		SolanaRPCEndpoint: getEnv("SOLANA_RPC_ENDPOINT", "https://api.mainnet-beta.solana.com"),
		TelegramToken:     os.Getenv("TELEGRAM_BOT_TOKEN"),
	}

	return config, nil
}

func getEnv(key, defaultValue string) string {
	value := os.Getenv(key)
	if value == "" {
		return defaultValue
	}
	return value
}
