package domain

import (
	"fmt"
	"time"
)

// Customer represents a customer in the system
type Customer struct {
	ID          string
	MikrotikID  string
	Username    string
	Name        string
	Phone       *string
	Email       *string
	ServiceType string // pppoe, hotspot, static_ip

	// PPPoE specific
	PPPoEUsername *string
	PPPoEPassword *string
	PPPoEProfile  *string

	// Hotspot specific
	HotspotUsername *string
	HotspotPassword *string
	HotspotMacAddr  *string

	// Static IP
	StaticIP *string

	// Network info
	AssignedIP *string
	MacAddress *string
	LastOnline *time.Time

	Status    string // active, suspended, inactive, pending
	CreatedAt time.Time
	UpdatedAt time.Time
}

// CustomerTrafficData represents traffic data for a customer
type CustomerTrafficData struct {
	CustomerID         string    `json:"customer_id"`
	CustomerName       string    `json:"customer_name"`
	Username           string    `json:"username"`
	ServiceType        string    `json:"service_type"`
	InterfaceName      string    `json:"interface_name"`
	RxBitsPerSecond    string    `json:"rx_bits_per_second"`
	TxBitsPerSecond    string    `json:"tx_bits_per_second"`
	RxPacketsPerSecond string    `json:"rx_packets_per_second"`
	TxPacketsPerSecond string    `json:"tx_packets_per_second"`
	DownloadSpeed      string    `json:"download_speed"`
	UploadSpeed        string    `json:"upload_speed"`
	Timestamp          time.Time `json:"timestamp"`
}

// CustomerRepository defines database operations for customers
type CustomerRepository interface {
	GetActivePPPoECustomers() ([]*Customer, error)
	GetCustomerByID(id string) (*Customer, error)
	GetCustomerByPPPoEUsername(username string) (*Customer, error)
	UpdateCustomerStatus(id string, status string, ipAddress *string, macAddress *string) error

	// CRUD operations
	CreateCustomer(customer *Customer) error
	UpdateCustomer(customer *Customer) error
	DeleteCustomer(id string) error
	ListCustomers(page, limit int) ([]*Customer, int, error)
}

// RedisPublisher defines interface for publishing to Redis
type RedisPublisher interface {
	Publish(channel string, message string) error
	PublishStream(streamKey string, data string) error
}

// GetInterfaceNameForCustomer returns the interface name for monitoring
func (c *Customer) GetInterfaceNameForCustomer() (string, error) {
	switch c.ServiceType {
	case "pppoe":
		if c.PPPoEUsername != nil && *c.PPPoEUsername != "" {
			return fmt.Sprintf("<%s>", *c.PPPoEUsername), nil
		}
		return "", fmt.Errorf("pppoe username not set for customer %s", c.ID)
	case "hotspot":
		return "", fmt.Errorf("hotspot interface monitoring not implemented yet")
	case "static_ip":
		return "", fmt.Errorf("static IP interface monitoring not implemented yet")
	default:
		return "", fmt.Errorf("unsupported service type: %s", c.ServiceType)
	}
}
