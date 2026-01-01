package services

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"strings"
	"sync"
	"time"

	"mikrotik-collector/internal/domain"
	"mikrotik-collector/internal/infrastructure/mikrotik"
)

// OnDemandTrafficService monitors traffic only for requested customers
type OnDemandTrafficService struct {
	client    *mikrotik.Client
	db        domain.CustomerRepository
	publisher domain.RedisPublisher

	// Active monitors: key = customerID, value = monitor context
	activeMonitors map[string]*CustomerMonitor
	mu             sync.Mutex

	// Lock for preventing duplicate start/stops per customer
	monitorLocks map[string]*sync.Mutex
	locksMu      sync.Mutex
}

// CustomerMonitor represents a monitored customer session
type CustomerMonitor struct {
	CustomerID    string
	InterfaceName string
	Cancel        context.CancelFunc
	Clients       int
	Observers     map[chan domain.CustomerTrafficData]bool
	restartCount  int // Track restart attempts
}

// NewOnDemandTrafficService creates a new on-demand traffic service
func NewOnDemandTrafficService(
	client *mikrotik.Client,
	db domain.CustomerRepository,
	publisher domain.RedisPublisher,
) *OnDemandTrafficService {
	return &OnDemandTrafficService{
		client:         client,
		db:             db,
		publisher:      publisher,
		activeMonitors: make(map[string]*CustomerMonitor),
		monitorLocks:   make(map[string]*sync.Mutex),
	}
}

// StartMonitoring starts monitoring a specific customer if not already started
func (s *OnDemandTrafficService) StartMonitoring(ctx context.Context, customerID string) (<-chan domain.CustomerTrafficData, error) {
	// 1. Get lock for this customer to prevent race conditions
	s.locksMu.Lock()
	if _, ok := s.monitorLocks[customerID]; !ok {
		s.monitorLocks[customerID] = &sync.Mutex{}
	}
	lock := s.monitorLocks[customerID]
	s.locksMu.Unlock()

	lock.Lock()
	defer lock.Unlock()

	s.mu.Lock()
	monitor, exists := s.activeMonitors[customerID]
	if exists {
		// Already monitoring, just increment client count
		monitor.Clients++
		s.mu.Unlock()
		log.Printf("[OnDemand] Customer %s: Client count incremented to %d", customerID, monitor.Clients)

		// Subscribe to existing monitor
		return s.addObserver(ctx, customerID)
	}
	s.mu.Unlock()

	// 2. Not monitoring yet, need to start.
	// Get customer details first
	customer, err := s.db.GetCustomerByID(customerID)
	if err != nil {
		return nil, fmt.Errorf("customer not found: %w", err)
	}

	// Validate customer data
	if customer.PPPoEUsername == nil || *customer.PPPoEUsername == "" {
		return nil, fmt.Errorf("customer has no PPPoE username configured")
	}

	// Get the actual active PPPoE interface from MikroTik
	interfaceName, err := s.getActiveInterfaceForCustomer(customer)
	if err != nil {
		return nil, fmt.Errorf("failed to get active interface: %w", err)
	}

	if interfaceName == "" {
		return nil, fmt.Errorf("customer is not currently connected (no active PPPoE session)")
	}

	// Create monitor context
	monitorCtx, cancel := context.WithCancel(context.Background())

	monitor = &CustomerMonitor{
		CustomerID:    customerID,
		InterfaceName: interfaceName,
		Cancel:        cancel,
		Clients:       1,
		Observers:     make(map[chan domain.CustomerTrafficData]bool),
		restartCount:  0,
	}

	s.mu.Lock()
	s.activeMonitors[customerID] = monitor
	s.mu.Unlock()

	// Start the actual background monitoring for this customer
	go s.runMonitorLoop(monitorCtx, customer, interfaceName)

	log.Printf("[OnDemand] Started monitoring for customer %s (%s) on interface %s", 
		customer.Name, *customer.PPPoEUsername, interfaceName)

	return s.addObserver(ctx, customerID)
}

