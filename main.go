package main

import (
	"context"
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
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
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

	// Initialize DB with GORM
	var db *gorm.DB
	if cfg.EnableTrafficMonitor {
		dbConn, err := InitDatabaseGORM(cfg)
		if err != nil {
			log.Printf("WARNING: Database connection failed: %v", err)
			cfg.EnableTrafficMonitor = false
		} else {
			db = dbConn
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
		callbackHandler = handlers.NewCallbackHandler(customerRepo, publisher)
		customerHandler = handlers.NewCustomerHandler(customerService)
	} else {
		// Fallback if DB connects fails, but wait, TrafficHandler needs repo...
		// If DB fails, we probably can't run most things.
		log.Println("Running in limited mode (No Database)")
	}

	// Setup Redis Event Subscriber (Global)
	go func() {
		ctx := context.Background()
		pubsub := publisher.client.Subscribe(ctx, "mikrotik:events")
		defer pubsub.Close()

		log.Println("Subscribed to 'mikrotik:events' Redis channel")

		ch := pubsub.Channel()
		broadcastChan := wsHandler.GetBroadcastChannel()

		for msg := range ch {
			// Forward Redis Pub/Sub message to WebSocket Broadcast channel
			log.Printf("[Redis] Received event: %s", msg.Payload)
			broadcastChan <- []byte(msg.Payload)
		}
	}()

	// Setup Gin router
	gin.SetMode(gin.ReleaseMode)
	router := gin.New()

	// Setup routes (API only, no template rendering)
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

	// Start WebSocket Broadcaster
	go wsHandler.Broadcaster()

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

// InitDatabaseGORM initializes GORM database connection
func InitDatabaseGORM(cfg *Config) (*gorm.DB, error) {
	log.Println("[Database] Initializing GORM connection...")

	dsn := cfg.GetDSN()
	log.Printf("[Database] DSN: host=%s port=%d dbname=%s user=%s",
		cfg.DBHost, cfg.DBPort, cfg.DBName, cfg.DBUser)

	// Configure GORM logger
	gormLogger := logger.New(
		log.New(os.Stdout, "\r\n", log.LstdFlags),
		logger.Config{
			SlowThreshold:             200 * time.Millisecond,
			LogLevel:                  logger.Info,
			IgnoreRecordNotFoundError: true,
			Colorful:                  true,
		},
	)

	db, err := gorm.Open(postgres.Open(dsn), &gorm.Config{
		Logger: gormLogger,
		NowFunc: func() time.Time {
			return time.Now().Local()
		},
	})

	if err != nil {
		log.Printf("[Database] ERROR: Failed to connect: %v", err)
		return nil, err
	}

	// Get underlying sql.DB for connection pool settings
	sqlDB, err := db.DB()
	if err != nil {
		log.Printf("[Database] ERROR: Failed to get underlying DB: %v", err)
		return nil, err
	}

	// Connection pool settings
	sqlDB.SetMaxIdleConns(10)
	sqlDB.SetMaxOpenConns(100)
	sqlDB.SetConnMaxLifetime(time.Hour)

	log.Println("[Database] Testing connection...")
	if err := sqlDB.Ping(); err != nil {
		log.Printf("[Database] ERROR: Ping failed: %v", err)
		return nil, err
	}

	log.Println("[Database] SUCCESS: Connection established and tested")
	return db, nil
}
