package main

import (
	"encoding/json"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"mikrotik-collector/internal/application/services"
	"mikrotik-collector/internal/infrastructure/mikrotik"

	"github.com/gorilla/websocket"
	"github.com/joho/godotenv"
)

var upgrader = websocket.Upgrader{
	CheckOrigin: func(r *http.Request) bool { return true },
}

var clients = make(map[*websocket.Conn]bool)
var broadcast = make(chan []byte)

func handleWS(w http.ResponseWriter, r *http.Request) {
	ws, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Printf("WebSocket upgrade error: %v", err)
		return
	}

	log.Printf("New WebSocket client connected from %s", r.RemoteAddr)
	clients[ws] = true

	defer func() {
		delete(clients, ws)
		ws.Close()
		log.Printf("WebSocket client disconnected")
	}()

	for {
		if _, _, err := ws.ReadMessage(); err != nil {
			break
		}
	}
}

func broadcaster() {
	for {
		msg := <-broadcast

		for client := range clients {
			err := client.WriteMessage(websocket.TextMessage, msg)
			if err != nil {
				log.Printf("Write error: %v", err)
				client.Close()
				delete(clients, client)
			}
		}
	}
}

func healthCheck(w http.ResponseWriter, r *http.Request) {
	status := map[string]interface{}{
		"status":    "ok",
		"timestamp": time.Now().Format(time.RFC3339),
		"clients":   len(clients),
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(status)
}

func main() {
	godotenv.Load()
	log.Println("=== MikroTik Traffic Monitor (Continuous Mode) ===")

	cfg := LoadConfig()
	if err := cfg.Validate(); err != nil {
		log.Fatal(err)
	}

	log.Printf("Config: MikroTik=%s:%s, Redis=%s, WS Port=%s, DB=%s:%d",
		cfg.MikroTikHost, cfg.MikroTikPort, cfg.RedisAddr, cfg.WSPort, cfg.DBHost, cfg.DBPort)

	// Initialize MikroTik client for Background Monitoring
	mtClient, err := mikrotik.NewClient(mikrotik.Config{
		Host:     cfg.MikroTikHost,
		Port:     cfg.MikroTikPortInt(),
		Username: cfg.MikroTikUsername,
		Password: cfg.MikroTikPassword,
		Timeout:  10 * time.Second,
		UseTLS:   false,
		Queue:    100,
	})
	if err != nil {
		log.Fatalf("Failed to connect to MikroTik (Background): %v", err)
	}
	defer mtClient.Close()
	log.Println("MikroTik (Background) connected successfully")

	// Initialize MikroTik client for Interactive Tasks (Ping, etc.)
	mtInteractiveClient, err := mikrotik.NewClient(mikrotik.Config{
		Host:     cfg.MikroTikHost,
		Port:     cfg.MikroTikPortInt(),
		Username: cfg.MikroTikUsername,
		Password: cfg.MikroTikPassword,
		Timeout:  10 * time.Second,
		UseTLS:   false,
		Queue:    100,
	})
	if err != nil {
		log.Fatalf("Failed to connect to MikroTik (Interactive): %v", err)
	}
	defer mtInteractiveClient.Close()
	log.Println("MikroTik (Interactive) connected successfully")

	// Initialize Redis publisher
	publisher := NewRedisPublisher(cfg)
	defer publisher.Close()

	// Initialize database and continuous traffic service
	var trafficService *services.ContinuousTrafficService
	var customerRepo *services.DatabaseCustomerRepository

	if cfg.EnableTrafficMonitor {
		db, err := InitDatabase(cfg)
		if err != nil {
			log.Printf("WARNING: Database connection failed: %v", err)
			log.Println("Traffic monitoring will be disabled")
			cfg.EnableTrafficMonitor = false
		} else {
			defer db.Close()
			log.Println("Database connected successfully")

			// Initialize continuous traffic service (Uses Background Client)
			customerRepo = services.NewDatabaseCustomerRepository(db)
			trafficService = services.NewContinuousTrafficService(
				mtClient,
				customerRepo,
				publisher,
			)

			// Start continuous monitoring
			if err := trafficService.Start(); err != nil {
				log.Fatalf("Failed to start continuous monitoring: %v", err)
			}
			defer trafficService.Stop()

			log.Println("Continuous traffic monitoring started")
		}
	}

	// Start Redis Stream consumer
	streamConsumer := NewRedisStreamConsumer(cfg, broadcast)
	go streamConsumer.Start()
	defer streamConsumer.Close()

	// Start WebSocket broadcaster
	go broadcaster()

	// Setup HTTP routes
	mux := http.NewServeMux()
	mux.HandleFunc("/ws", handleWS)
	mux.HandleFunc("/health", healthCheck)

	if cfg.EnableTrafficMonitor && trafficService != nil {
		log.Println("Registering traffic monitor routes...")
		handler := NewTrafficMonitorHandler(trafficService, customerRepo, mtInteractiveClient)
		handler.RegisterRoutes(mux)
	} else {
		log.Printf("Skipping traffic monitor routes registration. EnableTrafficMonitor=%v, trafficService=%v", cfg.EnableTrafficMonitor, trafficService != nil)
	}

	// Wrap with CORS middleware
	httpHandler := ChainMiddleware(mux, CORSMiddleware)

	server := &http.Server{
		Addr:         ":" + cfg.WSPort,
		Handler:      httpHandler,
		ReadTimeout:  15 * time.Second,
		WriteTimeout: 15 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

	go func() {
		log.Printf("Server started on :%s", cfg.WSPort)
		log.Printf("- WebSocket: ws://localhost:%s/ws", cfg.WSPort)
		log.Printf("- Health: http://localhost:%s/health", cfg.WSPort)
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatal(err)
		}
	}()

	// Graceful shutdown
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)
	<-sigChan

	log.Println("\nShutting down gracefully...")
	log.Println("Shutdown complete")
}
