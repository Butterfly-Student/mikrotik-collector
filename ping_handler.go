package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"strings"
	"time"

	"mikrotik-collector/internal/application/services"
	"mikrotik-collector/internal/infrastructure/mikrotik"

	"github.com/gorilla/websocket"
)

// PingHandler handles ping requests to customer IPs
type PingHandler struct {
	client *mikrotik.Client
	repo   services.CustomerRepository
}

// NewPingHandler creates a new ping handler
func NewPingHandler(
	client *mikrotik.Client,
	repo services.CustomerRepository,
) *PingHandler {
	return &PingHandler{
		client: client,
		repo:   repo,
	}
}

// PingResponse represents the response structure (Legacy)
type PingResponse struct {
	Status       string    `json:"status"`
	CustomerID   string    `json:"customer_id"`
	CustomerName string    `json:"customer_name"`
	IPAddress    string    `json:"ip_address"`
	IsReachable  bool      `json:"is_reachable"`
	PacketLoss   string    `json:"packet_loss,omitempty"`
	AvgTime      string    `json:"avg_time,omitempty"`
	MinTime      string    `json:"min_time,omitempty"`
	MaxTime      string    `json:"max_time,omitempty"`
	Sent         int       `json:"sent,omitempty"`
	Received     int       `json:"received,omitempty"`
	Error        string    `json:"error,omitempty"`
	Message      string    `json:"message,omitempty"`
	Timestamp    time.Time `json:"timestamp"`
}

// PingStreamResponse represents the WebSocket response
type PingStreamResponse struct {
	Type    string                `json:"type"` // "update" or "summary" or "error"
	Data    mikrotik.PingResponse `json:"data,omitempty"`
	Summary *PingSummary          `json:"summary,omitempty"`
	Error   string                `json:"error,omitempty"`
}

type PingSummary struct {
	Sent       int    `json:"sent"`
	Received   int    `json:"received"`
	PacketLoss string `json:"packet_loss"`
	MinRtt     string `json:"min_rtt"`
	AvgRtt     string `json:"avg_rtt"`
	MaxRtt     string `json:"max_rtt"`
}

// (Existing code...)