// getActiveInterfaceForCustomer finds the active PPPoE interface for a customer
func (s *OnDemandTrafficService) getActiveInterfaceForCustomer(customer *domain.Customer) (string, error) {
	if customer.PPPoEUsername == nil || *customer.PPPoEUsername == "" {
		return "", fmt.Errorf("no PPPoE username configured")
	}

	username := strings.ToLower(strings.TrimSpace(*customer.PPPoEUsername))
	
	// Query MikroTik for active PPPoE interfaces
	reply, err := s.client.Run(
		"/interface/print",
		"?type=pppoe-in",
		"?running=yes",
	)
	if err != nil {
		return "", fmt.Errorf("failed to query MikroTik interfaces: %w", err)
	}

	// Search for interface matching this username
	for _, re := range reply.Re {
		interfaceName := re.Map["name"]
		if interfaceName == "" {
			continue
		}

		// Extract username from interface name
		// Format: <pppoe-username>
		extractedUser := extractPPPoEUsername(interfaceName)
		if extractedUser == "" {
			continue
		}

		extractedUser = strings.ToLower(strings.TrimSpace(extractedUser))
		
		if extractedUser == username {
			log.Printf("[OnDemand] Found active interface '%s' for username '%s'", 
				interfaceName, username)
			return interfaceName, nil
		}
	}

	return "", fmt.Errorf("no active PPPoE session found for username '%s'", username)
}

// extractPPPoEUsername extracts username from PPPoE interface name
func extractPPPoEUsername(interfaceName string) string {
	// Format: <pppoe-username>
	if len(interfaceName) < 9 {
		return ""
	}

	if strings.HasPrefix(interfaceName, "<pppoe-") && strings.HasSuffix(interfaceName, ">") {
		return interfaceName[7 : len(interfaceName)-1]
	}

	return ""
}

// StopMonitoring decrements client count and stops monitoring if zero
func (s *OnDemandTrafficService) StopMonitoring(customerID string) {
	s.locksMu.Lock()
	if _, ok := s.monitorLocks[customerID]; !ok {
		s.locksMu.Unlock()
		return
	}
	lock := s.monitorLocks[customerID]
	s.locksMu.Unlock()

	lock.Lock()
	defer lock.Unlock()

	s.mu.Lock()
	monitor, exists := s.activeMonitors[customerID]
	if !exists {
		s.mu.Unlock()
		return
	}

	monitor.Clients--
	log.Printf("[OnDemand] Customer %s: Client count decremented to %d", customerID, monitor.Clients)

	if monitor.Clients <= 0 {
		// No more clients, stop monitoring
		monitor.Cancel()
		delete(s.activeMonitors, customerID)

		// Close all observers
		for ch := range monitor.Observers {
			close(ch)
		}

		log.Printf("[OnDemand] Stopped monitoring for customer %s", customerID)
	}
	s.mu.Unlock()
}

// runMonitorLoop runs the actual MikroTik monitoring command with auto-restart
func (s *OnDemandTrafficService) runMonitorLoop(ctx context.Context, customer *domain.Customer, interfaceName string) {
	maxRestarts := 3
	restartDelay := 5 * time.Second

	for {
		select {
		case <-ctx.Done():
			log.Printf("[OnDemand] Monitor context cancelled for %s", customer.Name)
			return
		default:
			// Check restart count
			s.mu.Lock()
			monitor, exists := s.activeMonitors[customer.ID]
			if !exists {
				s.mu.Unlock()
				return
			}
			
			if monitor.restartCount >= maxRestarts {
				log.Printf("[OnDemand] Max restart attempts (%d) reached for %s, stopping monitor", 
					maxRestarts, customer.Name)
				monitor.Cancel()
				delete(s.activeMonitors, customer.ID)
				
				// Notify observers about disconnection
				for ch := range monitor.Observers {
					select {
					case ch <- domain.CustomerTrafficData{
						CustomerID:   customer.ID,
						CustomerName: customer.Name,
						Timestamp:    time.Now(),
					}:
					default:
					}
					close(ch)
				}
				s.mu.Unlock()
				return
			}
			s.mu.Unlock()

			// Start monitoring stream
			trafficChan, err := mikrotik.MonitorTraffic(ctx, s.client, interfaceName)
			if err != nil {
				log.Printf("[OnDemand] Failed to start monitor for %s on %s: %v", 
					customer.Name, interfaceName, err)
				
				s.mu.Lock()
				if mon, ok := s.activeMonitors[customer.ID]; ok {
					mon.restartCount++
				}
				s.mu.Unlock()
				
				time.Sleep(restartDelay)
				continue
			}

			log.Printf("[OnDemand] Monitor stream active for %s on %s", customer.Name, interfaceName)

			// Process traffic data
			streamClosed := s.processTrafficStream(ctx, customer, trafficChan)
			
			if streamClosed {
				log.Printf("[OnDemand] Stream closed for %s, attempting restart...", customer.Name)
				
				s.mu.Lock()
				if mon, ok := s.activeMonitors[customer.ID]; ok {
					mon.restartCount++
				}
				s.mu.Unlock()
				
				time.Sleep(restartDelay)
			}
		}
	}
}

