package middleware

import (
	"log"
	"time"

	"github.com/gin-gonic/gin"
)

// Logger logs HTTP requests with useful information
func Logger() gin.HandlerFunc {
	return func(c *gin.Context) {
		// Start timer
		start := time.Now()
		path := c.Request.URL.Path
		raw := c.Request.URL.RawQuery

		// Process request
		c.Next()

		// Calculate latency
		latency := time.Since(start)

		// Get status code
		statusCode := c.Writer.Status()

		// Build query string
		if raw != "" {
			path = path + "?" + raw
		}

		// Log request
		log.Printf("[GIN] %s %s %d %v %s",
			c.Request.Method,
			path,
			statusCode,
			latency,
			c.ClientIP(),
		)
	}
}