// PingCustomerStreamHandler handles streaming ping via WebSocket
// GET /api/customers/{customer_id}/ping/ws
func (h *PingHandler) PingCustomerStreamHandler(w http.ResponseWriter, r *http.Request, customerID string) {
	upgrader := websocket.Upgrader{
		CheckOrigin: func(r *http.Request) bool { return true },
	}

	ws, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Printf("WS upgrade failed: %v", err)
		return
	}
	defer ws.Close()

	// 1. Get Customer
	customer, err := h.repo.GetCustomerByID(customerID)
	if err != nil {
		ws.WriteJSON(map[string]string{"type": "error", "error": "Customer not found"})
		return
	}

	// 2. Get IP
	ipAddress, err := h.getCustomerIPAddress(customer)
	if err != nil {
		ws.WriteJSON(map[string]string{"type": "error", "error": err.Error()})
		return
	}

	// 3. Start Streaming Ping
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

	// Track stats manually
	sent := 0
	received := 0
	// We can track RTTs related logic if we want to calc avg manually,
	// but mostly we just want sent/received/loss.

	for resp := range ptStream {
		// Increment stats
		// Note: MikroTik sends 'sent' and 'received' in summary, but for individual packets we count them.
		// Usually a line has 'seq'.
		if resp.Seq != "" {
			sent++
			if resp.Received != "" {
				// If valid reply
				received++
			} else if resp.Status == "" || resp.Time != "" {
				// Sometimes successful ping just has time and host, status empty means OK
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

// PingCustomerHandler handles ping requests
// GET /api/customers/{customer_id}/ping
func (h *PingHandler) PingCustomerHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	if r.Method != "GET" {
		w.WriteHeader(http.StatusMethodNotAllowed)
		json.NewEncoder(w).Encode(map[string]interface{}{
			"status":  "error",
			"message": "Method not allowed",
		})
		return
	}

	// Extract customer ID from URL path
	// Format: /api/customers/{customer_id}/ping
	pathParts := strings.Split(strings.Trim(r.URL.Path, "/"), "/")
	if len(pathParts) < 3 {
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(map[string]interface{}{
			"status":  "error",
			"message": "Invalid URL format. Expected: /api/customers/{customer_id}/ping",
		})
		return
	}

	customerID := pathParts[2]

	// Get customer from database (source of truth)
	customer, err := h.repo.GetCustomerByID(customerID)
	if err != nil {
		w.WriteHeader(http.StatusNotFound)
		json.NewEncoder(w).Encode(PingResponse{
			Status:     "error",
			CustomerID: customerID,
			Error:      "Customer not found in database",
			Message:    fmt.Sprintf("No customer with ID '%s' exists in the database", customerID),
			Timestamp:  time.Now(),
		})
		return
	}

	// Get IP address for this customer
	ipAddress, err := h.getCustomerIPAddress(customer)
	if err != nil {
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(PingResponse{
			Status:       "error",
			CustomerID:   customer.ID,
			CustomerName: customer.Name,
			Error:        err.Error(),
			Message:      "Cannot ping: customer has no IP address configured",
			Timestamp:    time.Now(),
		})
		return
	}

	// Verify customer exists in MikroTik (if PPPoE, check active sessions)
	if customer.ServiceType == "pppoe" {
		exists, err := h.verifyPPPoESession(customer)
		if err != nil {
			w.WriteHeader(http.StatusInternalServerError)
			json.NewEncoder(w).Encode(PingResponse{
				Status:       "error",
				CustomerID:   customer.ID,
				CustomerName: customer.Name,
				IPAddress:    ipAddress,
				Error:        err.Error(),
				Message:      "Failed to verify PPPoE session in MikroTik",
				Timestamp:    time.Now(),
			})
			return
		}

		if !exists {
			w.WriteHeader(http.StatusNotFound)
			json.NewEncoder(w).Encode(PingResponse{
				Status:       "error",
				CustomerID:   customer.ID,
				CustomerName: customer.Name,
				IPAddress:    ipAddress,
				Error:        "PPPoE session not found",
				Message:      fmt.Sprintf("Customer '%s' exists in database but has no active PPPoE session in MikroTik", customer.Name),
				Timestamp:    time.Now(),
			})
			return
		}
	}

	// Perform ping
	pingResult, err := h.pingIPAddress(ipAddress)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(PingResponse{
			Status:       "error",
			CustomerID:   customer.ID,
			CustomerName: customer.Name,
			IPAddress:    ipAddress,
			Error:        err.Error(),
			Message:      "Failed to execute ping command on MikroTik",
			Timestamp:    time.Now(),
		})
		return
	}

	// Build response
	response := PingResponse{
		Status:       "success",
		CustomerID:   customer.ID,
		CustomerName: customer.Name,
		IPAddress:    ipAddress,
		IsReachable:  pingResult.IsReachable,
		PacketLoss:   pingResult.PacketLoss,
		AvgTime:      pingResult.AvgTime,
		MinTime:      pingResult.MinTime,
		MaxTime:      pingResult.MaxTime,
		Sent:         pingResult.Sent,
		Received:     pingResult.Received,
		Timestamp:    time.Now(),
	}

	if pingResult.IsReachable {
		response.Message = fmt.Sprintf("Customer '%s' is reachable at %s", customer.Name, ipAddress)
	} else {
		response.Message = fmt.Sprintf("Customer '%s' is NOT reachable at %s (100%% packet loss)", customer.Name, ipAddress)
	}

	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(response)
}

// getCustomerIPAddress extracts IP address based on service type
func (h *PingHandler) getCustomerIPAddress(customer *services.Customer) (string, error) {
	switch customer.ServiceType {
	case "pppoe":
		// For PPPoE, we need to get IP from active session
		if customer.PPPoEUsername == nil || *customer.PPPoEUsername == "" {
			return "", fmt.Errorf("PPPoE username not configured")
		}

		// Query active PPPoE sessions
		reply, err := h.client.Run(
			"/ppp/active/print",
			fmt.Sprintf("?name=%s", *customer.PPPoEUsername),
		)
		if err != nil {
			return "", fmt.Errorf("failed to query PPPoE sessions: %w", err)
		}

		if len(reply.Re) == 0 {
			return "", fmt.Errorf("no active PPPoE session found")
		}

		ipAddress := reply.Re[0].Map["address"]
		if ipAddress == "" {
			return "", fmt.Errorf("PPPoE session has no IP address")
		}

		return ipAddress, nil

	case "hotspot":
		// For hotspot, check assigned IP or active session
		if customer.AssignedIP != nil && *customer.AssignedIP != "" {
			return *customer.AssignedIP, nil
		}
		return "", fmt.Errorf("no IP address assigned for hotspot user")

	case "static_ip":
		// For static IP, use configured static IP
		if customer.StaticIP != nil && *customer.StaticIP != "" {
			return *customer.StaticIP, nil
		}
		return "", fmt.Errorf("static IP not configured")

	default:
		return "", fmt.Errorf("unsupported service type: %s", customer.ServiceType)
	}
}

// verifyPPPoESession verifies if PPPoE session exists in MikroTik
func (h *PingHandler) verifyPPPoESession(customer *services.Customer) (bool, error) {
	if customer.PPPoEUsername == nil || *customer.PPPoEUsername == "" {
		return false, fmt.Errorf("PPPoE username not configured")
	}

	reply, err := h.client.Run(
		"/ppp/active/print",
		fmt.Sprintf("?name=%s", *customer.PPPoEUsername),
	)
	if err != nil {
		return false, err
	}

	return len(reply.Re) > 0, nil
}

// PingResult represents ping command result
type PingResult struct {
	IsReachable bool
	PacketLoss  string
	AvgTime     string
	MinTime     string
	MaxTime     string
	Sent        int
	Received    int
}

// pingIPAddress performs ping to IP address via MikroTik
func (h *PingHandler) pingIPAddress(ipAddress string) (*PingResult, error) {
	// Execute ping command on MikroTik
	// /ping address=X.X.X.X count=4
	reply, err := h.client.Run(
		"/ping",
		fmt.Sprintf("=address=%s", ipAddress),
		"=count=4",
	)
	if err != nil {
		return nil, fmt.Errorf("ping command failed: %w", err)
	}

	if len(reply.Re) == 0 {
		return &PingResult{
			IsReachable: false,
			PacketLoss:  "100%",
			Sent:        4,
			Received:    0,
		}, nil
	}

	// Parse ping results
	result := &PingResult{
		Sent: 4,
	}

	// MikroTik ping returns multiple responses, we need the summary
	for _, re := range reply.Re {
		if sent := re.Map["sent"]; sent != "" {
			fmt.Sscanf(sent, "%d", &result.Sent)
		}
		if received := re.Map["received"]; received != "" {
			fmt.Sscanf(received, "%d", &result.Received)
		}
		if avgTime := re.Map["avg-rtt"]; avgTime != "" {
			result.AvgTime = avgTime
		}
		if minTime := re.Map["min-rtt"]; minTime != "" {
			result.MinTime = minTime
		}
		if maxTime := re.Map["max-rtt"]; maxTime != "" {
			result.MaxTime = maxTime
		}
		if packetLoss := re.Map["packet-loss"]; packetLoss != "" {
			result.PacketLoss = packetLoss
		}
	}

	result.IsReachable = result.Received > 0

	// Calculate packet loss if not provided
	if result.PacketLoss == "" && result.Sent > 0 {
		lossPercent := float64(result.Sent-result.Received) / float64(result.Sent) * 100
		result.PacketLoss = fmt.Sprintf("%.0f%%", lossPercent)
	}

	return result, nil
}

// PingCustomerByIDHandler is a simplified handler for direct route registration
func (h *PingHandler) PingCustomerByIDHandler(w http.ResponseWriter, r *http.Request, customerID string) {
	w.Header().Set("Content-Type", "application/json")

	if r.Method != "GET" {
		w.WriteHeader(http.StatusMethodNotAllowed)
		json.NewEncoder(w).Encode(map[string]interface{}{
			"status":  "error",
			"message": "Method not allowed",
		})
		return
	}

	// Get customer from database (source of truth)
	customer, err := h.repo.GetCustomerByID(customerID)
	if err != nil {
		w.WriteHeader(http.StatusNotFound)
		json.NewEncoder(w).Encode(PingResponse{
			Status:     "error",
			CustomerID: customerID,
			Error:      "Customer not found in database",
			Message:    fmt.Sprintf("No customer with ID '%s' exists in the database", customerID),
			Timestamp:  time.Now(),
		})
		return
	}

	// Get IP address for this customer
	ipAddress, err := h.getCustomerIPAddress(customer)
	if err != nil {
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(PingResponse{
			Status:       "error",
			CustomerID:   customer.ID,
			CustomerName: customer.Name,
			Error:        err.Error(),
			Message:      "Cannot ping: customer has no IP address configured",
			Timestamp:    time.Now(),
		})
		return
	}

	// Verify customer exists in MikroTik (if PPPoE, check active sessions)
	if customer.ServiceType == "pppoe" {
		exists, err := h.verifyPPPoESession(customer)
		if err != nil {
			w.WriteHeader(http.StatusInternalServerError)
			json.NewEncoder(w).Encode(PingResponse{
				Status:       "error",
				CustomerID:   customer.ID,
				CustomerName: customer.Name,
				IPAddress:    ipAddress,
				Error:        err.Error(),
				Message:      "Failed to verify PPPoE session in MikroTik",
				Timestamp:    time.Now(),
			})
			return
		}

		if !exists {
			w.WriteHeader(http.StatusNotFound)
			json.NewEncoder(w).Encode(PingResponse{
				Status:       "error",
				CustomerID:   customer.ID,
				CustomerName: customer.Name,
				IPAddress:    ipAddress,
				Error:        "PPPoE session not found",
				Message:      fmt.Sprintf("Customer '%s' exists in database but has no active PPPoE session in MikroTik", customer.Name),
				Timestamp:    time.Now(),
			})
			return
		}
	}

	// Perform ping
	pingResult, err := h.pingIPAddress(ipAddress)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(PingResponse{
			Status:       "error",
			CustomerID:   customer.ID,
			CustomerName: customer.Name,
			IPAddress:    ipAddress,
			Error:        err.Error(),
			Message:      "Failed to execute ping command on MikroTik",
			Timestamp:    time.Now(),
		})
		return
	}

	// Build response
	response := PingResponse{
		Status:       "success",
		CustomerID:   customer.ID,
		CustomerName: customer.Name,
		IPAddress:    ipAddress,
		IsReachable:  pingResult.IsReachable,
		PacketLoss:   pingResult.PacketLoss,
		AvgTime:      pingResult.AvgTime,
		MinTime:      pingResult.MinTime,
		MaxTime:      pingResult.MaxTime,
		Sent:         pingResult.Sent,
		Received:     pingResult.Received,
		Timestamp:    time.Now(),
	}

	if pingResult.IsReachable {
		response.Message = fmt.Sprintf("Customer '%s' is reachable at %s", customer.Name, ipAddress)
	} else {
		response.Message = fmt.Sprintf("Customer '%s' is NOT reachable at %s (100%% packet loss)", customer.Name, ipAddress)
	}

	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(response)
}
