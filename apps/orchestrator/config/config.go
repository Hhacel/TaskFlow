package config

import (
	"fmt"
	"log/slog"
	"os"

	"github.com/hhace/taskflow/pkg/persistence"
	"gopkg.in/yaml.v3"
)

// Config represents the Orchestrator configuration.
type Config struct {
	Server       ServerConfig       `yaml:"server"`
	Database     persistence.Config `yaml:"database"`
	NATS         NATSConfig         `yaml:"nats"`
	Orchestrator OrchestratorConfig `yaml:"orchestrator"`
	Logging      LoggingConfig      `yaml:"logging"`
}

// ServerConfig contains HTTP server settings (used only for the health endpoint).
type ServerConfig struct {
	Port string `yaml:"port"`
}

// NATSConfig contains NATS connection settings and every subject the
// Orchestrator publishes to or subscribes on.
type NATSConfig struct {
	URL                   string `yaml:"url"`
	WorkflowStartSubject  string `yaml:"workflow_start_subject"`
	WorkflowCancelSubject string `yaml:"workflow_cancel_subject"`
	TaskDispatchSubject   string `yaml:"task_dispatch_subject"`
	TaskResultSubject     string `yaml:"task_result_subject"`
	QueueGroupName        string `yaml:"queue_group_name"`
	ReconnectWait         int    `yaml:"reconnect_wait_seconds"`
	MaxReconnects         int    `yaml:"max_reconnects"`
}

// OrchestratorConfig contains orchestration policy settings.
type OrchestratorConfig struct {
	// MaxAttempts is the maximum number of execution attempts for a task
	// before it is marked permanently FAILED (and the workflow fails with it).
	MaxAttempts int `yaml:"max_attempts"`
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
			TaskDispatchSubject:   "tasks.dispatch",
			TaskResultSubject:     "tasks.results",
			QueueGroupName:        "orchestrator",
			ReconnectWait:         2,
			MaxReconnects:         60,
		},
		Orchestrator: OrchestratorConfig{
			MaxAttempts: 3,
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

	if filepath == "" {
		slog.Warn("No config file path provided, using defaults")
		return config, nil
	}

	data, err := os.ReadFile(filepath)
	if err != nil {
		slog.Warn("Could not read config file, using defaults", "file", filepath, "error", err)
		return config, nil
	}

	if err := yaml.Unmarshal(data, config); err != nil {
		slog.Warn("Could not parse config file, using defaults", "file", filepath, "error", err)
		return config, nil
	}

	slog.Info("Config loaded successfully", "file", filepath, "dbHost", config.Database.Host)
	return config, nil
}

// LoadConfigWithEnvOverrides loads config from file and applies environment variable overrides
func LoadConfigWithEnvOverrides(filepath string) (*Config, error) {
	config, err := LoadConfig(filepath)
	if err != nil {
		return nil, err
	}

	if port := os.Getenv("PORT"); port != "" {
		config.Server.Port = port
	}
	if dbHost := os.Getenv("DB_HOST"); dbHost != "" {
		config.Database.Host = dbHost
	}
	if dbPort := os.Getenv("DB_PORT"); dbPort != "" {
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
		config.NATS.URL = natsURL
	}
	if logLevel := os.Getenv("LOG_LEVEL"); logLevel != "" {
		config.Logging.Level = logLevel
	}

	return config, nil
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
	if c.NATS.URL == "" {
		return fmt.Errorf("nats url is required")
	}
	if c.Orchestrator.MaxAttempts <= 0 {
		return fmt.Errorf("orchestrator max_attempts must be positive")
	}
	return nil
}
