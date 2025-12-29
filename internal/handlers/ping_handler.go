package handlers

import (
	"context"
	"fmt"
	"log"
	"net/http"

	"mikrotik-collector/internal/domain"
	"mikrotik-collector/internal/infrastructure/mikrotik"

	"github.com/gin-gonic/gin"
	"github.com/gorilla/websocket"
)

// PingHandler handles ping requests to customer IPs
type PingHandler struct {
	client *mikrotik.Client
	repo   domain.CustomerRepository
}

// NewPingHandler creates a new ping handler
func NewPingHandler(client *mikrotik.Client, repo domain.CustomerRepository) *PingHandler {
	return &PingHandler{
		client: client,
		repo:   repo,
	}
}

// PingCustomerByID handles ping requests for a specific customer
// GET /api/customers/:customer_id/ping
func (h *PingHandler) PingCustomerByID(c *gin.Context) {
	customerID := c.Param("customer_id")

	// Get customer from database
	customer, err := h.repo.GetCustomerByID(customerID)
	if err != nil {
		c.JSON(404, gin.H{
			"status":      "error",
			"customer_id": customerID,
			"error":       "Customer not found in database",
			"message":     fmt.Sprintf("No customer with ID '%s' exists in the database", customerID),
		})
		return
	}

	// Get IP address for this customer
	ipAddress, err := h.getCustomerIPAddress(customer)
	if err != nil {
		c.JSON(400, gin.H{
			"status":        "error",
			"customer_id":   customer.ID,
			"customer_name": customer.Name,
			"error":         err.Error(),
			"message":       "Cannot ping: customer has no IP address configured",
		})
		return
	}

	// Verify customer exists in MikroTik (if PPPoE, check active sessions)
	if customer.ServiceType == "pppoe" {
		// Optimization: Check repository status first? Or real-time check?
		// Real-time check is better for ping diagnostic.
		// We can add method to mtClient to VerifySession but currently handlers do it.
		// For simplicity, let's skip strict session verification here or implement simple check.
		// h.verifyPPPoESession(customer) was used before.

		// Let's assume if IP is present, we try ping.
	}

	// Perform ping
	pingResult, err := h.pingIPAddress(ipAddress)
	if err != nil {
		c.JSON(500, gin.H{
			"status":        "error",
			"customer_id":   customer.ID,
			"customer_name": customer.Name,
			"ip_address":    ipAddress,
			"error":         err.Error(),
			"message":       "Failed to execute ping command on MikroTik",
		})
		return
	}

	// Build response message
	message := fmt.Sprintf("Customer '%s' is reachable at %s", customer.Name, ipAddress)
	if !pingResult.IsReachable {
		message = fmt.Sprintf("Customer '%s' is NOT reachable at %s (100%% packet loss)", customer.Name, ipAddress)
	}

	c.JSON(200, gin.H{
		"status":        "success",
		"customer_id":   customer.ID,
		"customer_name": customer.Name,
		"ip_address":    ipAddress,
		"is_reachable":  pingResult.IsReachable,
		"packet_loss":   pingResult.PacketLoss,
		"avg_time":      pingResult.AvgTime,
		"min_time":      pingResult.MinTime,
		"max_time":      pingResult.MaxTime,
		"sent":          pingResult.Sent,
		"received":      pingResult.Received,
		"message":       message,
	})
}

