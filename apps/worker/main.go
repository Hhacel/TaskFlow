package main

import (
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"

	"github.com/gin-gonic/gin"
	"github.com/hhace/taskflow/apps/worker/config"
	"github.com/hhace/taskflow/apps/worker/internal/consumer"
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
		"natsURL", cfg.NATS.URL,
		"taskTimeout", cfg.GetTaskTimeout(),
		"maxConcurrentTasks", cfg.Worker.MaxConcurrentTasks)

	// Initialize task consumer
	taskConsumer, err := consumer.NewTaskConsumer(cfg)
	if err != nil {
		slog.Error("Failed to create task consumer", "error", err)
		os.Exit(1)
	}
	defer taskConsumer.Stop()

	// Start consuming tasks
	if err := taskConsumer.Start(); err != nil {
		slog.Error("Failed to start task consumer", "error", err)
		os.Exit(1)
	}

	// Setup HTTP server for health checks
	r := gin.Default()

	r.GET("/health", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{
			"status":  "ok",
			"service": "worker",
			"nats":    "connected",
		})
	})

	r.GET("/config", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{
			"server": cfg.Server,
			"nats": gin.H{
				"url":         cfg.NATS.URL,
				"queue_group": cfg.NATS.QueueGroupName,
			},
			"worker": cfg.Worker,
		})
	})

	// Start HTTP server in a goroutine
	go func() {
		slog.Info("Worker HTTP server starting", "port", cfg.Server.Port)
		if err := r.Run(":" + cfg.Server.Port); err != nil {
			slog.Error("Failed to start HTTP server", "error", err)
		}
	}()

	slog.Info("Worker started successfully")

	// Wait for interrupt signal to gracefully shutdown
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	slog.Info("Shutting down worker...")
}
