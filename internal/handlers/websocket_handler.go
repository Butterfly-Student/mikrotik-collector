package handlers

import (
	"log"
	"net/http"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/gorilla/websocket"
)

var upgrader = websocket.Upgrader{
	CheckOrigin: func(r *http.Request) bool { return true },
}

// WebSocketHandler handles WebSocket connections and broadcasting
type WebSocketHandler struct {
	clients   map[*websocket.Conn]bool
	clientsMu sync.RWMutex
	broadcast chan []byte
}

// NewWebSocketHandler creates a new WebSocket handler
func NewWebSocketHandler() *WebSocketHandler {
	return &WebSocketHandler{
		clients:   make(map[*websocket.Conn]bool),
		broadcast: make(chan []byte),
	}
}

// GetBroadcastChannel returns the broadcast channel for other components
func (h *WebSocketHandler) GetBroadcastChannel() chan []byte {
	return h.broadcast
}

// GetClientCount returns the number of connected clients
func (h *WebSocketHandler) GetClientCount() int {
	h.clientsMu.RLock()
	defer h.clientsMu.RUnlock()
	return len(h.clients)
}

// HandleWS handles WebSocket connection requests
func (h *WebSocketHandler) HandleWS(c *gin.Context) {
	ws, err := upgrader.Upgrade(c.Writer, c.Request, nil)
	if err != nil {
		log.Printf("WebSocket upgrade error: %v", err)
		return
	}

	log.Printf("New WebSocket client connected from %s", c.Request.RemoteAddr)

	h.clientsMu.Lock()
	h.clients[ws] = true
	h.clientsMu.Unlock()

	defer func() {
		h.clientsMu.Lock()
		delete(h.clients, ws)
		h.clientsMu.Unlock()
		ws.Close()
		log.Printf("WebSocket client disconnected")
	}()

	for {
		if _, _, err := ws.ReadMessage(); err != nil {
			break
		}
	}
}

// Broadcaster runs in a goroutine to broadcast messages to all clients
func (h *WebSocketHandler) Broadcaster() {
	for {
		msg := <-h.broadcast

		h.clientsMu.RLock()
		for client := range h.clients {
			err := client.WriteMessage(websocket.TextMessage, msg)
			if err != nil {
				log.Printf("Write error: %v", err)
				client.Close()

				h.clientsMu.RUnlock()
				h.clientsMu.Lock()
				delete(h.clients, client)
				h.clientsMu.Unlock()
				h.clientsMu.RLock()
			}
		}
		h.clientsMu.RUnlock()
	}
}

// HandleHealthCheck handles health check endpoint
func (h *WebSocketHandler) HandleHealthCheck(c *gin.Context) {
	c.JSON(200, gin.H{
		"status":    "ok",
		"timestamp": time.Now().Format(time.RFC3339),
		"clients":   h.GetClientCount(),
	})
}
