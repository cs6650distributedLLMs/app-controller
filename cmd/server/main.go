package main

import (
	"log"
	"os"
	"time"
	"tldr/internal/api/routes"
	"tldr/internal/clients"
	"tldr/internal/core"

	"github.com/gin-gonic/gin"
	"github.com/joho/godotenv"
)

func main() {
	godotenv.Load()

	core.InitSnowflake()

	gin.SetMode(os.Getenv("GIN_MODE"))
	router := gin.Default()

	clients.InitDyanmoDb()

	// Setup CORS
	router.Use(func(c *gin.Context) {
		c.Writer.Header().Set("Access-Control-Allow-Origin", "*")
		c.Writer.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
		c.Writer.Header().Set("Access-Control-Allow-Headers", "Content-Type, Content-Length, Accept-Encoding, Authorization")
		if c.Request.Method == "OPTIONS" {
			c.AbortWithStatus(204)
			return
		}
		c.Next()
	})

	// Health check endpoint
	router.GET("/health", func(c *gin.Context) {
		c.JSON(200, gin.H{
			"status":    "ok",
			"service":   "tldr",
			"timestamp": time.Now().UnixMilli(),
		})
	})

	// Setup routes
	routes.SetupRoutes(router)

	// Get port from environment or use default
	port := os.Getenv("APP_PORT")
	if port == "" {
		port = "8080"
	}

	// Start the server
	log.Printf("Server listening on port %s", port)
	if err := router.Run(":" + port); err != nil {
		log.Fatalf("Failed to start server: %v", err)
	}
}
