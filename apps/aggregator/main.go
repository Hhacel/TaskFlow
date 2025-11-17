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
		port = "8084"
	}

	r := gin.Default()
	
	r.GET("/health", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"status": "ok", "service": "aggregator"})
	})

	slog.Info("Aggregator starting", "port", port)
	if err := r.Run(":" + port); err != nil {
		slog.Error("Failed to start Aggregator", "error", err)
		os.Exit(1)
	}
}
