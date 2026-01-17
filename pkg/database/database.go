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

// Config holds database configuration
type Config struct {
	Host                   string `yaml:"host"`
	Port                   string `yaml:"port"`
	User                   string `yaml:"user"`
	Password               string `yaml:"password"`
	Database               string `yaml:"database"`
	SSLMode                string `yaml:"ssl_mode"`
	MaxOpenConns           int    `yaml:"max_open_conns"`
	MaxIdleConns           int    `yaml:"max_idle_conns"`
	ConnMaxLifetimeMinutes int    `yaml:"conn_max_lifetime_minutes"`
}

// Connect establishes connection to PostgreSQL database with retry logic
func Connect(config Config, maxRetries int) (*gorm.DB, error) {
	dsn := fmt.Sprintf(
		"host=%s port=%s user=%s password=%s dbname=%s sslmode=%s",
		config.Host, config.Port, config.User, config.Password, config.Database, config.SSLMode,
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
	var db *gorm.DB
	for i := 0; i < maxRetries; i++ {
		db, err = gorm.Open(postgres.Open(dsn), &gorm.Config{
			Logger: gormLogger,
		})

		if err == nil {
			// Test the connection
			if sqlDB, dbErr := db.DB(); dbErr == nil && sqlDB.Ping() == nil {
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
		return nil, fmt.Errorf("failed to connect to database after %d retries: %w", maxRetries, err)
	}

	// Configure connection pool
	sqlDB, err := db.DB()
	if err != nil {
		return nil, fmt.Errorf("failed to get database instance: %w", err)
	}

	// Connection pool settings
	sqlDB.SetMaxIdleConns(config.MaxIdleConns)
	sqlDB.SetMaxOpenConns(config.MaxOpenConns)
	sqlDB.SetConnMaxLifetime(time.Duration(config.ConnMaxLifetimeMinutes) * time.Minute)

	slog.Info("Database connected successfully",
		"host", config.Host,
		"database", config.Database,
		"maxOpenConns", config.MaxOpenConns,
		"maxIdleConns", config.MaxIdleConns)
		
	return db, nil
}

// Close closes the database connection
func Close(db *gorm.DB) error {
	if db == nil {
		return nil
	}

	sqlDB, err := db.DB()
	if err != nil {
		return err
	}

	return sqlDB.Close()
}

// Migrate runs auto migration for all models
func Migrate(db *gorm.DB) error {
	if db == nil {
		return fmt.Errorf("database not connected")
	}

	// Enable UUID extension for PostgreSQL
	if err := db.Exec("CREATE EXTENSION IF NOT EXISTS \"uuid-ossp\"").Error; err != nil {
		return fmt.Errorf("failed to create uuid extension: %w", err)
	}

	// Auto migrate your models here
	err := db.AutoMigrate(
		&models.Task{},
		&models.TaskExecutionResult{},
		// Add other models here as needed
	)

	if err != nil {
		return fmt.Errorf("failed to migrate database: %w", err)
	}

	slog.Info("Database migration completed")
	return nil
}

// Health checks database connection
func Health(db *gorm.DB) error {
	if db == nil {
		return fmt.Errorf("database not connected")
	}

	sqlDB, err := db.DB()
	if err != nil {
		return err
	}

	return sqlDB.Ping()
}
