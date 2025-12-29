package routes

import (
	"log"

	"mikrotik-collector/internal/handlers"
	"mikrotik-collector/internal/middleware"

	"github.com/gin-gonic/gin"
)

// SetupRoutes configures all application routes
func SetupRoutes(
	router *gin.Engine,
	wsHandler *handlers.WebSocketHandler,
	trafficHandler *handlers.TrafficMonitorHandler,
	callbackHandler *handlers.CallbackHandler,
	customerHandler *handlers.CustomerHandler,
) *gin.Engine {
	// Apply global middleware
	router.Use(middleware.CORS())
	// router.Use(middleware.Logger()) // Use standard Gin logger or custom
	router.Use(gin.Recovery())

	// WebSocket endpoint
	router.GET("/ws", wsHandler.HandleWS)

	// Health check endpoint
	router.GET("/health", wsHandler.HandleHealthCheck)

	// API routes
	api := router.Group("/api")
	{
		// Callback routes (MikroTik WebHooks)
		callbacks := api.Group("/callbacks")
		{
			callbacks.POST("/pppoe-up", callbackHandler.HandlePPPoEUp)
			callbacks.POST("/pppoe-down", callbackHandler.HandlePPPoEDown)
		}

		// Customer routes (CRUD)
		customers := api.Group("/customers")
		{
			// CRUD operations (handled by CustomerHandler)
			customers.GET("", customerHandler.ListCustomers)
			customers.POST("", customerHandler.CreateCustomer)
			customers.GET("/:id", customerHandler.GetCustomer)
			customers.PUT("/:id", customerHandler.UpdateCustomer)
			customers.DELETE("/:id", customerHandler.DeleteCustomer)

			// Monitoring Specifics (handled by TrafficMonitorHandler)
			// These extend the customer resource
			customers.GET("/:customer_id/ping", trafficHandler.GetPingHandler().PingCustomerByID)
			customers.GET("/:customer_id/ping/ws", trafficHandler.GetPingHandler().PingCustomerStream)
			customers.GET("/:customer_id/traffic/ws", trafficHandler.StreamCustomerTraffic)
		}

		// Monitor routes
		monitor := api.Group("/monitor")
		{
			monitor.GET("/status", trafficHandler.GetStatus)
		}

		// Reload customers route
		// trafficHandler.ReloadCustomers might be deprecated, but keeping if logic exists
		api.POST("/reload-customers", trafficHandler.ReloadCustomers)
	}

	// Log registered routes
	log.Println("Routes registered")

	return router
}
