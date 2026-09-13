package main

import (
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"

	"github.com/gin-gonic/gin"
	"github.com/hhace/taskflow/apps/orchestrator/config"
	"github.com/hhace/taskflow/apps/orchestrator/internal/orchestrator"
	"github.com/hhace/taskflow/pkg/messaging"
	"github.com/hhace/taskflow/pkg/persistence"
)

func main() {
	configPath := os.Getenv("CONFIG_PATH")
	cfg, err := config.LoadConfigWithEnvOverrides(configPath)
	if err != nil {
		slog.Error("Failed to load configuration", "error", err)
		os.Exit(1)
	}

	if err := cfg.Validate(); err != nil {
		slog.Error("Invalid configuration", "error", err)
		os.Exit(1)
	}

	slog.Info("Configuration loaded", "port", cfg.Server.Port, "dbHost", cfg.Database.Host, "natsURL", cfg.NATS.URL)

	db, err := persistence.Connect(cfg.Database, 10)
	if err != nil {
		slog.Error("Failed to connect to database", "error", err)
		os.Exit(1)
	}
	defer persistence.Close(db)

	if err := persistence.Migrate(db); err != nil {
		slog.Error("Failed to migrate database", "error", err)
		os.Exit(1)
	}

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

	repo := persistence.NewRepository(db)
	orch := orchestrator.New(cfg, repo, broker)

	if err := orch.Start(); err != nil {
		slog.Error("Failed to start orchestrator", "error", err)
		os.Exit(1)
	}
	defer orch.Stop()

	slog.Info("Orchestrator started successfully")

	// Minimal HTTP server exposing only a health check for container orchestration probes.
	r := gin.Default()
	r.GET("/health", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"status": "ok", "service": "orchestrator"})
	})

	go func() {
		if err := r.Run(":" + cfg.Server.Port); err != nil {
			slog.Error("Failed to start HTTP server", "error", err)
			os.Exit(1)
		}
	}()

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	slog.Info("Shutting down orchestrator...")
}
