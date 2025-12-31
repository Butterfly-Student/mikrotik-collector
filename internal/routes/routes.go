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
	router.Use(gin.Recovery())

	// Serve static files from frontend directory
	router.Static("/css", "./frontend/css")
	router.Static("/js", "./frontend/js")

	// Serve HTML files
	router.StaticFile("/", "./frontend/index.html")
	router.StaticFile("/index.html", "./frontend/index.html")
	router.StaticFile("/add-customer.html", "./frontend/add-customer.html")
	router.StaticFile("/edit-customer.html", "./frontend/edit-customer.html")

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
			customers.GET("/:id/ping", trafficHandler.GetPingHandler().PingCustomerByID)
			customers.GET("/:id/ping/ws", trafficHandler.GetPingHandler().PingCustomerStream)
			customers.GET("/:id/traffic/ws", trafficHandler.StreamCustomerTraffic)
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
