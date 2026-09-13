package config

import (
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestDefaultConfig(t *testing.T) {
	cfg := DefaultConfig()
	assert.Equal(t, "8082", cfg.Server.Port)
	assert.Equal(t, "tasks.dispatch", cfg.NATS.TaskDispatchSubject)
	assert.Equal(t, "tasks.results", cfg.NATS.TaskResultSubject)
	assert.Equal(t, "workers", cfg.NATS.QueueGroupName)
	assert.Equal(t, 300, cfg.Worker.DefaultTimeoutSeconds)
}

func TestLoadConfig(t *testing.T) {
	t.Run("empty filepath returns default config", func(t *testing.T) {
		got, err := LoadConfig("")
		assert.NoError(t, err)
		assert.Equal(t, DefaultConfig(), got)
	})

	t.Run("nonexistent file returns default config", func(t *testing.T) {
		got, err := LoadConfig("nonexistent.yaml")
		assert.NoError(t, err)
		assert.Equal(t, DefaultConfig(), got)
	})

	t.Run("valid config file loads correctly", func(t *testing.T) {
		got, err := LoadConfig(filepath.Join("test_data", "valid_config.yaml"))
		assert.NoError(t, err)
		want := DefaultConfig()
		assert.Equal(t, want, got)
	})

	t.Run("partial config file merges with defaults", func(t *testing.T) {
		got, err := LoadConfig(filepath.Join("test_data", "partial_config.yaml"))
		assert.NoError(t, err)
		want := DefaultConfig()
		want.Server.Port = "7070"
		want.NATS.URL = "nats://partial-nats:4222"
		assert.Equal(t, want, got)
	})
}

func TestLoadConfigWithEnvOverrides(t *testing.T) {
	tests := []struct {
		name    string
		envVars map[string]string
		mutate  func(*Config)
	}{
		{
			name:    "no env vars uses config file values",
			envVars: map[string]string{},
			mutate:  func(c *Config) {},
		},
		{
			name:    "PORT env var overrides config",
			envVars: map[string]string{"PORT": "9999"},
			mutate:  func(c *Config) { c.Server.Port = "9999" },
		},
		{
			name:    "NATS_URL env var overrides config",
			envVars: map[string]string{"NATS_URL": "nats://override:4222"},
			mutate:  func(c *Config) { c.NATS.URL = "nats://override:4222" },
		},
		{
			name:    "TASK_TIMEOUT env var overrides config",
			envVars: map[string]string{"TASK_TIMEOUT": "10m"},
			mutate:  func(c *Config) { c.Worker.DefaultTimeoutSeconds = 600 },
		},
		{
			name:    "LOG_LEVEL env var overrides config",
			envVars: map[string]string{"LOG_LEVEL": "debug"},
			mutate:  func(c *Config) { c.Logging.Level = "debug" },
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			for k, v := range tt.envVars {
				t.Setenv(k, v)
			}

			got, err := LoadConfigWithEnvOverrides(filepath.Join("test_data", "valid_config.yaml"))
			assert.NoError(t, err)

			want := DefaultConfig()
			tt.mutate(want)
			assert.Equal(t, want, got)
		})
	}
}

func TestConfig_DefaultTimeout(t *testing.T) {
	cfg := DefaultConfig()
	assert.Equal(t, "5m0s", cfg.DefaultTimeout().String())
}

func TestConfig_Validate(t *testing.T) {
	t.Run("valid config passes", func(t *testing.T) {
		assert.NoError(t, DefaultConfig().Validate())
	})

	t.Run("missing port fails", func(t *testing.T) {
		cfg := DefaultConfig()
		cfg.Server.Port = ""
		assert.Error(t, cfg.Validate())
	})

	t.Run("missing nats url fails", func(t *testing.T) {
		cfg := DefaultConfig()
		cfg.NATS.URL = ""
		assert.Error(t, cfg.Validate())
	})

	t.Run("zero timeout fails", func(t *testing.T) {
		cfg := DefaultConfig()
		cfg.Worker.DefaultTimeoutSeconds = 0
		assert.Error(t, cfg.Validate())
	})
}
