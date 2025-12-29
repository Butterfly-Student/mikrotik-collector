package middleware

import (
	"github.com/gin-gonic/gin"
)

// CORS adds CORS headers to responses
func CORS() gin.HandlerFunc {
	return func(c *gin.Context) {
		// Allow all origins (for development - restrict in production)
		c.Writer.Header().Set("Access-Control-Allow-Origin", "*")
		c.Writer.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
		c.Writer.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization")

		// Handle preflight requests
		if c.Request.Method == "OPTIONS" {
			c.AbortWithStatus(200)
			return
		}

		c.Next()
	}
}
