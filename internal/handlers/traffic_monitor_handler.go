package handlers

import (
	"log"
	"net/http"

	"mikrotik-collector/internal/application/services"
	"mikrotik-collector/internal/domain"
	"mikrotik-collector/internal/infrastructure/mikrotik"

	"github.com/gin-gonic/gin"
	"github.com/gorilla/websocket"
)

// TrafficMonitorHandler handles HTTP requests for traffic monitoring
type TrafficMonitorHandler struct {
	service     *services.OnDemandTrafficService
	repo        domain.CustomerRepository
	pingHandler *PingHandler
	mtClient    *mikrotik.Client
}

// NewTrafficMonitorHandler creates a new handler
func NewTrafficMonitorHandler(
	service *services.OnDemandTrafficService,
	repo domain.CustomerRepository,
	mtClient *mikrotik.Client,
) *TrafficMonitorHandler {
	return &TrafficMonitorHandler{
		service:     service,
		repo:        repo,
		pingHandler: NewPingHandler(mtClient, repo),
		mtClient:    mtClient,
	}
}

// GetStatus returns monitoring status
// GET /api/monitor/status
func (h *TrafficMonitorHandler) GetStatus(c *gin.Context) {
	// OnDemand service doesn't track global stats in the same way
	// We can return simple status
	c.JSON(200, gin.H{
		"status": "ok",
		"mode":   "on-demand",
	})
}

// ReloadCustomers handles forcing a reload (deprecated in OnDemand, but kept for compat)
// POST /api/reload-customers
func (h *TrafficMonitorHandler) ReloadCustomers(c *gin.Context) {
	c.JSON(200, gin.H{
		"status":  "success",
		"message": "Reload not required in On-Demand mode",
	})
}

// StreamCustomerTraffic streams traffic for a specific customer via WebSocket
// GET /api/customers/:customer_id/traffic/ws
func (h *TrafficMonitorHandler) StreamCustomerTraffic(c *gin.Context) {
	customerID := c.Param("customer_id")

	// Allow all origins for now
	upgrader := websocket.Upgrader{
		CheckOrigin:     func(r *http.Request) bool { return true },
		ReadBufferSize:  1024,
		WriteBufferSize: 1024,
	}

	// Upgrade to WebSocket
	ws, err := upgrader.Upgrade(c.Writer, c.Request, nil)
	if err != nil {
		log.Printf("WS upgrade error: %v", err)
		return
	}
	defer ws.Close()

	// Start On-Demand Monitoring
	// This returns a channel that receives traffic data for THIS connection
	streamChan, err := h.service.StartMonitoring(c.Request.Context(), customerID)
	if err != nil {
		log.Printf("[Handler] Failed to start stream for %s: %v", customerID, err)
		ws.WriteJSON(map[string]string{"type": "error", "message": err.Error()})
		return
	}

	// Ensure we stop monitoring when this handler exits (WS closes)
	defer h.service.StopMonitoring(customerID)

	// Listen for close messages from client
	go func() {
		for {
			if _, _, err := ws.ReadMessage(); err != nil {
				return
			}
		}
	}()

	// Stream data to WebSocket
	for data := range streamChan {
		err := ws.WriteJSON(gin.H{
			"type": "traffic_update",
			"data": data,
		})
		if err != nil {
			log.Printf("[Handler] WS Write error: %v", err)
			break
		}
	}
}

// GetPingHandler returns the ping handler for route registration
func (h *TrafficMonitorHandler) GetPingHandler() *PingHandler {
	return h.pingHandler
}

// ListCustomers has been moved to CustomerHandler, but we can implement a redirect or removal logic.
// Routes for ListCustomers are pointing to CustomerHandler now.
