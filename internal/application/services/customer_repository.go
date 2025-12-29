package services

import (
	"database/sql"
	"fmt"
)

// DatabaseCustomerRepository implements CustomerRepository using SQL database
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
func (r *DatabaseCustomerRepository) GetActivePPPoECustomers() ([]*Customer, error) {
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

	var customers []*Customer
	for rows.Next() {
		var c Customer
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
func (r *DatabaseCustomerRepository) GetCustomerByID(id string) (*Customer, error) {
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

	c := &Customer{}
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
