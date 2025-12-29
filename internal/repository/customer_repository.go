package repository

import (
	"database/sql"
	"fmt"
	"time"

	"mikrotik-collector/internal/domain"

	_ "github.com/lib/pq"
)

// DatabaseCustomerRepository implements domain.CustomerRepository
type DatabaseCustomerRepository struct {
	db *sql.DB
}

// NewDatabaseCustomerRepository creates a new database customer repository
func NewDatabaseCustomerRepository(db *sql.DB) *DatabaseCustomerRepository {
	return &DatabaseCustomerRepository{
		db: db,
	}
}

// GetActivePPPoECustomers retrieves all active PPPoE customers
func (r *DatabaseCustomerRepository) GetActivePPPoECustomers() ([]*domain.Customer, error) {
	query := `
		SELECT 
			id, mikrotik_id, username, name, phone, email, service_type,
			pppoe_username, pppoe_password, pppoe_profile,
			hotspot_username, hotspot_password, hotspot_mac_address,
			static_ip, assigned_ip, mac_address, last_online,
			status, created_at, updated_at
		FROM customers
		WHERE status = 'active' AND service_type = 'pppoe'
		ORDER BY name
	`

	rows, err := r.db.Query(query)
	if err != nil {
		return nil, fmt.Errorf("failed to query customers: %w", err)
	}
	defer rows.Close()

	var customers []*domain.Customer
	for rows.Next() {
		var c domain.Customer
		err := rows.Scan(
			&c.ID, &c.MikrotikID, &c.Username, &c.Name,
			&c.Phone, &c.Email, &c.ServiceType,
			&c.PPPoEUsername, &c.PPPoEPassword, &c.PPPoEProfile,
			&c.HotspotUsername, &c.HotspotPassword, &c.HotspotMacAddr,
			&c.StaticIP, &c.AssignedIP, &c.MacAddress, &c.LastOnline,
			&c.Status, &c.CreatedAt, &c.UpdatedAt,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan customer: %w", err)
		}
		customers = append(customers, &c)
	}
	return customers, nil
}

// GetCustomerByID retrieves a customer by ID
func (r *DatabaseCustomerRepository) GetCustomerByID(id string) (*domain.Customer, error) {
	query := `
		SELECT 
			id, mikrotik_id, username, name, phone, email, service_type,
			pppoe_username, pppoe_password, pppoe_profile,
			hotspot_username, hotspot_password, hotspot_mac_address,
			static_ip, assigned_ip, mac_address, last_online,
			status, created_at, updated_at
		FROM customers
		WHERE id = $1
	`
	c := &domain.Customer{}
	err := r.db.QueryRow(query, id).Scan(
		&c.ID, &c.MikrotikID, &c.Username, &c.Name,
		&c.Phone, &c.Email, &c.ServiceType,
		&c.PPPoEUsername, &c.PPPoEPassword, &c.PPPoEProfile,
		&c.HotspotUsername, &c.HotspotPassword, &c.HotspotMacAddr,
		&c.StaticIP, &c.AssignedIP, &c.MacAddress, &c.LastOnline,
		&c.Status, &c.CreatedAt, &c.UpdatedAt,
	)
	if err == sql.ErrNoRows {
		return nil, fmt.Errorf("customer not found: %s", id)
	}
	if err != nil {
		return nil, fmt.Errorf("failed to query customer: %w", err)
	}
	return c, nil
}

// GetCustomerByPPPoEUsername retrieves a customer by PPPoE Username
func (r *DatabaseCustomerRepository) GetCustomerByPPPoEUsername(username string) (*domain.Customer, error) {
	query := `
		SELECT 
			id, mikrotik_id, username, name, phone, email, service_type,
			pppoe_username, pppoe_password, pppoe_profile,
			hotspot_username, hotspot_password, hotspot_mac_address,
			static_ip, assigned_ip, mac_address, last_online,
			status, created_at, updated_at
		FROM customers
		WHERE pppoe_username = $1
	`
	c := &domain.Customer{}
	err := r.db.QueryRow(query, username).Scan(
		&c.ID, &c.MikrotikID, &c.Username, &c.Name,
		&c.Phone, &c.Email, &c.ServiceType,
		&c.PPPoEUsername, &c.PPPoEPassword, &c.PPPoEProfile,
		&c.HotspotUsername, &c.HotspotPassword, &c.HotspotMacAddr,
		&c.StaticIP, &c.AssignedIP, &c.MacAddress, &c.LastOnline,
		&c.Status, &c.CreatedAt, &c.UpdatedAt,
	)
	if err == sql.ErrNoRows {
		return nil, fmt.Errorf("customer not found with pppoe_username: %s", username)
	}
	if err != nil {
		return nil, fmt.Errorf("failed to query customer: %w", err)
	}
	return c, nil
}

// UpdateCustomerStatus updates status of a customer
func (r *DatabaseCustomerRepository) UpdateCustomerStatus(id string, status string, ipAddress *string, macAddress *string) error {
	query := `
		UPDATE customers
		SET 
			status = $2,
			assigned_ip = COALESCE($3, assigned_ip),
			mac_address = COALESCE($4, mac_address),
			last_online = CASE WHEN $2 = 'active' THEN CURRENT_TIMESTAMP ELSE last_online END,
			updated_at = CURRENT_TIMESTAMP
		WHERE id = $1
	`
	result, err := r.db.Exec(query, id, status, ipAddress, macAddress)
	if err != nil {
		return fmt.Errorf("failed to update customer status: %w", err)
	}
	rows, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("failed to check rows affected: %w", err)
	}
	if rows == 0 {
		return fmt.Errorf("customer not found: %s", id)
	}
	return nil
}

