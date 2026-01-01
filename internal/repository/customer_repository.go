package repository

import (
	"fmt"
	"log"
	"time"

	"mikrotik-collector/internal/domain"

	"gorm.io/gorm"
)

// DatabaseCustomerRepository implements domain.CustomerRepository
type DatabaseCustomerRepository struct {
	db *gorm.DB
}

// NewDatabaseCustomerRepository creates a new database customer repository
func NewDatabaseCustomerRepository(db *gorm.DB) *DatabaseCustomerRepository {
	return &DatabaseCustomerRepository{
		db: db,
	}
}

// GetActivePPPoECustomers retrieves all active PPPoE customers
func (r *DatabaseCustomerRepository) GetActivePPPoECustomers() ([]*domain.Customer, error) {
	log.Println("[CustomerRepo] GetActivePPPoECustomers - Starting query for active PPPoE customers")
	
	var customers []*domain.Customer
	
	err := r.db.Where("status = ? AND service_type = ?", "active", "pppoe").
		Order("name").
		Find(&customers).Error
	
	if err != nil {
		log.Printf("[CustomerRepo] GetActivePPPoECustomers - ERROR: %v\n", err)
		return nil, fmt.Errorf("failed to query customers: %w", err)
	}
	
	log.Printf("[CustomerRepo] GetActivePPPoECustomers - SUCCESS: Found %d active PPPoE customers\n", len(customers))
	return customers, nil
}

// GetCustomerByID retrieves a customer by ID
func (r *DatabaseCustomerRepository) GetCustomerByID(id string) (*domain.Customer, error) {
	log.Printf("[CustomerRepo] GetCustomerByID - Searching for customer with ID: %s\n", id)
	
	var customer domain.Customer
	
	err := r.db.Where("id = ?", id).First(&customer).Error
	if err == gorm.ErrRecordNotFound {
		log.Printf("[CustomerRepo] GetCustomerByID - Customer not found: %s\n", id)
		return nil, fmt.Errorf("customer not found: %s", id)
	}
	if err != nil {
		log.Printf("[CustomerRepo] GetCustomerByID - ERROR: %v\n", err)
		return nil, fmt.Errorf("failed to query customer: %w", err)
	}
	
	log.Printf("[CustomerRepo] GetCustomerByID - SUCCESS: Found customer %s (%s)\n", customer.Name, customer.ID)
	return &customer, nil
}

// GetCustomerByPPPoEUsername retrieves a customer by PPPoE Username
func (r *DatabaseCustomerRepository) GetCustomerByPPPoEUsername(username string) (*domain.Customer, error) {
	log.Printf("[CustomerRepo] GetCustomerByPPPoEUsername - Searching for customer with PPPoE username: %s\n", username)
	
	var customer domain.Customer
	
	err := r.db.Where("pppoe_username = ?", username).First(&customer).Error
	if err == gorm.ErrRecordNotFound {
		log.Printf("[CustomerRepo] GetCustomerByPPPoEUsername - Customer not found with PPPoE username: %s\n", username)
		return nil, fmt.Errorf("customer not found with pppoe_username: %s", username)
	}
	if err != nil {
		log.Printf("[CustomerRepo] GetCustomerByPPPoEUsername - ERROR: %v\n", err)
		return nil, fmt.Errorf("failed to query customer: %w", err)
	}
	
	log.Printf("[CustomerRepo] GetCustomerByPPPoEUsername - SUCCESS: Found customer %s (ID: %s)\n", customer.Name, customer.ID)
	return &customer, nil
}

