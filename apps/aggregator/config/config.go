package config

import (
	"log/slog"
	"os"
	"time"

	"github.com/hhace/taskflow/pkg/database"
	"gopkg.in/yaml.v3"
)

// Config represents the aggregator configuration
type Config struct {
	Server   ServerConfig    `yaml:"server"`
	Database database.Config `yaml:"database"`
	NATS     NATSConfig      `yaml:"nats"`
	GRPC     GRPCConfig      `yaml:"grpc"`
	Logging  LoggingConfig   `yaml:"logging"`
}

// ServerConfig contains HTTP server settings
type ServerConfig struct {
	Port string `yaml:"port"`
}

// NATSConfig contains NATS connection settings
type NATSConfig struct {
	URL               string `yaml:"url"`
	TaskResultSubject string `yaml:"task_result_subject"`
	ReconnectWait     int    `yaml:"reconnect_wait_seconds"`
	MaxReconnects     int    `yaml:"max_reconnects"`
}

// GRPCConfig contains gRPC settings for notifier communication
type GRPCConfig struct {
	NotifierAddress string `yaml:"notifier_address"`
	Timeout         int    `yaml:"timeout_seconds"`
}

// LoggingConfig contains logging settings
type LoggingConfig struct {
	Level  string `yaml:"level"`
	Format string `yaml:"format"`
}

// DefaultConfig returns a config with default values
func DefaultConfig() *Config {
	return &Config{
		Server: ServerConfig{
			Port: "8084",
		},
		Database: database.Config{
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
			URL:               "nats://nats:4222",
			TaskResultSubject: "tasks.results",
			ReconnectWait:     2,
			MaxReconnects:     60,
		},
		GRPC: GRPCConfig{
			NotifierAddress: "notifier:8083",
			Timeout:         10,
		},
		Logging: LoggingConfig{
			Level:  "info",
			Format: "json",
		},
	}
}

// LoadConfig loads configuration from a YAML file
func LoadConfig(filepath string) (*Config, error) {
	config := DefaultConfig()

	// If no file specified, return default config
	if filepath == "" {
		slog.Info("No config file path provided, using defaults")
		return config, nil
	}

	// Read the config file
	data, err := os.ReadFile(filepath)
	if err != nil {
		slog.Warn("Could not read config file, using defaults", "path", filepath, "error", err)
		return config, nil // Return defaults instead of error
	}

	// Parse YAML
	if err := yaml.Unmarshal(data, config); err != nil {
		slog.Warn("Could not parse config file, using defaults", "path", filepath, "error", err)
		return config, nil // Return defaults instead of error
	}

	slog.Info("Config loaded successfully", "path", filepath, "dbHost", config.Database.Host)
	return config, nil
}

// LoadConfigWithEnvOverrides loads config from file and applies environment variable overrides
func LoadConfigWithEnvOverrides(filepath string) (*Config, error) {
	slog.Debug("Loading config with env overrides", "path", filepath)
	config, err := LoadConfig(filepath)
	if err != nil {
		return nil, err
	}

	slog.Debug("Config before env overrides", "dbHost", config.Database.Host, "natsURL", config.NATS.URL)

	// Override with environment variables if they exist
	if port := os.Getenv("PORT"); port != "" {
		slog.Debug("Overriding port with env var", "value", port)
		config.Server.Port = port
	}

	if dbHost := os.Getenv("DB_HOST"); dbHost != "" {
		slog.Debug("Overriding DB host with env var", "value", dbHost)
		config.Database.Host = dbHost
	}

	if dbPort := os.Getenv("DB_PORT"); dbPort != "" {
		slog.Debug("Overriding DB port with env var", "value", dbPort)
		config.Database.Port = dbPort
	}

	if dbUser := os.Getenv("DB_USER"); dbUser != "" {
		config.Database.User = dbUser
	}

	if dbPassword := os.Getenv("DB_PASSWORD"); dbPassword != "" {
		config.Database.Password = dbPassword
	}

	if dbName := os.Getenv("DB_NAME"); dbName != "" {
		config.Database.Database = dbName
	}

	if natsURL := os.Getenv("NATS_URL"); natsURL != "" {
		slog.Debug("Overriding NATS URL with env var", "value", natsURL)
		config.NATS.URL = natsURL
	}

	if notifierAddr := os.Getenv("NOTIFIER_ADDRESS"); notifierAddr != "" {
		slog.Debug("Overriding notifier address with env var", "value", notifierAddr)
		config.GRPC.NotifierAddress = notifierAddr
	}

	if logLevel := os.Getenv("LOG_LEVEL"); logLevel != "" {
		config.Logging.Level = logLevel
	}

	slog.Info("Final config loaded", "dbHost", config.Database.Host, "natsURL", config.NATS.URL, "notifierAddr", config.GRPC.NotifierAddress)

	return config, nil
}

// GetGRPCTimeout returns the gRPC timeout duration
func (c *Config) GetGRPCTimeout() time.Duration {
	return time.Duration(c.GRPC.Timeout) * time.Second
}
