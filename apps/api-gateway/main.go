package main

import (
	"log/slog"
	"net/http"
	"os"

	"github.com/gin-gonic/gin"
)

func main() {
	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}

	r := gin.Default()
	
	// Health check
	r.GET("/health", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"status": "ok", "service": "api-gateway"})
	})

	// API routes
	v1 := r.Group("/api/v1")
	{
		v1.GET("/tasks", getTasks)
		v1.POST("/tasks", createTask)
	}

	slog.Info("API Gateway starting", "port", port)
	if err := r.Run(":" + port); err != nil {
		slog.Error("Failed to start API Gateway", "error", err)
		os.Exit(1)
	}
}

func getTasks(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{"tasks": []string{}})
}

func createTask(c *gin.Context) {
	c.JSON(http.StatusCreated, gin.H{"message": "task created"})
}
