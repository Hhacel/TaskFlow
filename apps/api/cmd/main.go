package main

import (
	"log/slog"
	"os"
	"os/signal"
	"syscall"

	"github.com/gin-gonic/gin"
	schedulerConfig "github.com/hhace/taskflow/apps/api/config"
	"github.com/hhace/taskflow/apps/api/internal/api"
	"github.com/hhace/taskflow/apps/api/internal/handlers"
	"github.com/hhace/taskflow/pkg/persistence"
)

func main() {
	// Load configuration
	configPath := os.Getenv("CONFIG_PATH")
	cfg, err := schedulerConfig.LoadConfigWithEnvOverrides(configPath)
	if err != nil {
		slog.Error("Failed to load configuration", "error", err)
		os.Exit(1)
	}

	// Validate configuration
	if err := cfg.Validate(); err != nil {
		slog.Error("Invalid configuration", "error", err)
		os.Exit(1)
	}

	slog.Info("Configuration loaded",
		"port", cfg.Server.Port,
		"dbHost", cfg.Database.Host)

	// Initialize database connection
	slog.Info("Initializing database connection")

	db, err := persistence.Connect(cfg.Database, 10)

	if err != nil {
		slog.Error("Failed to connect to database", "error", err)
		os.Exit(1)
	}
	defer persistence.Close(db)

	// Run database migrations
	slog.Info("Running database migrations")
	if err := persistence.Migrate(db); err != nil {
		slog.Error("Failed to migrate database", "error", err)
		os.Exit(1)
	}

	// Initialize services
	repo := persistence.NewRepository(db)
	server, err := handlers.NewSchedulerServer(cfg, repo)
	if err != nil {
		slog.Error("Failed to create scheduler server", "error", err)
		os.Exit(1)
	}

	// Setup routes
	r := setupRoutes(server, cfg)

	// Start server in a goroutine
	go func() {
		slog.Info("API service starting", "port", cfg.Server.Port)
		if err := r.Run(":" + cfg.Server.Port); err != nil {
			slog.Error("Failed to start API service", "error", err)
			os.Exit(1)
		}
	}()

	slog.Info("API started successfully")

	// Wait for interrupt signal to gracefully shutdown
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	slog.Info("Shutting down API...")
}

// setupRoutes configures all the routes using the generated API
func setupRoutes(server api.ServerInterface, cfg *schedulerConfig.Config) *gin.Engine {
	r := gin.Default()

	// Register the generated API routes
	api.RegisterHandlers(r, server)

	// Add config endpoint
	r.GET("/config", func(c *gin.Context) {
		c.JSON(200, gin.H{
			"server": cfg.Server,
			"database": gin.H{
				"host":     cfg.Database.Host,
				"port":     cfg.Database.Port,
				"database": cfg.Database.Database,
			},
		})
	})

	return r
}