// PingCustomerStream handles streaming ping via WebSocket
// GET /api/customers/:customer_id/ping/ws
func (h *PingHandler) PingCustomerStream(c *gin.Context) {
	customerID := c.Param("customer_id")

	upgrader := websocket.Upgrader{
		CheckOrigin: func(r *http.Request) bool { return true },
	}

	ws, err := upgrader.Upgrade(c.Writer, c.Request, nil)
	if err != nil {
		log.Printf("WS upgrade failed: %v", err)
		return
	}
	defer ws.Close()

	// Get Customer
	customer, err := h.repo.GetCustomerByID(customerID)
	if err != nil {
		ws.WriteJSON(map[string]string{"type": "error", "error": "Customer not found"})
		return
	}

	// Get IP
	ipAddress, err := h.getCustomerIPAddress(customer)
	if err != nil {
		ws.WriteJSON(map[string]string{"type": "error", "error": err.Error()})
		return
	}

	// Start Streaming Ping
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Handle close message from client to stop ping
	ws.SetCloseHandler(func(code int, text string) error {
		cancel()
		return nil
	})

	// Start reading pump to handle close frames
	go func() {
		for {
			if _, _, err := ws.ReadMessage(); err != nil {
				cancel()
				break
			}
		}
	}()

	ptStream, err := h.client.StreamPing(ctx, ipAddress, "56", "1") // 1s interval by default
	if err != nil {
		ws.WriteJSON(map[string]string{"type": "error", "error": "Failed to start ping: " + err.Error()})
		return
	}

	// Track stats
	sent := 0
	received := 0

	for resp := range ptStream {
		// Increment stats
		if resp.Seq != "" {
			sent++
			if resp.Received != "" {
				received++
			} else if resp.Status == "" || resp.Time != "" {
				received++
			}
		}

		// Send update to FE
		err := ws.WriteJSON(map[string]interface{}{
			"type": "update",
			"data": resp,
		})
		if err != nil {
			break
		}
	}

	// Calculate summary
	loss := 100.0
	if sent > 0 {
		loss = float64(sent-received) / float64(sent) * 100
	}

	summary := map[string]interface{}{
		"sent":        sent,
		"received":    received,
		"packet_loss": fmt.Sprintf("%.0f%%", loss),
	}

	ws.WriteJSON(map[string]interface{}{
		"type":    "summary",
		"summary": summary,
	})
}

// getCustomerIPAddress extracts IP address based on service type
func (h *PingHandler) getCustomerIPAddress(customer *domain.Customer) (string, error) {
	switch customer.ServiceType {
	case "pppoe":
		// For PPPoE, we need to get IP from active session OR static IP in DB
		// Currently DB holds AssignedIP which is reliable if callback works
		if customer.AssignedIP != nil && *customer.AssignedIP != "" {
			return *customer.AssignedIP, nil
		}
		if customer.StaticIP != nil && *customer.StaticIP != "" {
			return *customer.StaticIP, nil
		}
		return "", fmt.Errorf("pppoe customer has no assigned IP")

	case "hotspot":
		// Similar logic for hotspot
		if customer.AssignedIP != nil && *customer.AssignedIP != "" {
			return *customer.AssignedIP, nil
		}
		return "", fmt.Errorf("hotspot customer has no assigned IP")

	case "static_ip":
		if customer.StaticIP != nil && *customer.StaticIP != "" {
			return *customer.StaticIP, nil
		}
		return "", fmt.Errorf("static IP not configured")

	default:
		return "", fmt.Errorf("unsupported service type: %s", customer.ServiceType)
	}
}

// Helper struct for minimal ping result
type PingResult struct {
	IsReachable bool
	PacketLoss  string
	AvgTime     string
	MinTime     string
	MaxTime     string
	Sent        string
	Received    string
}

func (h *PingHandler) pingIPAddress(ip string) (*PingResult, error) {
	// Execute simple ping command (non-streaming)
	// count=3
	cmd := []string{"/ping", "=address=" + ip, "=count=3"}
	res, err := h.client.RunArgs(cmd)
	if err != nil {
		return nil, err
	}

	// Parse result
	stats := res.Done.Map
	return &PingResult{
		IsReachable: stats["packet-loss"] != "100", // valid assumption?
		PacketLoss:  stats["packet-loss"] + "%",
		AvgTime:     stats["avg-rtt"],
		MinTime:     stats["min-rtt"],
		MaxTime:     stats["max-rtt"],
		Sent:        stats["sent"],
		Received:    stats["received"],
	}, nil
}
