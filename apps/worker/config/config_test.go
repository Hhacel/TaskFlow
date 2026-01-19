package config

import (
	"os"
	"path/filepath"
	"reflect"
	"testing"
	"time"

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
					Port: "8082",
				},
				Worker: WorkerConfig{
					TaskTimeoutMinutes: 5,
					MaxConcurrentTasks: 10,
				},
				NATS: NATSConfig{
					URL:                 "nats://nats:4222",
					TaskScheduleSubject: "tasks.schedule",
					TaskResultSubject:   "tasks.results",
					ReconnectWait:       2,
					MaxReconnects:       60,
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
					Port: "8082",
				},
				Worker: WorkerConfig{
					TaskTimeoutMinutes: 5,
					MaxConcurrentTasks: 10,
				},
				NATS: NATSConfig{
					URL:                 "nats://nats:4222",
					TaskScheduleSubject: "tasks.schedule",
					TaskResultSubject:   "tasks.results",
					QueueGroupName:      "workers",
					ReconnectWait:       2,
					MaxReconnects:       60,
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
				Worker: WorkerConfig{
					TaskTimeoutMinutes: 5,  // from defaults
					MaxConcurrentTasks: 10, // from defaults
				},
				NATS: NATSConfig{
					URL:                 "nats://partial-nats:4222", // from partial config
					TaskScheduleSubject: "tasks.schedule",           // from defaults
					TaskResultSubject:   "tasks.results",            // from defaults
					QueueGroupName:      "workers",                  // from defaults
					ReconnectWait:       2,                          // from defaults
					MaxReconnects:       60,                         // from defaults
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
					Port: "8082",
				},
				Worker: WorkerConfig{
					TaskTimeoutMinutes: 5,
					MaxConcurrentTasks: 10,
				},
				NATS: NATSConfig{
					URL:                 "nats://nats:4222",
					TaskScheduleSubject: "tasks.schedule",
					TaskResultSubject:   "tasks.results",
					QueueGroupName:      "workers",
					ReconnectWait:       2,
					MaxReconnects:       60,
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
				Worker: WorkerConfig{
					TaskTimeoutMinutes: 5,
					MaxConcurrentTasks: 10,
				},
				NATS: NATSConfig{
					URL:                 "nats://nats:4222",
					TaskScheduleSubject: "tasks.schedule",
					TaskResultSubject:   "tasks.results",
					QueueGroupName:      "workers",
					ReconnectWait:       2,
					MaxReconnects:       60,
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
					Port: "8082",
				},
				Worker: WorkerConfig{
					TaskTimeoutMinutes: 5,
					MaxConcurrentTasks: 10,
				},
				NATS: NATSConfig{
					URL:                 "nats://nats:4222",
					TaskScheduleSubject: "tasks.schedule",
					TaskResultSubject:   "tasks.results",
					QueueGroupName:      "workers",
					ReconnectWait:       2,
					MaxReconnects:       60,
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
					Port: "8082",
				},
				Worker: WorkerConfig{
					TaskTimeoutMinutes: 5,
					MaxConcurrentTasks: 10,
				},
				NATS: NATSConfig{
					URL:                 "nats://env-nats:4223",
					TaskScheduleSubject: "tasks.schedule",
					TaskResultSubject:   "tasks.results",
					QueueGroupName:      "workers",
					ReconnectWait:       2,
					MaxReconnects:       60,
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
					Port: "8082",
				},
				Worker: WorkerConfig{
					TaskTimeoutMinutes: 5,
					MaxConcurrentTasks: 10,
				},
				NATS: NATSConfig{
					URL:                 "nats://nats:4222",
					TaskScheduleSubject: "tasks.schedule",
					TaskResultSubject:   "tasks.results",
					QueueGroupName:      "workers",
					ReconnectWait:       2,
					MaxReconnects:       60,
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
					Port: "8082",
				},
				Worker: WorkerConfig{
					TaskTimeoutMinutes: 5,
					MaxConcurrentTasks: 10,
				},
				NATS: NATSConfig{
					URL:                 "nats://nats:4222",
					TaskScheduleSubject: "tasks.schedule",
					TaskResultSubject:   "tasks.results",
					QueueGroupName:      "workers",
					ReconnectWait:       2,
					MaxReconnects:       60,
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
				Worker: WorkerConfig{
					TaskTimeoutMinutes: 5,
					MaxConcurrentTasks: 10,
				},
				NATS: NATSConfig{
					URL:                 "nats://multi-env-nats:4223",
					TaskScheduleSubject: "tasks.schedule",
					TaskResultSubject:   "tasks.results",
					QueueGroupName:      "workers",
					ReconnectWait:       2,
					MaxReconnects:       60,
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
				Worker: WorkerConfig{
					TaskTimeoutMinutes: 5,
					MaxConcurrentTasks: 10,
				},
				NATS: NATSConfig{
					URL:                 "nats://nats:4222",
					TaskScheduleSubject: "tasks.schedule",
					TaskResultSubject:   "tasks.results",
					QueueGroupName:      "workers",
					ReconnectWait:       2,
					MaxReconnects:       60,
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

func TestConfig_GetTaskTimeout(t *testing.T) {
	tests := []struct {
		name   string
		config *Config
		want   time.Duration
	}{
		{
			name: "returns correct duration for default value (5 minutes)",
			config: &Config{
				Worker: WorkerConfig{
					TaskTimeoutMinutes: 5,
				},
			},
			want: 5 * time.Minute,
		},
		{
			name: "returns correct duration for 10 minutes",
			config: &Config{
				Worker: WorkerConfig{
					TaskTimeoutMinutes: 10,
				},
			},
			want: 10 * time.Minute,
		},
		{
			name: "returns correct duration for 30 minutes",
			config: &Config{
				Worker: WorkerConfig{
					TaskTimeoutMinutes: 30,
				},
			},
			want: 30 * time.Minute,
		},
		{
			name: "returns correct duration for 1 minute",
			config: &Config{
				Worker: WorkerConfig{
					TaskTimeoutMinutes: 1,
				},
			},
			want: 1 * time.Minute,
		},
		{
			name: "returns zero duration for 0 minutes",
			config: &Config{
				Worker: WorkerConfig{
					TaskTimeoutMinutes: 0,
				},
			},
			want: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.config.GetTaskTimeout()
			assert.Equal(t, tt.want, got)
		})
	}
}
