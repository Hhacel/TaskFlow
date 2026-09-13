package config

import (
	"fmt"
	"log/slog"
	"os"
	"time"

	"github.com/hhace/taskflow/pkg/persistence"
	"gopkg.in/yaml.v3"
)

// Config represents the API Gateway configuration
type Config struct {
	Server   ServerConfig       `yaml:"server"`
	Database persistence.Config `yaml:"database"`
	NATS     NATSConfig         `yaml:"nats"`
	Logging  LoggingConfig      `yaml:"logging"`
}

// ServerConfig contains HTTP server settings
type ServerConfig struct {
	Port string `yaml:"port"`
}

// NATSConfig contains NATS connection settings and the subjects used to send
// control commands to the Orchestrator.
type NATSConfig struct {
	URL                   string `yaml:"url"`
	WorkflowStartSubject  string `yaml:"workflow_start_subject"`
	WorkflowCancelSubject string `yaml:"workflow_cancel_subject"`
	ReconnectWait         int    `yaml:"reconnect_wait_seconds"`
	MaxReconnects         int    `yaml:"max_reconnects"`
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
		NATS: NATSConfig{
			URL:                   "nats://nats:4222",
			WorkflowStartSubject:  "workflow.commands.start",
			WorkflowCancelSubject: "workflow.commands.cancel",
			ReconnectWait:         2,
			MaxReconnects:         60,
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
		slog.Warn("No config file path provided, using defaults")
		return config, nil
	}

	// Read the config file
	data, err := os.ReadFile(filepath)
	if err != nil {
		slog.Warn("Could not read config file, using defaults", "file", filepath, "error", err)
		return config, nil // Return defaults instead of error
	}

	// Parse YAML
	if err := yaml.Unmarshal(data, config); err != nil {
		slog.Warn("Could not parse config file, using defaults", "file", filepath, "error", err)
		return config, nil // Return defaults instead of error
	}

	slog.Info("Config loaded successfully", "file", filepath, "dbHost", config.Database.Host)
	return config, nil
}

// LoadConfigWithEnvOverrides loads config from file and applies environment variable overrides
func LoadConfigWithEnvOverrides(filepath string) (*Config, error) {
	slog.Info("LoadConfigWithEnvOverrides called", "path", filepath)
	config, err := LoadConfig(filepath)
	if err != nil {
		return nil, err
	}

	// Override with environment variables if they exist
	if port := os.Getenv("PORT"); port != "" {
		slog.Info("Overriding port with env var", "port", port)
		config.Server.Port = port
	}

	if dbHost := os.Getenv("DB_HOST"); dbHost != "" {
		slog.Info("Overriding DB host with env var", "dbHost", dbHost)
		config.Database.Host = dbHost
	}

	if dbPort := os.Getenv("DB_PORT"); dbPort != "" {
		slog.Info("Overriding DB port with env var", "dbPort", dbPort)
		config.Database.Port = dbPort
	}

	if dbUser := os.Getenv("DB_USER"); dbUser != "" {
		slog.Info("Overriding DB user with env var", "dbUser", dbUser)
		config.Database.User = dbUser
	}

	if dbPassword := os.Getenv("DB_PASSWORD"); dbPassword != "" {
		slog.Info("Overriding DB password with env var")
		config.Database.Password = dbPassword
	}

	if dbName := os.Getenv("DB_NAME"); dbName != "" {
		slog.Info("Overriding DB name with env var", "dbName", dbName)
		config.Database.Database = dbName
	}

	if natsURL := os.Getenv("NATS_URL"); natsURL != "" {
		slog.Info("Overriding NATS URL with env var", "natsURL", natsURL)
		config.NATS.URL = natsURL
	}

	if logLevel := os.Getenv("LOG_LEVEL"); logLevel != "" {
		config.Logging.Level = logLevel
	}

	return config, nil
}

// GetDSN returns the PostgreSQL connection string
func (c *Config) GetDSN() string {
	return fmt.Sprintf("host=%s port=%s user=%s password=%s dbname=%s sslmode=%s",
		c.Database.Host,
		c.Database.Port,
		c.Database.User,
		c.Database.Password,
		c.Database.Database,
		c.Database.SSLMode,
	)
}

// GetConnMaxLifetime returns the connection max lifetime as a duration
func (c *Config) GetConnMaxLifetime() time.Duration {
	return time.Duration(c.Database.ConnMaxLifetimeMinutes) * time.Minute
}

// Validate checks if the configuration is valid
func (c *Config) Validate() error {
	if c.Server.Port == "" {
		return fmt.Errorf("server port is required")
	}

	if c.Database.Host == "" {
		return fmt.Errorf("database host is required")
	}

	if c.Database.User == "" {
		return fmt.Errorf("database user is required")
	}

	if c.Database.Database == "" {
		return fmt.Errorf("database name is required")
	}

	if c.Database.MaxOpenConns <= 0 {
		return fmt.Errorf("max open connections must be positive")
	}

	if c.Database.MaxIdleConns <= 0 {
		return fmt.Errorf("max idle connections must be positive")
	}

	return nil
}
