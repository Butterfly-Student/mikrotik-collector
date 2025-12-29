package main

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"strings"

	"mikrotik-collector/internal/application/services"
	"mikrotik-collector/internal/infrastructure/mikrotik"
)

// TrafficMonitorHandler handles HTTP requests for traffic monitoring
type TrafficMonitorHandler struct {
	service     *services.ContinuousTrafficService
	repo        services.CustomerRepository
	pingHandler *PingHandler
}

// NewTrafficMonitorHandler creates a new handler
func NewTrafficMonitorHandler(
	service *services.ContinuousTrafficService,
	repo services.CustomerRepository,
	mtClient *mikrotik.Client,
) *TrafficMonitorHandler {
	return &TrafficMonitorHandler{
		service:     service,
		repo:        repo,
		pingHandler: NewPingHandler(mtClient, repo),
	}
}

// CustomerListResponse represents a simplified customer for frontend
type CustomerListResponse struct {
	ID            string `json:"id"`
	Name          string `json:"name"`
	Username      string `json:"username"`
	ServiceType   string `json:"service_type"`
	PPPoEUsername string `json:"pppoe_username,omitempty"`
	Status        string `json:"status"`
	IsMonitoring  bool   `json:"is_monitoring"`
}

// ListCustomersHandler returns list of active PPPoE customers
// GET /api/customers
func (h *TrafficMonitorHandler) ListCustomersHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	if r.Method != "GET" {
		w.WriteHeader(http.StatusMethodNotAllowed)
		json.NewEncoder(w).Encode(map[string]interface{}{
			"status":  "error",
			"message": "Method not allowed",
		})
		return
	}

	customers, err := h.repo.GetActivePPPoECustomers()
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(map[string]interface{}{
			"status":  "error",
			"message": err.Error(),
		})
		return
	}

	// Get active interfaces
	activeInterfaces := h.service.GetActiveInterfaces()
	activeInterfaceMap := make(map[string]bool)
	for _, iface := range activeInterfaces {
		activeInterfaceMap[iface] = true
	}

	var response []CustomerListResponse
	for _, c := range customers {
		pppoeUsername := ""
		if c.PPPoEUsername != nil {
			pppoeUsername = *c.PPPoEUsername
		}

		// Check if customer is currently being monitored
		interfaceName := fmt.Sprintf("<pppoe-%s>", pppoeUsername)
		isMonitoring := activeInterfaceMap[interfaceName]

		response = append(response, CustomerListResponse{
			ID:            c.ID,
			Name:          c.Name,
			Username:      c.Username,
			ServiceType:   c.ServiceType,
			PPPoEUsername: pppoeUsername,
			Status:        "active",
			IsMonitoring:  isMonitoring,
		})
	}

	json.NewEncoder(w).Encode(map[string]interface{}{
		"status":    "success",
		"customers": response,
		"count":     len(response),
	})
}

// StatusHandler returns monitoring status
// GET /api/monitor/status
func (h *TrafficMonitorHandler) StatusHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	if r.Method != "GET" {
		w.WriteHeader(http.StatusMethodNotAllowed)
		json.NewEncoder(w).Encode(map[string]interface{}{
			"status":  "error",
			"message": "Method not allowed",
		})
		return
	}

	activeInterfaces := h.service.GetActiveInterfaces()
	monitorCount := h.service.GetMonitorCount()
	customerCount := h.service.GetCustomerCount()

	json.NewEncoder(w).Encode(map[string]interface{}{
		"status":            "ok",
		"active_interfaces": activeInterfaces,
		"monitor_count":     monitorCount,
		"customer_count":    customerCount,
		"mode":              "continuous",
	})
}

// ReloadCustomersHandler reloads customers from database
// POST /api/reload-customers
func (h *TrafficMonitorHandler) ReloadCustomersHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	if r.Method != "POST" {
		w.WriteHeader(http.StatusMethodNotAllowed)
		json.NewEncoder(w).Encode(map[string]interface{}{
			"status":  "error",
			"message": "Method not allowed",
		})
		return
	}

	if err := h.service.ReloadCustomers(); err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(map[string]interface{}{
			"status":  "error",
			"message": err.Error(),
		})
		return
	}

	json.NewEncoder(w).Encode(map[string]interface{}{
		"status":  "success",
		"message": "Customers reloaded successfully",
	})
}

// CustomersPingHandler handles ping requests to specific customer
// GET /api/customers/{customer_id}/ping
func (h *TrafficMonitorHandler) CustomersPingHandler(w http.ResponseWriter, r *http.Request) {
	// Extract customer ID from path
	// Path format: /api/customers/{customer_id}/ping
	path := strings.TrimPrefix(r.URL.Path, "/api/customers/")
	path = strings.TrimSuffix(path, "/ping")

	customerID := path

	if customerID == "" {
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(map[string]interface{}{
			"status":  "error",
			"message": "Customer ID is required",
		})
		return
	}

	// Delegate to ping handler
	h.pingHandler.PingCustomerByIDHandler(w, r, customerID)
}

// CustomersPingStreamHandler handles ping streaming requests
// GET /api/customers/{customer_id}/ping/ws
func (h *TrafficMonitorHandler) CustomersPingStreamHandler(w http.ResponseWriter, r *http.Request) {
	// Extract customer ID from path
	// Path format: /api/customers/{customer_id}/ping/ws
	path := strings.TrimPrefix(r.URL.Path, "/api/customers/")
	path = strings.TrimSuffix(path, "/ping/ws")

	customerID := path

	if customerID == "" {
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(map[string]interface{}{
			"status":  "error",
			"message": "Customer ID is required",
		})
		return
	}

	// Delegate to ping handler
	h.pingHandler.PingCustomerStreamHandler(w, r, customerID)
}

// RegisterRoutes registers all traffic monitor routes to the given mux
func (h *TrafficMonitorHandler) RegisterRoutes(mux *http.ServeMux) {
	// Customer list
	mux.HandleFunc("/api/customers/", func(w http.ResponseWriter, r *http.Request) {
		// List customers (handle both with and without trailing slash)
		if r.URL.Path == "/api/customers" || r.URL.Path == "/api/customers/" {
			h.ListCustomersHandler(w, r)
			return
		}

		// Match /api/customers/{id}/ping/ws
		if strings.HasSuffix(r.URL.Path, "/ping/ws") && strings.HasPrefix(r.URL.Path, "/api/customers/") {
			h.CustomersPingStreamHandler(w, r)
			return
		}

		// Match /api/customers/{id}/ping
		if strings.HasSuffix(r.URL.Path, "/ping") && strings.HasPrefix(r.URL.Path, "/api/customers/") {
			h.CustomersPingHandler(w, r)
			return
		}

		// 404 for other paths
		w.WriteHeader(http.StatusNotFound)
		json.NewEncoder(w).Encode(map[string]interface{}{
			"status":  "error",
			"message": "Not found",
		})
	})

	// Monitor status
	mux.HandleFunc("/api/monitor/status", h.StatusHandler)

	// Reload customers
	mux.HandleFunc("/api/reload-customers", h.ReloadCustomersHandler)

	log.Println("Traffic monitor API routes registered:")
	log.Println("  GET  /api/customers")
	log.Println("  GET  /api/customers/{customer_id}/ping")
	log.Println("  GET  /api/monitor/status")
	log.Println("  POST /api/reload-customers")
}
