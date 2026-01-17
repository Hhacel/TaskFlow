package main

import (
	"context"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/hhace/taskflow/apps/scheduler/config"
	"github.com/hhace/taskflow/apps/scheduler/internal/consumer"
	"github.com/hhace/taskflow/apps/scheduler/internal/scheduler"
	"github.com/hhace/taskflow/pkg/database"
	"github.com/nats-io/nats.go"
)

func main() {
	// Load configuration
	configPath := os.Getenv("CONFIG_PATH")
	if configPath == "" {
		configPath = "/app/config.yaml"
	}

	cfg, err := config.LoadConfigWithEnvOverrides(configPath)
	if err != nil {
		slog.Error("Failed to load configuration", "error", err)
		os.Exit(1)
	}

	slog.Info("Configuration loaded",
		"port", cfg.Server.Port,
		"dbHost", cfg.Database.Host,
		"natsURL", cfg.NATS.URL)

	// Connect to NATS
	opts := []nats.Option{
		nats.ReconnectWait(time.Duration(cfg.NATS.ReconnectWait) * time.Second),
		nats.MaxReconnects(cfg.NATS.MaxReconnects),
		nats.DisconnectErrHandler(func(nc *nats.Conn, err error) {
			if err != nil {
				slog.Warn("NATS disconnected", "error", err)
			}
		}),
		nats.ReconnectHandler(func(nc *nats.Conn) {
			slog.Info("NATS reconnected", "url", nc.ConnectedUrl())
		}),
	}

	nc, err := nats.Connect(cfg.NATS.URL, opts...)
	if err != nil {
		slog.Error("Failed to connect to NATS", "error", err)
		os.Exit(1)
	}

	slog.Info("Connected to NATS", "url", cfg.NATS.URL)

	// Connect to database
	db, err := database.Connect(cfg.Database, 10)
	if err != nil {
		slog.Error("Failed to connect to database", "error", err)
		os.Exit(1)
	}
	defer database.Close(db)
	slog.Info("Database connected successfully")

	// migrate the task execution results table
	if err := database.Migrate(db); err != nil {
		slog.Error("Failed to migrate database", "error", err)
		os.Exit(1)
	}

	repo := database.NewRepository(db)

	// Create and start result consumer
	resultConsumer, err := consumer.NewResultConsumer(cfg, repo, nc)
	if err != nil {
		slog.Error("Failed to create result consumer", "error", err)
		os.Exit(1)
	}

	if err := resultConsumer.Start(); err != nil {
		slog.Error("Failed to start result consumer", "error", err)
		os.Exit(1)
	}

	// Create and start task scheduler
	taskScheduler := scheduler.NewTaskScheduler(cfg, repo, resultConsumer.GetNATSConnection())
	if err := taskScheduler.Start(); err != nil {
		slog.Error("Failed to start task scheduler", "error", err)
		os.Exit(1)
	}

	// Setup HTTP server for health checks
	r := gin.Default()

	r.GET("/health", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{
			"status":   "healthy",
			"service":  "scheduler",
			"database": "connected",
		})
	})

	r.GET("/config", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{
			"port":           cfg.Server.Port,
			"nats_url":       cfg.NATS.URL,
			"result_subject": cfg.NATS.TaskResultSubject,
			"notifier_addr":  cfg.GRPC.NotifierAddress,
		})
	})

	// Start HTTP server in a goroutine
	srv := &http.Server{
		Addr:    ":" + cfg.Server.Port,
		Handler: r,
	}

	go func() {
		slog.Info("Scheduler HTTP server starting", "port", cfg.Server.Port)
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			slog.Error("Failed to start HTTP server", "error", err)
			os.Exit(1)
		}
	}()

	slog.Info("Scheduler started successfully")

	// Wait for interrupt signal to gracefully shutdown
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	slog.Info("Shutting down scheduler...")

	// Graceful shutdown
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	if err := srv.Shutdown(ctx); err != nil {
		slog.Error("HTTP server forced to shutdown", "error", err)
	}

	if err := resultConsumer.Stop(); err != nil {
		slog.Error("Failed to stop result consumer", "error", err)
	}

	taskScheduler.Stop()

	slog.Info("Scheduler stopped")
}