// processTrafficStream processes traffic data from the stream
func (s *OnDemandTrafficService) processTrafficStream(
	ctx context.Context, 
	customer *domain.Customer, 
	trafficChan <-chan mikrotik.InterfaceTraffic,
) bool {
	for {
		select {
		case <-ctx.Done():
			return false
		case traffic, ok := <-trafficChan:
			if !ok {
				log.Printf("[OnDemand] Traffic channel closed for %s", customer.Name)
				return true // Stream closed
			}
			
			// Validate traffic data
			if traffic.Name == "" {
				log.Printf("[OnDemand] WARNING: Received traffic data with empty interface name for %s", 
					customer.Name)
				continue
			}

			data := s.mapToCustomerTraffic(customer, traffic)
			s.publishTrafficData(data)
		}
	}
}

// addObserver creates a channel and adds it to the monitor's observers
func (s *OnDemandTrafficService) addObserver(ctx context.Context, customerID string) (<-chan domain.CustomerTrafficData, error) {
	s.mu.Lock()
	monitor, exists := s.activeMonitors[customerID]
	if !exists {
		s.mu.Unlock()
		return nil, fmt.Errorf("monitor not running for customer %s", customerID)
	}

	// Create buffered channel to prevent blocking
	ch := make(chan domain.CustomerTrafficData, 50)
	monitor.Observers[ch] = true
	s.mu.Unlock()

	// Cleanup routine: remove observer when context is done
	go func() {
		<-ctx.Done()
		s.mu.Lock()
		if m, ok := s.activeMonitors[customerID]; ok && m.Observers != nil {
			delete(m.Observers, ch)
			close(ch)
		}
		s.mu.Unlock()
	}()

	return ch, nil
}

func (s *OnDemandTrafficService) mapToCustomerTraffic(c *domain.Customer, t mikrotik.InterfaceTraffic) domain.CustomerTrafficData {
	return domain.CustomerTrafficData{
		CustomerID:         c.ID,
		CustomerName:       c.Name,
		Username:           c.Username,
		ServiceType:        c.ServiceType,
		InterfaceName:      t.Name,
		RxBitsPerSecond:    t.RxBitsPerSecond,
		TxBitsPerSecond:    t.TxBitsPerSecond,
		RxPacketsPerSecond: t.RxPacketsPerSecond,
		TxPacketsPerSecond: t.TxPacketsPerSecond,
		DownloadSpeed:      formatSpeed(t.RxBitsPerSecond),
		UploadSpeed:        formatSpeed(t.TxBitsPerSecond),
		Timestamp:          time.Now(),
	}
}

func (s *OnDemandTrafficService) publishTrafficData(data domain.CustomerTrafficData) {
	// 1. Publish to Redis (optional, for history/other consumers)
	jsonData, _ := json.Marshal(data)
	s.publisher.PublishStream("mikrotik:traffic:customers", string(jsonData))

	// 2. Broadcast to in-memory observers (active websockets)
	s.mu.Lock()
	defer s.mu.Unlock()

	if monitor, ok := s.activeMonitors[data.CustomerID]; ok {
		for ch := range monitor.Observers {
			select {
			case ch <- data:
			default:
				// Skip if channel full to prevent blocking
			}
		}
	}
}

// formatSpeed converts bits per second to human-readable format
func formatSpeed(bps string) string {
	if bps == "" || bps == "0" {
		return "0 bps"
	}
	
	// Simple implementation - use the one from continuous service if available
	return bps + " bps"
}