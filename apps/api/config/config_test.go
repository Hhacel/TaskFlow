package config

import (
	"os"
	"path/filepath"
	"reflect"
	"testing"
	"time"

	"github.com/hhace/taskflow/pkg/persistence"
	"github.com/stretchr/testify/assert"
)

func TestDefaultConfig(t *testing.T) {
	tests := []struct {
		name string
		want *Config
	}{
		{
			name: "returns config with correct default values",
			want: &Config{
				Server: ServerConfig{
					Port: "8081",
				},
				Database: persistence.Config{
					Host:     "postgres",
					Port:     "5432",
					User:     "taskflow",
					Password: "taskflow",
					Database: "taskflow",
					SSLMode:  "disable",
				},
				Logging: LoggingConfig{
					Level:  "info",
					Format: "json",
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := DefaultConfig()
			reflect.DeepEqual(tt.want, got)
		})
	}
}

func TestLoadConfig(t *testing.T) {
	tests := []struct {
		name       string
		filepath   string
		wantErr    bool
		wantConfig *Config
	}{
		{
			name:       "empty filepath returns default config",
			filepath:   "",
			wantErr:    false,
			wantConfig: DefaultConfig(),
		},
		{
			name:     "valid config file loads correctly",
			filepath: filepath.Join("test_data", "valid_config.yaml"),
			wantErr:  false,
			wantConfig: &Config{
				Server: ServerConfig{
					Port: "8081",
				},
				Database: persistence.Config{
					Host:                   "postgres",
					Port:                   "5432",
					User:                   "taskflow",
					Password:               "taskflow",
					Database:               "taskflow",
					SSLMode:                "disable",
					MaxOpenConns:           25,
					MaxIdleConns:           5,
					ConnMaxLifetimeMinutes: 60,
				},
				Logging: LoggingConfig{
					Level:  "info",
					Format: "json",
				},
			},
		},
		{
			name:       "nonexistent file returns default config",
			filepath:   "nonexistent.yaml",
			wantErr:    false,
			wantConfig: DefaultConfig(),
		},
		{
			name:       "invalid yaml returns default config",
			filepath:   filepath.Join("test_data", "invalid_config.yaml"),
			wantErr:    false,
			wantConfig: DefaultConfig(),
		},
		{
			name:     "partial config file merges with defaults",
			filepath: filepath.Join("test_data", "partial_config.yaml"),
			wantErr:  false,
			wantConfig: &Config{
				Server: ServerConfig{
					Port: "7070", // from partial config
				},
				Database: persistence.Config{
					Host:                   "postgres", // from defaults
					Port:                   "5432",     // from defaults
					User:                   "taskflow", // from defaults
					Password:               "taskflow", // from defaults
					Database:               "taskflow", // from defaults
					SSLMode:                "disable",  // from defaults
					MaxOpenConns:           25,         // from defaults
					MaxIdleConns:           5,          // from defaults
					ConnMaxLifetimeMinutes: 60,         // from defaults
				},
				Logging: LoggingConfig{
					Level:  "info", // from defaults
					Format: "json", // from defaults
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := LoadConfig(tt.filepath)
			if tt.wantErr {
				assert.NotNil(t, err)
			} else {
				assert.Nil(t, err)
			}

			assert.Equal(t, tt.wantConfig, got)
		})
	}
}

func TestLoadConfigWithEnvOverrides(t *testing.T) {
	tests := []struct {
		name       string
		filepath   string
		envVars    map[string]string
		wantErr    bool
		wantConfig *Config
	}{
		{
			name:     "no env vars uses config file values",
			filepath: filepath.Join("test_data", "valid_config.yaml"),
			envVars:  map[string]string{},
			wantErr:  false,
			wantConfig: &Config{
				Server: ServerConfig{
					Port: "8081",
				},
				Database: persistence.Config{
					Host:                   "postgres",
					Port:                   "5432",
					User:                   "taskflow",
					Password:               "taskflow",
					Database:               "taskflow",
					SSLMode:                "disable",
					MaxOpenConns:           25,
					MaxIdleConns:           5,
					ConnMaxLifetimeMinutes: 60,
				},
				Logging: LoggingConfig{
					Level:  "info",
					Format: "json",
				},
			},
		},
		{
			name:     "PORT env var overrides config",
			filepath: filepath.Join("test_data", "valid_config.yaml"),
			envVars: map[string]string{
				"PORT": "9999",
			},
			wantErr: false,
			wantConfig: &Config{
				Server: ServerConfig{
					Port: "9999",
				},
				Database: persistence.Config{
					Host:                   "postgres",
					Port:                   "5432",
					User:                   "taskflow",
					Password:               "taskflow",
					Database:               "taskflow",
					SSLMode:                "disable",
					MaxOpenConns:           25,
					MaxIdleConns:           5,
					ConnMaxLifetimeMinutes: 60,
				},
				Logging: LoggingConfig{
					Level:  "info",
					Format: "json",
				},
			},
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
			wantErr: false,
			wantConfig: &Config{
				Server: ServerConfig{
					Port: "8081",
				},
				Database: persistence.Config{
					Host:                   "env-postgres",
					Port:                   "5433",
					User:                   "envuser",
					Password:               "envpass",
					Database:               "envdb",
					SSLMode:                "disable",
					MaxOpenConns:           25,
					MaxIdleConns:           5,
					ConnMaxLifetimeMinutes: 60,
				},
				Logging: LoggingConfig{
					Level:  "info",
					Format: "json",
				},
			},
		},
		{
			name:     "NATS_URL env var overrides config",
			filepath: filepath.Join("test_data", "valid_config.yaml"),
			envVars: map[string]string{
				"NATS_URL": "nats://env-nats:4223",
			},
			wantErr: false,
			wantConfig: &Config{
				Server: ServerConfig{
					Port: "8081",
				},
				Database: persistence.Config{
					Host:                   "postgres",
					Port:                   "5432",
					User:                   "taskflow",
					Password:               "taskflow",
					Database:               "taskflow",
					SSLMode:                "disable",
					MaxOpenConns:           25,
					MaxIdleConns:           5,
					ConnMaxLifetimeMinutes: 60,
				},
				Logging: LoggingConfig{
					Level:  "info",
					Format: "json",
				},
			},
		},
		{
			name:     "NOTIFIER_ADDRESS env var overrides config",
			filepath: filepath.Join("test_data", "valid_config.yaml"),
			envVars: map[string]string{
				"NOTIFIER_ADDRESS": "env-notifier:8084",
			},
			wantErr: false,
			wantConfig: &Config{
				Server: ServerConfig{
					Port: "8081",
				},
				Database: persistence.Config{
					Host:                   "postgres",
					Port:                   "5432",
					User:                   "taskflow",
					Password:               "taskflow",
					Database:               "taskflow",
					SSLMode:                "disable",
					MaxOpenConns:           25,
					MaxIdleConns:           5,
					ConnMaxLifetimeMinutes: 60,
				},
				Logging: LoggingConfig{
					Level:  "info",
					Format: "json",
				},
			},
		},
		{
			name:     "LOG_LEVEL env var overrides config",
			filepath: filepath.Join("test_data", "valid_config.yaml"),
			envVars: map[string]string{
				"LOG_LEVEL": "debug",
			},
			wantErr: false,
			wantConfig: &Config{
				Server: ServerConfig{
					Port: "8081",
				},
				Database: persistence.Config{
					Host:                   "postgres",
					Port:                   "5432",
					User:                   "taskflow",
					Password:               "taskflow",
					Database:               "taskflow",
					SSLMode:                "disable",
					MaxOpenConns:           25,
					MaxIdleConns:           5,
					ConnMaxLifetimeMinutes: 60,
				},
				Logging: LoggingConfig{
					Level:  "debug",
					Format: "json",
				},
			},
		},
		{
			name:     "multiple env vars override config",
			filepath: filepath.Join("test_data", "partial_config.yaml"),
			envVars: map[string]string{
				"PORT":             "7777",
				"DB_HOST":          "multi-env-postgres",
				"NATS_URL":         "nats://multi-env-nats:4223",
				"NOTIFIER_ADDRESS": "multi-env-notifier:8085",
			},
			wantErr: false,
			wantConfig: &Config{
				Server: ServerConfig{
					Port: "7777",
				},
				Database: persistence.Config{
					Host:                   "multi-env-postgres",
					Port:                   "5432",
					User:                   "taskflow",
					Password:               "taskflow",
					Database:               "taskflow",
					SSLMode:                "disable",
					MaxOpenConns:           25,
					MaxIdleConns:           5,
					ConnMaxLifetimeMinutes: 60,
				},
				Logging: LoggingConfig{
					Level:  "info",
					Format: "json",
				},
			},
		},
		{
			name:     "env vars work with default config when no file",
			filepath: "",
			envVars: map[string]string{
				"PORT":    "6666",
				"DB_HOST": "override-postgres",
			},
			wantErr: false,
			wantConfig: &Config{
				Server: ServerConfig{
					Port: "6666",
				},
				Database: persistence.Config{
					Host:                   "override-postgres",
					Port:                   "5432",
					User:                   "taskflow",
					Password:               "taskflow",
					Database:               "taskflow",
					SSLMode:                "disable",
					MaxOpenConns:           25,
					MaxIdleConns:           5,
					ConnMaxLifetimeMinutes: 60,
				},
				Logging: LoggingConfig{
					Level:  "info",
					Format: "json",
				},
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
			if tt.wantErr {
				assert.NotNil(t, err)
			} else {
				assert.Nil(t, err)
			}

			assert.Equal(t, tt.wantConfig, got)
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
		{
			name: "returns correct DSN with empty password",
			config: &Config{
				Database: persistence.Config{
					Host:     "localhost",
					Port:     "5432",
					User:     "testuser",
					Password: "",
					Database: "testdb",
					SSLMode:  "disable",
				},
			},
			want: "host=localhost port=5432 user=testuser password= dbname=testdb sslmode=disable",
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
			name: "returns correct duration for default value (60 minutes)",
			config: &Config{
				Database: persistence.Config{
					ConnMaxLifetimeMinutes: 60,
				},
			},
			want: 60 * time.Minute,
		},
		{
			name: "returns correct duration for 5 minutes",
			config: &Config{
				Database: persistence.Config{
					ConnMaxLifetimeMinutes: 5,
				},
			},
			want: 5 * time.Minute,
		},
		{
			name: "returns correct duration for 120 minutes",
			config: &Config{
				Database: persistence.Config{
					ConnMaxLifetimeMinutes: 120,
				},
			},
			want: 120 * time.Minute,
		},
		{
			name: "returns zero duration for 0 minutes",
			config: &Config{
				Database: persistence.Config{
					ConnMaxLifetimeMinutes: 0,
				},
			},
			want: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.config.GetConnMaxLifetime()
			assert.Equal(t, tt.want, got)
		})
	}
}
