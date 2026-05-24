package config

import (
	"fmt"
	"os"
	"strconv"

	"github.com/joho/godotenv"
)

type Config struct {
	LineChannelSecret      string
	LineChannelAccessToken string
	AnthropicAPIKey        string
	DatabaseURL            string
	Port                   string
	ContextMessageCount    int
	OpinionCommand         string
}

func Load() (*Config, error) {
	// .env が存在する場合のみ読み込む（本番では環境変数から直接取得）
	_ = godotenv.Load()

	cfg := &Config{
		LineChannelSecret:      os.Getenv("LINE_CHANNEL_SECRET"),
		LineChannelAccessToken: os.Getenv("LINE_CHANNEL_ACCESS_TOKEN"),
		AnthropicAPIKey:        os.Getenv("ANTHROPIC_API_KEY"),
		DatabaseURL:            os.Getenv("DATABASE_URL"),
		Port:                   getEnvOrDefault("PORT", "8080"),
		OpinionCommand:         getEnvOrDefault("OPINION_COMMAND", "/opinion"),
	}

	var err error
	cfg.ContextMessageCount, err = strconv.Atoi(getEnvOrDefault("CONTEXT_MESSAGE_COUNT", "20"))
	if err != nil {
		return nil, fmt.Errorf("CONTEXT_MESSAGE_COUNT must be an integer: %w", err)
	}

	if err := cfg.validate(); err != nil {
		return nil, err
	}

	return cfg, nil
}

func (c *Config) validate() error {
	required := map[string]string{
		"LINE_CHANNEL_SECRET":       c.LineChannelSecret,
		"LINE_CHANNEL_ACCESS_TOKEN": c.LineChannelAccessToken,
		"ANTHROPIC_API_KEY":         c.AnthropicAPIKey,
		"DATABASE_URL":              c.DatabaseURL,
	}
	for name, val := range required {
		if val == "" {
			return fmt.Errorf("required environment variable %s is not set", name)
		}
	}
	if c.ContextMessageCount <= 0 {
		return fmt.Errorf("CONTEXT_MESSAGE_COUNT must be a positive integer")
	}
	return nil
}

func getEnvOrDefault(key, defaultVal string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return defaultVal
}
