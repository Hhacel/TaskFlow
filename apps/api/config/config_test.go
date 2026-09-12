package config

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/hhace/taskflow/pkg/persistence"
	"github.com/stretchr/testify/assert"
)

func TestDefaultConfig(t *testing.T) {
	cfg := DefaultConfig()

	assert.Equal(t, "8081", cfg.Server.Port)
	assert.Equal(t, "postgres", cfg.Database.Host)
	assert.Equal(t, "taskflow", cfg.Database.User)
	assert.Equal(t, "nats://nats:4222", cfg.NATS.URL)
	assert.Equal(t, "workflow.commands.start", cfg.NATS.WorkflowStartSubject)
	assert.Equal(t, "workflow.commands.cancel", cfg.NATS.WorkflowCancelSubject)
	assert.Equal(t, "info", cfg.Logging.Level)
}

func TestLoadConfig(t *testing.T) {
	t.Run("empty filepath returns default config", func(t *testing.T) {
		got, err := LoadConfig("")
		assert.NoError(t, err)
		assert.Equal(t, DefaultConfig(), got)
	})

	t.Run("valid config file loads correctly", func(t *testing.T) {
		got, err := LoadConfig(filepath.Join("test_data", "valid_config.yaml"))
		assert.NoError(t, err)

		want := DefaultConfig()
		want.Database.MaxOpenConns = 25
		want.Database.MaxIdleConns = 5
		want.Database.ConnMaxLifetimeMinutes = 60
		assert.Equal(t, want, got)
	})

	t.Run("nonexistent file returns default config", func(t *testing.T) {
		got, err := LoadConfig("nonexistent.yaml")
		assert.NoError(t, err)
		assert.Equal(t, DefaultConfig(), got)
	})

	t.Run("partial config file merges with defaults", func(t *testing.T) {
		got, err := LoadConfig(filepath.Join("test_data", "partial_config.yaml"))
		assert.NoError(t, err)

		want := DefaultConfig()
		want.Server.Port = "7070"
		assert.Equal(t, want, got)
	})
}

func TestLoadConfigWithEnvOverrides(t *testing.T) {
	tests := []struct {
		name     string
		filepath string
		envVars  map[string]string
		mutate   func(*Config)
	}{
		{
			name:     "no env vars uses config file values",
			filepath: filepath.Join("test_data", "valid_config.yaml"),
			envVars:  map[string]string{},
			mutate:   func(c *Config) {},
		},
		{
			name:     "PORT env var overrides config",
			filepath: filepath.Join("test_data", "valid_config.yaml"),
			envVars:  map[string]string{"PORT": "9999"},
			mutate:   func(c *Config) { c.Server.Port = "9999" },
		},
		{
			name:     "all database env vars override config",
			filepath: filepath.Join("test_data", "valid_config.yaml"),
			envVars: map[string]string{
				"DB_HOST":     "env-postgres",
				"DB_PORT":     "5433",
				"DB_USER":     "envuser",
				"DB_PASSWORD": "envpass",
				"DB_NAME":     "envdb",
			},
			mutate: func(c *Config) {
				c.Database.Host = "env-postgres"
				c.Database.Port = "5433"
				c.Database.User = "envuser"
				c.Database.Password = "envpass"
				c.Database.Database = "envdb"
			},
		},
		{
			name:     "NATS_URL env var overrides config",
			filepath: filepath.Join("test_data", "valid_config.yaml"),
			envVars:  map[string]string{"NATS_URL": "nats://env-nats:4223"},
			mutate:   func(c *Config) { c.NATS.URL = "nats://env-nats:4223" },
		},
		{
			name:     "LOG_LEVEL env var overrides config",
			filepath: filepath.Join("test_data", "valid_config.yaml"),
			envVars:  map[string]string{"LOG_LEVEL": "debug"},
			mutate:   func(c *Config) { c.Logging.Level = "debug" },
		},
		{
			name:     "multiple env vars override config",
			filepath: filepath.Join("test_data", "partial_config.yaml"),
			envVars: map[string]string{
				"PORT":     "7777",
				"DB_HOST":  "multi-env-postgres",
				"NATS_URL": "nats://multi-env-nats:4223",
			},
			mutate: func(c *Config) {
				c.Server.Port = "7777"
				c.Database.Host = "multi-env-postgres"
				c.NATS.URL = "nats://multi-env-nats:4223"
			},
		},
		{
			name:     "env vars work with default config when no file",
			filepath: "",
			envVars: map[string]string{
				"PORT":    "6666",
				"DB_HOST": "override-postgres",
			},
			mutate: func(c *Config) {
				c.Server.Port = "6666"
				c.Database.Host = "override-postgres"
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			for key, value := range tt.envVars {
				os.Setenv(key, value)
				defer os.Unsetenv(key)
			}

			got, err := LoadConfigWithEnvOverrides(tt.filepath)
			assert.NoError(t, err)

			want, err := LoadConfig(tt.filepath)
			assert.NoError(t, err)
			tt.mutate(want)

			assert.Equal(t, want, got)
		})
	}
}

func TestConfig_GetDSN(t *testing.T) {
	tests := []struct {
		name   string
		config *Config
		want   string
	}{
		{
			name: "returns correct DSN with default values",
			config: &Config{
				Database: persistence.Config{
					Host:     "postgres",
					Port:     "5432",
					User:     "taskflow",
					Password: "taskflow",
					Database: "taskflow",
					SSLMode:  "disable",
				},
			},
			want: "host=postgres port=5432 user=taskflow password=taskflow dbname=taskflow sslmode=disable",
		},
		{
			name: "returns correct DSN with custom values",
			config: &Config{
				Database: persistence.Config{
					Host:     "custom-host",
					Port:     "5433",
					User:     "customuser",
					Password: "custompass",
					Database: "customdb",
					SSLMode:  "require",
				},
			},
			want: "host=custom-host port=5433 user=customuser password=custompass dbname=customdb sslmode=require",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.config.GetDSN()
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestConfig_GetConnMaxLifetime(t *testing.T) {
	tests := []struct {
		name   string
		config *Config
		want   time.Duration
	}{
		{
			name:   "returns correct duration for default value (60 minutes)",
			config: &Config{Database: persistence.Config{ConnMaxLifetimeMinutes: 60}},
			want:   60 * time.Minute,
		},
		{
			name:   "returns correct duration for 5 minutes",
			config: &Config{Database: persistence.Config{ConnMaxLifetimeMinutes: 5}},
			want:   5 * time.Minute,
		},
		{
			name:   "returns zero duration for 0 minutes",
			config: &Config{Database: persistence.Config{ConnMaxLifetimeMinutes: 0}},
			want:   0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.config.GetConnMaxLifetime()
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestConfig_Validate(t *testing.T) {
	valid := DefaultConfig()
	assert.NoError(t, valid.Validate())

	missingPort := DefaultConfig()
	missingPort.Server.Port = ""
	assert.Error(t, missingPort.Validate())

	missingDBHost := DefaultConfig()
	missingDBHost.Database.Host = ""
	assert.Error(t, missingDBHost.Validate())
}
