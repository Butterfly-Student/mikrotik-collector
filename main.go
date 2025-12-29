package main

import (
	"context"
	"database/sql"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"mikrotik-collector/internal/application/services"
	"mikrotik-collector/internal/handlers"
	"mikrotik-collector/internal/infrastructure/mikrotik"
	"mikrotik-collector/internal/repository"
	"mikrotik-collector/internal/routes"

	"github.com/gin-gonic/gin"
	"github.com/joho/godotenv"
)

func main() {
	godotenv.Load()
	log.Println("=== MikroTik Traffic Monitor (On-Demand Mode) ===")

	cfg := LoadConfig()
	if err := cfg.Validate(); err != nil {
		log.Fatal(err)
	}

	log.Printf("Config: MikroTik=%s:%s, Redis=%s, WS Port=%s, DB=%s:%d",
		cfg.MikroTikHost, cfg.MikroTikPort, cfg.RedisAddr, cfg.WSPort, cfg.DBHost, cfg.DBPort)

	// Initialize MikroTik client
	mtClient, err := mikrotik.NewClient(mikrotik.Config{
		Host:     cfg.MikroTikHost,
		Port:     cfg.MikroTikPortInt(),
		Username: cfg.MikroTikUsername,
		Password: cfg.MikroTikPassword,
		Timeout:  10 * time.Second,
		UseTLS:   false,
		Queue:    1000,
	})
	if err != nil {
		log.Fatalf("Failed to connect to MikroTik: %v", err)
	}
	defer mtClient.Close()
	log.Println("MikroTik connected successfully")

	// Initialize WebSocket handler (global broadcasts)
	wsHandler := handlers.NewWebSocketHandler()

	// Initialize Redis publisher
	publisher := NewRedisPublisher(cfg)
	defer publisher.Close()

	// Initialize DB
	var db *sql.DB
	if cfg.EnableTrafficMonitor {
		dbConn, err := InitDatabase(cfg)
		if err != nil {
			log.Printf("WARNING: Database connection failed: %v", err)
			cfg.EnableTrafficMonitor = false
		} else {
			db = dbConn
			defer db.Close()
			log.Println("Database connected successfully")
		}
	}

	// Initialize Handlers placeholders
	var trafficHandler *handlers.TrafficMonitorHandler
	var callbackHandler *handlers.CallbackHandler
	var customerHandler *handlers.CustomerHandler

	// Initialize Services if DB is up
	if db != nil {
		// New Repositories
		customerRepo := repository.NewDatabaseCustomerRepository(db)

		// New Services
		trafficService := services.NewOnDemandTrafficService(mtClient, customerRepo, publisher)
		customerService := services.NewCustomerService(customerRepo, mtClient)

		// Create Handlers
		trafficHandler = handlers.NewTrafficMonitorHandler(trafficService, customerRepo, mtClient)
		callbackHandler = handlers.NewCallbackHandler(customerRepo)
		customerHandler = handlers.NewCustomerHandler(customerService)
	} else {
		// Fallback if DB connects fails, but wait, TrafficHandler needs repo...
		// If DB fails, we probably can't run most things.
		log.Println("Running in limited mode (No Database)")
	}

	// Setup Gin router
	gin.SetMode(gin.ReleaseMode)
	router := gin.New()

	// Setup routes
	if customerHandler != nil {
		log.Println("Setting up routes...")
		routes.SetupRoutes(router, wsHandler, trafficHandler, callbackHandler, customerHandler)
	} else {
		// Minimal setup
		router.Use(gin.Recovery())
		router.GET("/health", wsHandler.HandleHealthCheck)
	}

	// Create HTTP server
	server := &http.Server{
		Addr:         ":" + cfg.WSPort,
		Handler:      router,
		ReadTimeout:  15 * time.Second,
		WriteTimeout: 15 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

	// Start server
	go func() {
		log.Printf("Server started on :%s", cfg.WSPort)
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatal(err)
		}
	}()

	// Graceful shutdown
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)
	<-sigChan

	log.Println("\nShutting down gracefully...")

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := server.Shutdown(ctx); err != nil {
		log.Printf("Server shutdown error: %v", err)
	}

	log.Println("Shutdown complete")
}
