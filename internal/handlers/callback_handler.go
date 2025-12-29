package handlers

import (
	"log"

	"mikrotik-collector/internal/domain"

	"github.com/gin-gonic/gin"
)

// CallbackHandler handles MikroTik callbacks
type CallbackHandler struct {
	repo domain.CustomerRepository
}

// NewCallbackHandler creates a new callback handler
func NewCallbackHandler(repo domain.CustomerRepository) *CallbackHandler {
	return &CallbackHandler{
		repo: repo,
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
		c.JSON(400, gin.H{"status": "error", "message": err.Error()})
		return
	}

	// Find customer by PPPoE Username
	// We added GetCustomerByPPPoEUsername to repository, so we should use it!
	targetCustomer, err := h.repo.GetCustomerByPPPoEUsername(req.User)
	if err != nil {
		// Log warning but return success to not break MikroTik script
		log.Printf("Callback: Unknown PPPoE user %s connected: %v", req.User, err)
		c.JSON(200, gin.H{"status": "ignored", "message": "User not found in system"})
		return
	}

	err = h.repo.UpdateCustomerStatus(targetCustomer.ID, "active", &req.IPAddress, &req.MacAddress)
	if err != nil {
		c.JSON(500, gin.H{"status": "error", "message": err.Error()})
		return
	}

	log.Printf("Callback: Customer %s (%s) is now ONLINE", targetCustomer.Name, req.User)
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
		c.JSON(400, gin.H{"status": "error", "message": err.Error()})
		return
	}

	// Find customer
	targetCustomer, err := h.repo.GetCustomerByPPPoEUsername(req.User)
	if err != nil {
		c.JSON(200, gin.H{"status": "ignored", "message": "User not found"})
		return
	}

	err = h.repo.UpdateCustomerStatus(targetCustomer.ID, "offline", nil, nil)
	if err != nil {
		c.JSON(500, gin.H{"status": "error", "message": err.Error()})
		return
	}

	log.Printf("Callback: Customer %s (%s) is now OFFLINE", targetCustomer.Name, req.User)
	c.JSON(200, gin.H{"status": "success"})
}
