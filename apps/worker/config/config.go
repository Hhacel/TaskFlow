package config

import (
	"fmt"
	"log/slog"
	"os"
	"time"

	"gopkg.in/yaml.v3"
)

// Config represents the worker configuration
type Config struct {
	Server  ServerConfig  `yaml:"server"`
	NATS    NATSConfig    `yaml:"nats"`
	Worker  WorkerConfig  `yaml:"worker"`
	Logging LoggingConfig `yaml:"logging"`
}

// ServerConfig contains HTTP server settings
type ServerConfig struct {
	Port string `yaml:"port"`
}

// NATSConfig contains NATS connection settings
type NATSConfig struct {
	URL                 string `yaml:"url"`
	TaskScheduleSubject string `yaml:"task_schedule_subject"`
	TaskResultSubject   string `yaml:"task_result_subject"`
	QueueGroupName      string `yaml:"queue_group_name"`
	ReconnectWait       int    `yaml:"reconnect_wait_seconds"`
	MaxReconnects       int    `yaml:"max_reconnects"`
}

// WorkerConfig contains worker execution settings
type WorkerConfig struct {
	TaskTimeoutMinutes int `yaml:"task_timeout_minutes"`
	MaxConcurrentTasks int `yaml:"max_concurrent_tasks"`
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
			Port: "8082",
		},
		NATS: NATSConfig{
			URL:                 "nats://nats:4222",
			TaskScheduleSubject: "tasks.schedule",
			TaskResultSubject:   "tasks.results",
			QueueGroupName:      "workers",
			ReconnectWait:       2,
			MaxReconnects:       60,
		},
		Worker: WorkerConfig{
			TaskTimeoutMinutes: 5,
			MaxConcurrentTasks: 10,
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
		slog.Warn("Could not read config file, using defaults", "path", filepath, "error", err)
		return config, nil // Return defaults instead of error
	}

	// Parse YAML
	if err := yaml.Unmarshal(data, config); err != nil {
		slog.Warn("Could not parse config file, using defaults", "path", filepath, "error", err)
		return config, nil // Return defaults instead of error
	}

	slog.Info("Config loaded successfully", "path", filepath, "natsURL", config.NATS.URL)
	return config, nil
}

// LoadConfigWithEnvOverrides loads config from file and applies environment variable overrides
func LoadConfigWithEnvOverrides(filepath string) (*Config, error) {
	slog.Debug("Loading config with env overrides", "path", filepath)
	config, err := LoadConfig(filepath)
	if err != nil {
		return nil, err
	}

	// Override with environment variables if they exist
	if port := os.Getenv("PORT"); port != "" {
		slog.Debug("Overriding port with env var", "value", port)
		config.Server.Port = port
	}

	if natsURL := os.Getenv("NATS_URL"); natsURL != "" {
		slog.Debug("Overriding NATS URL with env var", "value", natsURL)
		config.NATS.URL = natsURL
	}

	if timeout := os.Getenv("TASK_TIMEOUT"); timeout != "" {
		if duration, err := time.ParseDuration(timeout); err == nil {
			slog.Debug("Overriding task timeout with env var", "value", timeout)
			config.Worker.TaskTimeoutMinutes = int(duration.Minutes())
		}
	}

	if logLevel := os.Getenv("LOG_LEVEL"); logLevel != "" {
		config.Logging.Level = logLevel
	}

	return config, nil
}

// GetTaskTimeout returns the task timeout as a duration
func (c *Config) GetTaskTimeout() time.Duration {
	return time.Duration(c.Worker.TaskTimeoutMinutes) * time.Minute
}

// Validate checks if the configuration is valid
func (c *Config) Validate() error {
	if c.Server.Port == "" {
		return fmt.Errorf("server port is required")
	}

	if c.NATS.URL == "" {
		return fmt.Errorf("NATS URL is required")
	}

	if c.NATS.TaskScheduleSubject == "" {
		return fmt.Errorf("task schedule subject is required")
	}

	if c.NATS.TaskResultSubject == "" {
		return fmt.Errorf("task result subject is required")
	}

	if c.Worker.TaskTimeoutMinutes <= 0 {
		return fmt.Errorf("task timeout must be positive")
	}

	if c.Worker.MaxConcurrentTasks <= 0 {
		return fmt.Errorf("max concurrent tasks must be positive")
	}

	return nil
}
