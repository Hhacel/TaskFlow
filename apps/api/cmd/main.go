package main

import (
	"log/slog"
	"os"
	"os/signal"
	"syscall"

	"github.com/gin-gonic/gin"
	"github.com/hhace/taskflow/apps/api/config"
	"github.com/hhace/taskflow/apps/api/internal/handlers"
	"github.com/hhace/taskflow/pkg/messaging"
	"github.com/hhace/taskflow/pkg/persistence"
)

func main() {
	// Load configuration
	configPath := os.Getenv("CONFIG_PATH")
	cfg, err := config.LoadConfigWithEnvOverrides(configPath)
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

	// Connect to NATS broker used to send control commands to the Orchestrator
	broker, err := messaging.NewNATSBroker(messaging.NATSConfig{
		URL:           cfg.NATS.URL,
		ReconnectWait: cfg.NATS.ReconnectWait,
		MaxReconnects: cfg.NATS.MaxReconnects,
	})
	if err != nil {
		slog.Error("Failed to connect to NATS", "error", err)
		os.Exit(1)
	}
	defer broker.Close()

	// Initialize services
	repo := persistence.NewRepository(db)
	handler := handlers.NewWorkflowHandler(cfg, repo, broker)

	// Setup routes
	r := setupRoutes(handler, cfg)

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

// setupRoutes configures every route for the 5 TaskFlow use cases.
func setupRoutes(h *handlers.WorkflowHandler, cfg *config.Config) *gin.Engine {
	r := gin.Default()

	v1 := r.Group("/api/v1")
	{
		v1.POST("/workflows", h.CreateWorkflow)            // PU-001
		v1.POST("/workflows/:id/start", h.StartWorkflow)   // PU-002
		v1.GET("/workflows/:id", h.GetWorkflowStatus)      // PU-003
		v1.GET("/tasks/:taskId/results", h.GetTaskResult)  // PU-004
		v1.POST("/workflows/:id/cancel", h.CancelWorkflow) // PU-005
	}

	r.GET("/health", func(c *gin.Context) {
		c.JSON(200, gin.H{"status": "ok", "service": "api"})
	})

	r.GET("/config", func(c *gin.Context) {
		c.JSON(200, gin.H{
			"server": cfg.Server,
			"database": gin.H{
				"host":     cfg.Database.Host,
				"port":     cfg.Database.Port,
				"database": cfg.Database.Database,
			},
			"nats": cfg.NATS,
		})
	})

	return r
}