// UpdateCustomerStatus updates status of a customer
func (r *DatabaseCustomerRepository) UpdateCustomerStatus(id string, status string, ipAddress *string, macAddress *string) error {
	log.Printf("[CustomerRepo] UpdateCustomerStatus - Updating customer %s to status: %s\n", id, status)
	
	updates := map[string]interface{}{
		"status":     status,
		"updated_at": time.Now(),
	}
	
	if ipAddress != nil {
		log.Printf("[CustomerRepo] UpdateCustomerStatus - Setting IP address: %s\n", *ipAddress)
		updates["assigned_ip"] = *ipAddress
	}
	
	if macAddress != nil {
		log.Printf("[CustomerRepo] UpdateCustomerStatus - Setting MAC address: %s\n", *macAddress)
		updates["mac_address"] = *macAddress
	}
	
	if status == "active" {
		log.Println("[CustomerRepo] UpdateCustomerStatus - Updating last_online timestamp")
		updates["last_online"] = time.Now()
	}
	
	result := r.db.Model(&domain.Customer{}).
		Where("id = ?", id).
		Updates(updates)
	
	if result.Error != nil {
		log.Printf("[CustomerRepo] UpdateCustomerStatus - ERROR: %v\n", result.Error)
		return fmt.Errorf("failed to update customer status: %w", result.Error)
	}
	
	if result.RowsAffected == 0 {
		log.Printf("[CustomerRepo] UpdateCustomerStatus - Customer not found: %s\n", id)
		return fmt.Errorf("customer not found: %s", id)
	}
	
	log.Printf("[CustomerRepo] UpdateCustomerStatus - SUCCESS: Updated customer %s (rows affected: %d)\n", id, result.RowsAffected)
	return nil
}

// CreateCustomer creates a new customer
func (r *DatabaseCustomerRepository) CreateCustomer(c *domain.Customer) error {
	log.Printf("[CustomerRepo] CreateCustomer - Creating new customer: %s (ID: %s)\n", c.Name, c.ID)
	
	if c.CreatedAt.IsZero() {
		c.CreatedAt = time.Now()
	}
	c.UpdatedAt = time.Now()
	
	log.Printf("[CustomerRepo] CreateCustomer - Customer details - ServiceType: %s, Status: %s, PPPoE Username: %s\n", 
		c.ServiceType, c.Status, c.PPPoEUsername)
	
	err := r.db.Create(c).Error
	if err != nil {
		log.Printf("[CustomerRepo] CreateCustomer - ERROR: %v\n", err)
		return fmt.Errorf("failed to create customer: %w", err)
	}
	
	log.Printf("[CustomerRepo] CreateCustomer - SUCCESS: Created customer %s (ID: %s)\n", c.Name, c.ID)
	return nil
}

// UpdateCustomer updates an existing customer
func (r *DatabaseCustomerRepository) UpdateCustomer(c *domain.Customer) error {
	log.Printf("[CustomerRepo] UpdateCustomer - Updating customer: %s (ID: %s)\n", c.Name, c.ID)
	
	c.UpdatedAt = time.Now()
	
	log.Printf("[CustomerRepo] UpdateCustomer - Customer details - ServiceType: %s, Status: %s, PPPoE Username: %s\n", 
		c.ServiceType, c.Status, c.PPPoEUsername)
	
	result := r.db.Model(&domain.Customer{}).
		Where("id = ?", c.ID).
		Updates(c)
	
	if result.Error != nil {
		log.Printf("[CustomerRepo] UpdateCustomer - ERROR: %v\n", result.Error)
		return fmt.Errorf("failed to update customer: %w", result.Error)
	}
	
	if result.RowsAffected == 0 {
		log.Printf("[CustomerRepo] UpdateCustomer - Customer not found: %s\n", c.ID)
		return fmt.Errorf("customer not found: %s", c.ID)
	}
	
	log.Printf("[CustomerRepo] UpdateCustomer - SUCCESS: Updated customer %s (rows affected: %d)\n", c.ID, result.RowsAffected)
	return nil
}

// DeleteCustomer deletes a customer
func (r *DatabaseCustomerRepository) DeleteCustomer(id string) error {
	result := r.db.Where("id = ?", id).Delete(&domain.Customer{})
	
	if result.Error != nil {
		return fmt.Errorf("failed to delete customer: %w", result.Error)
	}
	
	if result.RowsAffected == 0 {
		return fmt.Errorf("customer not found: %s", id)
	}
	
	return nil
}

// ListCustomers returns paginated customers
func (r *DatabaseCustomerRepository) ListCustomers(page, limit int) ([]*domain.Customer, int, error) {
	var customers []*domain.Customer
	var total int64
	
	// Count total
	err := r.db.Model(&domain.Customer{}).Count(&total).Error
	if err != nil {
		return nil, 0, fmt.Errorf("failed to count customers: %w", err)
	}
	
	offset := (page - 1) * limit
	
	err = r.db.Order("created_at DESC").
		Limit(limit).
		Offset(offset).
		Find(&customers).Error
	
	if err != nil {
		return nil, 0, fmt.Errorf("failed to query customers: %w", err)
	}
	
	return customers, int(total), nil
}