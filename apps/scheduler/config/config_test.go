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
					Port: "8084",
				},
				Database: persistence.Config{
					Host:     "postgres",
					Port:     "5432",
					User:     "taskflow",
					Password: "taskflow",
					Database: "taskflow",
					SSLMode:  "disable",
				},
				NATS: NATSConfig{
					URL:                 "nats://nats:4222",
					TaskScheduleSubject: "tasks.schedule",
					TaskResultSubject:   "tasks.results",
					ReconnectWait:       2,
					MaxReconnects:       60,
				},
				GRPC: GRPCConfig{
					NotifierAddress: "notifier:8083",
					Timeout:         10,
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
					Port: "8084",
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
					ConnMaxLifetimeMinutes: 5,
				},
				NATS: NATSConfig{
					URL:                 "nats://nats:4222",
					TaskScheduleSubject: "tasks.schedule",
					TaskResultSubject:   "tasks.results",
					ReconnectWait:       2,
					MaxReconnects:       60,
				},
				GRPC: GRPCConfig{
					NotifierAddress: "notifier:8083",
					Timeout:         10,
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
					Host:     "postgres", // from defaults
					Port:     "5432",     // from defaults
					User:     "taskflow", // from defaults
					Password: "taskflow", // from defaults
					Database: "taskflow", // from defaults
					SSLMode:  "disable",  // from defaults
				},
				NATS: NATSConfig{
					URL:                 "nats://partial-nats:4222", // from partial config
					TaskScheduleSubject: "tasks.schedule",           // from defaults
					TaskResultSubject:   "tasks.results",            // from defaults
					ReconnectWait:       2,                          // from defaults
					MaxReconnects:       60,                         // from defaults
				},
				GRPC: GRPCConfig{
					NotifierAddress: "notifier:8083", // from defaults
					Timeout:         10,              // from defaults
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
					Port: "8084",
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
					ConnMaxLifetimeMinutes: 5,
				},
				NATS: NATSConfig{
					URL:                 "nats://nats:4222",
					TaskScheduleSubject: "tasks.schedule",
					TaskResultSubject:   "tasks.results",
					ReconnectWait:       2,
					MaxReconnects:       60,
				},
				GRPC: GRPCConfig{
					NotifierAddress: "notifier:8083",
					Timeout:         10,
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
					ConnMaxLifetimeMinutes: 5,
				},
				NATS: NATSConfig{
					URL:                 "nats://nats:4222",
					TaskScheduleSubject: "tasks.schedule",
					TaskResultSubject:   "tasks.results",
					ReconnectWait:       2,
					MaxReconnects:       60,
				},
				GRPC: GRPCConfig{
					NotifierAddress: "notifier:8083",
					Timeout:         10,
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
					Port: "8084",
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
					ConnMaxLifetimeMinutes: 5,
				},
				NATS: NATSConfig{
					URL:                 "nats://nats:4222",
					TaskScheduleSubject: "tasks.schedule",
					TaskResultSubject:   "tasks.results",
					ReconnectWait:       2,
					MaxReconnects:       60,
				},
				GRPC: GRPCConfig{
					NotifierAddress: "notifier:8083",
					Timeout:         10,
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
					Port: "8084",
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
					ConnMaxLifetimeMinutes: 5,
				},
				NATS: NATSConfig{
					URL:                 "nats://env-nats:4223",
					TaskScheduleSubject: "tasks.schedule",
					TaskResultSubject:   "tasks.results",
					ReconnectWait:       2,
					MaxReconnects:       60,
				},
				GRPC: GRPCConfig{
					NotifierAddress: "notifier:8083",
					Timeout:         10,
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
					Port: "8084",
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
					ConnMaxLifetimeMinutes: 5,
				},
				NATS: NATSConfig{
					URL:                 "nats://nats:4222",
					TaskScheduleSubject: "tasks.schedule",
					TaskResultSubject:   "tasks.results",
					ReconnectWait:       2,
					MaxReconnects:       60,
				},
				GRPC: GRPCConfig{
					NotifierAddress: "env-notifier:8084",
					Timeout:         10,
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
					Port: "8084",
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
					ConnMaxLifetimeMinutes: 5,
				},
				NATS: NATSConfig{
					URL:                 "nats://nats:4222",
					TaskScheduleSubject: "tasks.schedule",
					TaskResultSubject:   "tasks.results",
					ReconnectWait:       2,
					MaxReconnects:       60,
				},
				GRPC: GRPCConfig{
					NotifierAddress: "notifier:8083",
					Timeout:         10,
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
					Host:     "multi-env-postgres",
					Port:     "5432",
					User:     "taskflow",
					Password: "taskflow",
					Database: "taskflow",
					SSLMode:  "disable",
				},
				NATS: NATSConfig{
					URL:                 "nats://multi-env-nats:4223",
					TaskScheduleSubject: "tasks.schedule",
					TaskResultSubject:   "tasks.results",
					ReconnectWait:       2,
					MaxReconnects:       60,
				},
				GRPC: GRPCConfig{
					NotifierAddress: "multi-env-notifier:8085",
					Timeout:         10,
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
					Host:     "override-postgres",
					Port:     "5432",
					User:     "taskflow",
					Password: "taskflow",
					Database: "taskflow",
					SSLMode:  "disable",
				},
				NATS: NATSConfig{
					URL:                 "nats://nats:4222",
					TaskScheduleSubject: "tasks.schedule",
					TaskResultSubject:   "tasks.results",
					ReconnectWait:       2,
					MaxReconnects:       60,
				},
				GRPC: GRPCConfig{
					NotifierAddress: "notifier:8083",
					Timeout:         10,
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

func TestGetGRPCTimeout(t *testing.T) {
	tests := []struct {
		name           string
		timeoutSeconds int
		want           time.Duration
	}{
		{
			name:           "default timeout of 10 seconds",
			timeoutSeconds: 10,
			want:           10 * time.Second,
		},
		{
			name:           "timeout of 5 seconds",
			timeoutSeconds: 5,
			want:           5 * time.Second,
		},
		{
			name:           "timeout of 30 seconds",
			timeoutSeconds: 30,
			want:           30 * time.Second,
		},
		{
			name:           "timeout of 1 second",
			timeoutSeconds: 1,
			want:           1 * time.Second,
		},
		{
			name:           "timeout of 60 seconds",
			timeoutSeconds: 60,
			want:           60 * time.Second,
		},
		{
			name:           "timeout of 0 seconds",
			timeoutSeconds: 0,
			want:           0 * time.Second,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			config := &Config{
				GRPC: GRPCConfig{
					Timeout: tt.timeoutSeconds,
				},
			}

			got := config.GetGRPCTimeout()
			assert.Equal(t, tt.want, got)
		})
	}
}
