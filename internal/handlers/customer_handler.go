package handlers

import (
	"log"
	"strconv"

	"mikrotik-collector/internal/application/services"
	"mikrotik-collector/internal/domain"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

// CustomerHandler handles CRUD requests for customers
type CustomerHandler struct {
	service *services.CustomerService
}

// NewCustomerHandler creates a new customer handler
func NewCustomerHandler(service *services.CustomerService) *CustomerHandler {
	return &CustomerHandler{
		service: service,
	}
}

// CreateCustomerRequest represents payload for creating customer
type CreateCustomerRequest struct {
	Name        string `json:"name" binding:"required"`
	Username    string `json:"username" binding:"required"`     // App username
	ServiceType string `json:"service_type" binding:"required"` // pppoe, hotspot

	PPPoEUsername *string `json:"pppoe_username"`
	PPPoEPassword *string `json:"pppoe_password"`
	PPPoEProfile  *string `json:"pppoe_profile"`

	Phone *string `json:"phone"`
	Email *string `json:"email"`
}

// CreateCustomer handles customer creation
// POST /api/customers
func (h *CustomerHandler) CreateCustomer(c *gin.Context) {
	var req CreateCustomerRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(400, gin.H{"status": "error", "message": err.Error()})
		return
	}

	newID := uuid.New().String()

	customer := &domain.Customer{
		ID:            newID,
		Name:          req.Name,
		Username:      req.Username,
		ServiceType:   req.ServiceType,
		PPPoEUsername: req.PPPoEUsername,
		PPPoEPassword: req.PPPoEPassword,
		PPPoEProfile:  req.PPPoEProfile,
		Phone:         req.Phone,
		Email:         req.Email,
		Status:        "active", // Default status
	}

	if err := h.service.CreateCustomer(customer); err != nil {
		log.Printf("Failed to create customer: %v", err)
		c.JSON(500, gin.H{"status": "error", "message": err.Error()})
		return
	}

	c.JSON(201, gin.H{
		"status": "success",
		"data":   customer,
	})
}

// UpdateCustomer handles customer update
// PUT /api/customers/:id
func (h *CustomerHandler) UpdateCustomer(c *gin.Context) {
	id := c.Param("id")

	var req CreateCustomerRequest // Reuse struct or create Update struct
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(400, gin.H{"status": "error", "message": err.Error()})
		return
	}

	// Map request to domain.Customer
	// Note: In real app, we should fetch first to merge, but Service handles full update currently
	customer := &domain.Customer{
		ID:            id,
		Name:          req.Name,
		Username:      req.Username,
		ServiceType:   req.ServiceType,
		PPPoEUsername: req.PPPoEUsername,
		PPPoEPassword: req.PPPoEPassword,
		PPPoEProfile:  req.PPPoEProfile,
		Phone:         req.Phone,
		Email:         req.Email,
	}

	if err := h.service.UpdateCustomer(customer); err != nil {
		c.JSON(500, gin.H{"status": "error", "message": err.Error()})
		return
	}

	c.JSON(200, gin.H{"status": "success"})
}

// DeleteCustomer handles customer deletion
// DELETE /api/customers/:id
func (h *CustomerHandler) DeleteCustomer(c *gin.Context) {
	id := c.Param("id")

	if err := h.service.DeleteCustomer(id); err != nil {
		c.JSON(500, gin.H{"status": "error", "message": err.Error()})
		return
	}

	c.JSON(200, gin.H{"status": "success"})
}

// GetCustomer handles getting single customer
// GET /api/customers/:id
func (h *CustomerHandler) GetCustomer(c *gin.Context) {
	id := c.Param("id")

	customer, err := h.service.GetCustomer(id)
	if err != nil {
		c.JSON(404, gin.H{"status": "error", "message": err.Error()})
		return
	}

	c.JSON(200, gin.H{"status": "success", "data": customer})
}

// ListCustomers handles listing customers with pagination
// GET /api/customers
func (h *CustomerHandler) ListCustomers(c *gin.Context) {
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "20"))

	customers, total, err := h.service.ListCustomers(page, limit)
	if err != nil {
		c.JSON(500, gin.H{"status": "error", "message": err.Error()})
		return
	}

	c.JSON(200, gin.H{
		"status": "success",
		"data":   customers,
		"meta": gin.H{
			"page":  page,
			"limit": limit,
			"total": total,
		},
	})
}
