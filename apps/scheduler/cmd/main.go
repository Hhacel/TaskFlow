package main

import (
	"log/slog"
	"os"

	"github.com/gin-gonic/gin"
	"github.com/hhace/taskflow/apps/scheduler/internal/api"
	"github.com/hhace/taskflow/apps/scheduler/internal/handlers"
	"github.com/hhace/taskflow/pkg/database"
)

func main() {
	// Initialize database connection
	slog.Info("Initializing database connection")
	config := database.LoadConfig()
	
	if err := database.Connect(config, 10); err != nil {
		slog.Error("Failed to connect to database", "error", err)
		os.Exit(1)
	}
	defer database.Close()

	// Run database migrations
	slog.Info("Running database migrations")
	if err := database.Migrate(); err != nil {
		slog.Error("Failed to migrate database", "error", err)
		os.Exit(1)
	}

	// Initialize services
	taskRepo := database.NewTaskRepository(database.DB)
	server, err := handlers.NewSchedulerServer(taskRepo)
	if err != nil {
		slog.Error("Failed to create scheduler server", "error", err)
		os.Exit(1)
	}

	// Setup routes
	r := setupRoutes(server)

	// Start server
	port := os.Getenv("PORT")
	if port == "" {
		port = "8081"
	}

	slog.Info("Scheduler service starting", "port", port)
	if err := r.Run(":" + port); err != nil {
		slog.Error("Failed to start scheduler service", "error", err)
		os.Exit(1)
	}
}

// setupRoutes configures all the routes using the generated API
func setupRoutes(server api.ServerInterface) *gin.Engine {
	r := gin.Default()
	
	// Register the generated API routes
	api.RegisterHandlers(r, server)

	return r
}