// CreateCustomer creates a new customer
func (r *DatabaseCustomerRepository) CreateCustomer(c *domain.Customer) error {
	query := `
		INSERT INTO customers (
			id, mikrotik_id, username, name, phone, email, service_type,
			pppoe_username, pppoe_password, pppoe_profile,
			hotspot_username, hotspot_password, hotspot_mac_address,
			static_ip, assigned_ip, mac_address, status, created_at, updated_at
		) VALUES (
			$1, $2, $3, $4, $5, $6, $7,
			$8, $9, $10,
			$11, $12, $13,
			$14, $15, $16, $17, $18, $19
		)
	`
	if c.CreatedAt.IsZero() {
		c.CreatedAt = time.Now()
	}
	c.UpdatedAt = time.Now()

	_, err := r.db.Exec(query,
		c.ID, c.MikrotikID, c.Username, c.Name, c.Phone, c.Email, c.ServiceType,
		c.PPPoEUsername, c.PPPoEPassword, c.PPPoEProfile,
		c.HotspotUsername, c.HotspotPassword, c.HotspotMacAddr,
		c.StaticIP, c.AssignedIP, c.MacAddress, c.Status, c.CreatedAt, c.UpdatedAt,
	)
	if err != nil {
		return fmt.Errorf("failed to create customer: %w", err)
	}
	return nil
}

// UpdateCustomer updates an existing customer
func (r *DatabaseCustomerRepository) UpdateCustomer(c *domain.Customer) error {
	query := `
		UPDATE customers SET
			mikrotik_id = $2, username = $3, name = $4, phone = $5, email = $6, service_type = $7,
			pppoe_username = $8, pppoe_password = $9, pppoe_profile = $10,
			hotspot_username = $11, hotspot_password = $12, hotspot_mac_address = $13,
			static_ip = $14, assigned_ip = $15, mac_address = $16, status = $17, updated_at = $18
		WHERE id = $1
	`
	c.UpdatedAt = time.Now()

	result, err := r.db.Exec(query,
		c.ID, c.MikrotikID, c.Username, c.Name, c.Phone, c.Email, c.ServiceType,
		c.PPPoEUsername, c.PPPoEPassword, c.PPPoEProfile,
		c.HotspotUsername, c.HotspotPassword, c.HotspotMacAddr,
		c.StaticIP, c.AssignedIP, c.MacAddress, c.Status, c.UpdatedAt,
	)
	if err != nil {
		return fmt.Errorf("failed to update customer: %w", err)
	}
	rows, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("failed to check rows affected: %w", err)
	}
	if rows == 0 {
		return fmt.Errorf("customer not found: %s", c.ID)
	}
	return nil
}

// DeleteCustomer deletes a customer
func (r *DatabaseCustomerRepository) DeleteCustomer(id string) error {
	query := `DELETE FROM customers WHERE id = $1`
	result, err := r.db.Exec(query, id)
	if err != nil {
		return fmt.Errorf("failed to delete customer: %w", err)
	}
	rows, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("failed to check rows affected: %w", err)
	}
	if rows == 0 {
		return fmt.Errorf("customer not found: %s", id)
	}
	return nil
}

// ListCustomers returns paginated customers
func (r *DatabaseCustomerRepository) ListCustomers(page, limit int) ([]*domain.Customer, int, error) {
	offset := (page - 1) * limit

	// Count total
	var total int
	err := r.db.QueryRow("SELECT COUNT(*) FROM customers").Scan(&total)
	if err != nil {
		return nil, 0, fmt.Errorf("failed to count customers: %w", err)
	}

	query := `
		SELECT 
			id, mikrotik_id, username, name, phone, email, service_type,
			pppoe_username, pppoe_password, pppoe_profile,
			hotspot_username, hotspot_password, hotspot_mac_address,
			static_ip, assigned_ip, mac_address, last_online,
			status, created_at, updated_at
		FROM customers
		ORDER BY created_at DESC
		LIMIT $1 OFFSET $2
	`

	rows, err := r.db.Query(query, limit, offset)
	if err != nil {
		return nil, 0, fmt.Errorf("failed to query customers: %w", err)
	}
	defer rows.Close()

	var customers []*domain.Customer
	for rows.Next() {
		var c domain.Customer
		err := rows.Scan(
			&c.ID, &c.MikrotikID, &c.Username, &c.Name,
			&c.Phone, &c.Email, &c.ServiceType,
			&c.PPPoEUsername, &c.PPPoEPassword, &c.PPPoEProfile,
			&c.HotspotUsername, &c.HotspotPassword, &c.HotspotMacAddr,
			&c.StaticIP, &c.AssignedIP, &c.MacAddress, &c.LastOnline,
			&c.Status, &c.CreatedAt, &c.UpdatedAt,
		)
		if err != nil {
			return nil, 0, fmt.Errorf("failed to scan customer: %w", err)
		}
		customers = append(customers, &c)
	}
	return customers, total, nil
}
