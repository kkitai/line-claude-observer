package config

import (
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func setEnv(t *testing.T, envs map[string]string) {
	t.Helper()
	for k, v := range envs {
		t.Setenv(k, v)
	}
}

func validEnvs() map[string]string {
	return map[string]string{
		"LINE_CHANNEL_SECRET":       "secret",
		"LINE_CHANNEL_ACCESS_TOKEN": "token",
		"ANTHROPIC_API_KEY":         "apikey",
		"DATABASE_URL":              "postgres://localhost/test",
	}
}

func TestLoad_Defaults(t *testing.T) {
	setEnv(t, validEnvs())

	cfg, err := Load()
	require.NoError(t, err)

	assert.Equal(t, "8080", cfg.Port)
	assert.Equal(t, 20, cfg.ContextMessageCount)
	assert.Equal(t, "/opinion", cfg.OpinionCommand)
}

func TestLoad_CustomValues(t *testing.T) {
	envs := validEnvs()
	envs["PORT"] = "9090"
	envs["CONTEXT_MESSAGE_COUNT"] = "50"
	envs["OPINION_COMMAND"] = "/review"
	setEnv(t, envs)

	cfg, err := Load()
	require.NoError(t, err)

	assert.Equal(t, "9090", cfg.Port)
	assert.Equal(t, 50, cfg.ContextMessageCount)
	assert.Equal(t, "/review", cfg.OpinionCommand)
	assert.Equal(t, "secret", cfg.LineChannelSecret)
	assert.Equal(t, "token", cfg.LineChannelAccessToken)
	assert.Equal(t, "apikey", cfg.AnthropicAPIKey)
	assert.Equal(t, "postgres://localhost/test", cfg.DatabaseURL)
}

func TestLoad_MissingRequired(t *testing.T) {
	requiredKeys := []string{
		"LINE_CHANNEL_SECRET",
		"LINE_CHANNEL_ACCESS_TOKEN",
		"ANTHROPIC_API_KEY",
		"DATABASE_URL",
	}

	for _, key := range requiredKeys {
		t.Run("missing_"+key, func(t *testing.T) {
			envs := validEnvs()
			delete(envs, key)
			setEnv(t, envs)
			// 対象キーを明示的に空に設定
			t.Setenv(key, "")

			_, err := Load()
			require.Error(t, err)
			assert.Contains(t, err.Error(), key)
		})
	}
}

func TestLoad_InvalidContextMessageCount(t *testing.T) {
	envs := validEnvs()
	envs["CONTEXT_MESSAGE_COUNT"] = "not-a-number"
	setEnv(t, envs)

	_, err := Load()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "CONTEXT_MESSAGE_COUNT")
}

func TestLoad_ZeroContextMessageCount(t *testing.T) {
	envs := validEnvs()
	envs["CONTEXT_MESSAGE_COUNT"] = "0"
	setEnv(t, envs)

	_, err := Load()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "CONTEXT_MESSAGE_COUNT")
}

func TestGetEnvOrDefault(t *testing.T) {
	t.Run("returns env value when set", func(t *testing.T) {
		t.Setenv("TEST_KEY", "value")
		assert.Equal(t, "value", getEnvOrDefault("TEST_KEY", "default"))
	})

	t.Run("returns default when not set", func(t *testing.T) {
		os.Unsetenv("TEST_KEY_UNSET")
		assert.Equal(t, "default", getEnvOrDefault("TEST_KEY_UNSET", "default"))
	})
}
