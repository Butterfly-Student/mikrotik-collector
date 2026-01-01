package handlers

import (
	"fmt"
	"log"

	"mikrotik-collector/internal/domain"

	"github.com/gin-gonic/gin"
)

// CallbackHandler handles MikroTik callbacks
type CallbackHandler struct {
	repo      domain.CustomerRepository
	publisher domain.RedisPublisher
}

// NewCallbackHandler creates a new callback handler
func NewCallbackHandler(repo domain.CustomerRepository, publisher domain.RedisPublisher) *CallbackHandler {
	return &CallbackHandler{
		repo:      repo,
		publisher: publisher,
	}
}

// PPPoEUpRequest represents the payload for on-up callback
type PPPoEUpRequest struct {
	User       string `json:"user" binding:"required"`
	IPAddress  string `json:"ip"`
	Interface  string `json:"interface"`
	MacAddress string `json:"mac_address"`
}

// HandlePPPoEUp handles PPPoE on-up callback
// POST /api/callbacks/pppoe-up
func (h *CallbackHandler) HandlePPPoEUp(c *gin.Context) {
	var req PPPoEUpRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		log.Printf("[ERROR] JSON Bind Error: %v", err)
		c.JSON(400, gin.H{"status": "error", "message": err.Error()})
		return
	}

	log.Printf("[DEBUG] Looking for user: %s", req.User)

	targetCustomer, err := h.repo.GetCustomerByPPPoEUsername(req.User)
	if err != nil {
		log.Printf("[WARN] User not found: %s, Error: %v", req.User, err)
		c.JSON(200, gin.H{"status": "ignored", "message": "User not found in system"})
		return
	}

	log.Printf("[INFO] Found customer: ID=%s, Name=%s", targetCustomer.ID, targetCustomer.Name)

	err = h.repo.UpdateCustomerStatus(targetCustomer.ID, "active", &req.IPAddress, &req.MacAddress)
	if err != nil {
		c.JSON(500, gin.H{"status": "error", "message": err.Error()})
		return
	}

	log.Printf("Callback: Customer %s (%s) is now ONLINE", targetCustomer.Name, req.User)

	// Publish event to Redis
	eventData := fmt.Sprintf(`{"type":"pppoe_event","status":"connected","customer_id":"%s","name":"%s","ip":"%s","interface":"%s"}`,
		targetCustomer.ID, targetCustomer.Name, req.IPAddress, req.Interface)

	if err := h.publisher.Publish("mikrotik:events", eventData); err != nil {
		log.Printf("[WARN] Failed to publish Redis event: %v", err)
	}

	c.JSON(200, gin.H{"status": "success"})
}

// PPPoEDownRequest represents the payload for on-down callback
type PPPoEDownRequest struct {
	User string `json:"user" binding:"required"`
}

// HandlePPPoEDown handles PPPoE on-down callback
// POST /api/callbacks/pppoe-down
func (h *CallbackHandler) HandlePPPoEDown(c *gin.Context) {
	var req PPPoEDownRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		log.Printf("[ERROR] JSON Bind Error: %v", err)
		c.JSON(400, gin.H{"status": "error", "message": err.Error()})
		return
	}

	log.Printf("[DEBUG] Looking for user: %s", req.User)

	targetCustomer, err := h.repo.GetCustomerByPPPoEUsername(req.User)
	if err != nil {
		log.Printf("[WARN] User not found: %s, Error: %v", req.User, err)
		c.JSON(200, gin.H{"status": "ignored", "message": "User not found in system"})
		return
	}

	log.Printf("[INFO] Found customer: ID=%s, Name=%s", targetCustomer.ID, targetCustomer.Name)

	err = h.repo.UpdateCustomerStatus(targetCustomer.ID, "inactive", nil, nil)
	if err != nil {
		c.JSON(500, gin.H{"status": "error", "message": err.Error()})
		return
	}

	log.Printf("Callback: Customer %s (%s) is now OFFLINE", targetCustomer.Name, req.User)

	// Publish event to Redis
	eventData := fmt.Sprintf(`{"type":"pppoe_event","status":"disconnected","customer_id":"%s","name":"%s"}`,
		targetCustomer.ID, targetCustomer.Name)

	if err := h.publisher.Publish("mikrotik:events", eventData); err != nil {
		log.Printf("[WARN] Failed to publish Redis event: %v", err)
	}

	c.JSON(200, gin.H{"status": "success"})
}
