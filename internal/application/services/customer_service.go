package services

import (
	"fmt"
	"log"

	"mikrotik-collector/internal/domain"
	"mikrotik-collector/internal/infrastructure/mikrotik"
)

// CustomerService handles business logic for customers
type CustomerService struct {
	repo     domain.CustomerRepository
	mtClient *mikrotik.Client
}

// NewCustomerService creates a new customer service
func NewCustomerService(repo domain.CustomerRepository, mtClient *mikrotik.Client) *CustomerService {
	return &CustomerService{
		repo:     repo,
		mtClient: mtClient,
	}
}

// CreateCustomer creates a customer in DB and MikroTik (if PPPoE)
func (s *CustomerService) CreateCustomer(c *domain.Customer) error {
	// 1. Create in Database first (Source of Truth)
	if err := s.repo.CreateCustomer(c); err != nil {
		return fmt.Errorf("failed to create customer in db: %w", err)
	}

	// 2. Sync to MikroTik if PPPoE
	if c.ServiceType == "pppoe" && s.mtClient != nil {
		username := ""
		password := ""
		profile := "default"

		if c.PPPoEUsername != nil {
			username = *c.PPPoEUsername
		}
		if c.PPPoEPassword != nil {
			password = *c.PPPoEPassword
		}
		if c.PPPoEProfile != nil && *c.PPPoEProfile != "" {
			profile = *c.PPPoEProfile
		}

		if username == "" {
			// Require username for PPPoE
			// Rollback DB?
			s.repo.DeleteCustomer(c.ID)
			return fmt.Errorf("pppoe username is required")
		}

		// Create Secret
		mtID, err := s.mtClient.CreatePPPoESecret(
			username,
			password,
			profile,
			"", // local address
			"", // remote address - usually assigned by profile/pool, but can be set if static
		)

		if err != nil {
			// Rollback DB
			log.Printf("Failed to create MikroTik secret for %s: %v. Rolling back DB.", username, err)
			s.repo.DeleteCustomer(c.ID)
			return fmt.Errorf("failed to create mikrotik secret: %w", err)
		}

		// Update DB with MikroTik ID
		c.MikrotikID = mtID
		s.repo.UpdateCustomer(c)
	}

	return nil
}

// UpdateCustomer updates customer in DB and MikroTik
func (s *CustomerService) UpdateCustomer(c *domain.Customer) error {
	// Get existing to compare?
	oldC, err := s.repo.GetCustomerByID(c.ID)
	if err != nil {
		return err
	}

	// 1. Update Database
	if err := s.repo.UpdateCustomer(c); err != nil {
		return fmt.Errorf("failed to update customer in db: %w", err)
	}

	// 2. Sync to MikroTik
	if c.ServiceType == "pppoe" && s.mtClient != nil {
		// Needs MikroTik ID. If missing, try to find by OLD username
		mtID := c.MikrotikID
		if mtID == "" {
			usernameToFind := ""
			if oldC.PPPoEUsername != nil {
				usernameToFind = *oldC.PPPoEUsername
			}
			if usernameToFind != "" {
				foundID, err := s.mtClient.FindPPPoESecretID(usernameToFind)
				if err == nil && foundID != "" {
					mtID = foundID
				}
			}
		}

		if mtID != "" {
			username := ""
			password := ""
			profile := ""

			if c.PPPoEUsername != nil {
				username = *c.PPPoEUsername
			}
			if c.PPPoEPassword != nil {
				password = *c.PPPoEPassword
			}
			if c.PPPoEProfile != nil {
				profile = *c.PPPoEProfile
			}

			err := s.mtClient.UpdatePPPoESecret(
				mtID,
				username,
				password,
				profile,
				"", "",
			)
			if err != nil {
				return fmt.Errorf("failed to update mikrotik secret: %w", err)
			}
		} else {
			// Not found in MikroTik? Maybe active but no secret?
			// Or maybe we should create it?
			// For Safe Update, let's just log warning.
			log.Printf("Warning: MikroTik Secret ID not found for customer %s. Skipping MikroTik update.", c.Name)
		}
	}

	return nil
}

// DeleteCustomer deletes customer from DB and MikroTik
func (s *CustomerService) DeleteCustomer(id string) error {
	c, err := s.repo.GetCustomerByID(id)
	if err != nil {
		return err
	}

	// 1. Delete from MikroTik first? Or DB first?
	// If we delete from DB first, we lose the ID needed for MikroTik.

	if c.ServiceType == "pppoe" && s.mtClient != nil {
		mtID := c.MikrotikID
		if mtID == "" && c.PPPoEUsername != nil {
			mtID, _ = s.mtClient.FindPPPoESecretID(*c.PPPoEUsername)
		}

		if mtID != "" {
			if err := s.mtClient.DeletePPPoESecret(mtID); err != nil {
				log.Printf("Warning: Failed to delete MikroTik secret: %v", err)
				// Proceed to delete from DB anyway?
				// Yes, because we want to remove from our system.
			}
		}
	}

	// 2. Delete from Database
	return s.repo.DeleteCustomer(id)
}

// GetCustomer returns a customer
func (s *CustomerService) GetCustomer(id string) (*domain.Customer, error) {
	return s.repo.GetCustomerByID(id)
}

// ListCustomers returns list of customers
func (s *CustomerService) ListCustomers(page, limit int) ([]*domain.Customer, int, error) {
	return s.repo.ListCustomers(page, limit)
}
