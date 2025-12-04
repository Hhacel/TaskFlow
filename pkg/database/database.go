package database

import (
	"fmt"
	"log"
	"log/slog"
	"os"
	"time"

	"github.com/hhace/taskflow/models"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

// DB is the global database instance
var DB *gorm.DB

// Config holds database configuration
type Config struct {
	Host     string
	Port     string
	User     string
	Password string
	DBName   string
	SSLMode  string
}

// LoadConfig loads database configuration from environment variables
func LoadConfig() *Config {
	return &Config{
		Host:     getEnv("DB_HOST", "localhost"),
		Port:     getEnv("DB_PORT", "5432"),
		User:     getEnv("DB_USER", "taskflow"),
		Password: getEnv("DB_PASSWORD", "taskflow"),
		DBName:   getEnv("DB_NAME", "taskflow"),
		SSLMode:  getEnv("DB_SSLMODE", "disable"),
	}
}

// Connect establishes connection to PostgreSQL database with retry logic
func Connect(config *Config, maxRetries int) error {
	dsn := fmt.Sprintf(
		"host=%s port=%s user=%s password=%s dbname=%s sslmode=%s",
		config.Host, config.Port, config.User, config.Password, config.DBName, config.SSLMode,
	)

	// Configure GORM logger
	gormLogger := logger.New(
		log.New(os.Stdout, "\r\n", log.LstdFlags),
		logger.Config{
			SlowThreshold:             time.Second, // Slow SQL threshold
			LogLevel:                  logger.Info, // Log level
			IgnoreRecordNotFoundError: true,        // Ignore ErrRecordNotFound error for logger
			Colorful:                  true,        // Enable color
		},
	)

	// Simple retry logic with fixed delay
	var err error

	for i := 0; i < maxRetries; i++ {
		DB, err = gorm.Open(postgres.Open(dsn), &gorm.Config{
			Logger: gormLogger,
		})

		if err == nil {
			// Test the connection
			if sqlDB, dbErr := DB.DB(); dbErr == nil && sqlDB.Ping() == nil {
				break
			}
		}

		if i < maxRetries-1 {
			slog.Warn("Database connection failed, retrying", "attempt", i+1, "error", err)
			time.Sleep(2 * time.Second) // Simple 2-second delay
		} else {
			slog.Error("Database connection failed on final attempt", "attempt", i+1, "error", err)
		}
	}

	if err != nil {
		return fmt.Errorf("failed to connect to database after %d retries: %w", maxRetries, err)
	}

	// Configure connection pool
	sqlDB, err := DB.DB()
	if err != nil {
		return fmt.Errorf("failed to get database instance: %w", err)
	}

	// Connection pool settings
	sqlDB.SetMaxIdleConns(10)           // Maximum idle connections
	sqlDB.SetMaxOpenConns(100)          // Maximum open connections
	sqlDB.SetConnMaxLifetime(time.Hour) // Connection max lifetime

	slog.Info("Database connected successfully")
	return nil
}

// Close closes the database connection
func Close() error {
	if DB == nil {
		return nil
	}

	sqlDB, err := DB.DB()
	if err != nil {
		return err
	}

	return sqlDB.Close()
}

// Migrate runs auto migration for all models
func Migrate() error {
	if DB == nil {
		return fmt.Errorf("database not connected")
	}

	// Enable UUID extension for PostgreSQL
	if err := DB.Exec("CREATE EXTENSION IF NOT EXISTS \"uuid-ossp\"").Error; err != nil {
		return fmt.Errorf("failed to create uuid extension: %w", err)
	}

	// Auto migrate your models here
	err := DB.AutoMigrate(
		&models.Task{},
		// Add other models here as you create them
	)

	if err != nil {
		return fmt.Errorf("failed to migrate database: %w", err)
	}

	slog.Info("Database migration completed")
	return nil
}

// Helper function to get environment variables with default values
func getEnv(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}

// Health checks database connection
func Health() error {
	if DB == nil {
		return fmt.Errorf("database not connected")
	}

	sqlDB, err := DB.DB()
	if err != nil {
		return err
	}

	return sqlDB.Ping()
}

