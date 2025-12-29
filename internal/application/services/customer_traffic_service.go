package services

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"strconv"
	"strings"
	"sync"
	"time"

	"mikrotik-collector/internal/infrastructure/mikrotik"
)

// ContinuousTrafficService monitors all active PPPoE interfaces continuously
type ContinuousTrafficService struct {
	client    *mikrotik.Client
	db        CustomerRepository
	publisher RedisPublisher

	// Active monitors: key = interface_name, value = monitor context
	activeMonitors map[string]*InterfaceMonitor

	// Customer mapping: key = pppoe_username (lowercase), value = customer
	customerMap map[string]*Customer

	mu     sync.RWMutex
	ctx    context.Context
	cancel context.CancelFunc
	wg     sync.WaitGroup
}

// InterfaceMonitor represents a monitored interface
type InterfaceMonitor struct {
	InterfaceName string
	Customer      *Customer
	Cancel        context.CancelFunc
	StartedAt     time.Time
}

// NewContinuousTrafficService creates a new continuous traffic service
func NewContinuousTrafficService(
	client *mikrotik.Client,
	db CustomerRepository,
	publisher RedisPublisher,
) *ContinuousTrafficService {
	ctx, cancel := context.WithCancel(context.Background())

	return &ContinuousTrafficService{
		client:         client,
		db:             db,
		publisher:      publisher,
		activeMonitors: make(map[string]*InterfaceMonitor),
		customerMap:    make(map[string]*Customer),
		ctx:            ctx,
		cancel:         cancel,
	}
}

// Start begins continuous monitoring
func (s *ContinuousTrafficService) Start() error {
	log.Println("[ContinuousTrafficService] Starting continuous traffic monitoring...")

	// Step 1: Load customers from database (ONCE)
	if err := s.loadCustomers(); err != nil {
		return fmt.Errorf("failed to load customers: %w", err)
	}

	// Step 2: Get all active PPPoE interfaces from MikroTik (ONCE)
	activeInterfaces, err := s.getActivePPPoEInterfaces()
	if err != nil {
		return fmt.Errorf("failed to get active interfaces: %w", err)
	}

	// Step 3: Match and start monitoring
	matched := s.matchAndStartMonitors(activeInterfaces)

	log.Printf("[ContinuousTrafficService] Started monitoring %d/%d customers",
		matched, len(s.customerMap))

	return nil
}

// Stop stops all monitoring
func (s *ContinuousTrafficService) Stop() {
	log.Println("[ContinuousTrafficService] Stopping continuous monitoring...")

	s.mu.Lock()
	// Cancel all individual monitors
	for _, monitor := range s.activeMonitors {
		monitor.Cancel()
	}
	s.mu.Unlock()

	// Cancel main context
	s.cancel()

	// Wait for all goroutines to finish
	s.wg.Wait()

	log.Println("[ContinuousTrafficService] All monitors stopped")
}

// loadCustomers loads all active PPPoE customers from database (ONCE)
func (s *ContinuousTrafficService) loadCustomers() error {
	customers, err := s.db.GetActivePPPoECustomers()
	if err != nil {
		return err
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	s.customerMap = make(map[string]*Customer)
	for _, customer := range customers {
		if customer.PPPoEUsername != nil && *customer.PPPoEUsername != "" {
			username := strings.ToLower(strings.TrimSpace(*customer.PPPoEUsername))
			s.customerMap[username] = customer
		}
	}

	log.Printf("[ContinuousTrafficService] Loaded %d customers from database", len(s.customerMap))
	return nil
}

// getActivePPPoEInterfaces gets all active PPPoE interfaces from MikroTik (ONCE)
func (s *ContinuousTrafficService) getActivePPPoEInterfaces() ([]string, error) {
	// Query interfaces that are: PPPoE + Running
	reply, err := s.client.Run(
		"/interface/print",
		"?type=pppoe-in",
		"?running=yes",
	)
	if err != nil {
		return nil, fmt.Errorf("failed to query interfaces: %w", err)
	}

	var interfaces []string
	for _, re := range reply.Re {
		interfaceName := re.Map["name"]
		if interfaceName != "" {
			interfaces = append(interfaces, interfaceName)
		}
	}

	log.Printf("[ContinuousTrafficService] Found %d active PPPoE interfaces", len(interfaces))
	return interfaces, nil
}

// matchAndStartMonitors matches interfaces with customers and starts monitoring
func (s *ContinuousTrafficService) matchAndStartMonitors(interfaces []string) int {
	s.mu.Lock()
	defer s.mu.Unlock()

	matched := 0
	for _, interfaceName := range interfaces {
		// Extract PPPoE username from interface name
		// Format: <pppoe-username>
		pppoeUsername := extractPPPoEUsername(interfaceName)
		if pppoeUsername == "" {
			continue
		}

		// Normalize username
		pppoeUsername = strings.ToLower(strings.TrimSpace(pppoeUsername))

		// Check if we have this customer in database
		customer, exists := s.customerMap[pppoeUsername]
		if !exists {
			log.Printf("[DEBUG] Ignored interface '%s' (username '%s'): not in database",
				interfaceName, pppoeUsername)
			continue
		}

		// Start monitoring for this interface
		s.startMonitorForInterface(interfaceName, customer)
		matched++
	}

	return matched
}

// startMonitorForInterface starts a continuous monitor for a single interface
func (s *ContinuousTrafficService) startMonitorForInterface(interfaceName string, customer *Customer) {
	// Check if already monitoring
	if _, exists := s.activeMonitors[interfaceName]; exists {
		return
	}

	// Create context for this monitor
	ctx, cancel := context.WithCancel(s.ctx)

	monitor := &InterfaceMonitor{
		InterfaceName: interfaceName,
		Customer:      customer,
		Cancel:        cancel,
		StartedAt:     time.Now(),
	}

	s.activeMonitors[interfaceName] = monitor

	// Start monitoring goroutine
	s.wg.Add(1)
	go s.monitorInterface(ctx, monitor)

	log.Printf("[ContinuousTrafficService] Started monitoring: %s → %s",
		interfaceName, customer.Name)
}

// monitorInterface monitors a single interface continuously using MikroTik's streaming API
func (s *ContinuousTrafficService) monitorInterface(ctx context.Context, monitor *InterfaceMonitor) {
	defer s.wg.Done()
	defer func() {
		s.mu.Lock()
		delete(s.activeMonitors, monitor.InterfaceName)
		s.mu.Unlock()
		log.Printf("[ContinuousTrafficService] Stopped monitoring: %s → %s",
			monitor.InterfaceName, monitor.Customer.Name)
	}()

	// Keep running until context is cancelled
	for {
		select {
		case <-ctx.Done():
			return
		default:
			// (Re)start monitoring
			s.runMonitorStream(ctx, monitor)

			// If runMonitorStream returns, it means it failed or stopped.
			// Check context again before retrying
			select {
			case <-ctx.Done():
				return
			case <-time.After(5 * time.Second):
				// Wait before retrying to avoid hot loop on persistent failure
				log.Printf("[INFO] Restarting monitor for %s...", monitor.InterfaceName)
			}
		}
	}
}

func (s *ContinuousTrafficService) runMonitorStream(ctx context.Context, monitor *InterfaceMonitor) {
	// Use MikroTik's monitor-traffic command (it streams data automatically)
	trafficChan, err := mikrotik.MonitorTraffic(ctx, s.client, monitor.InterfaceName)
	if err != nil {
		log.Printf("[ERROR] Failed to start monitoring %s: %v", monitor.InterfaceName, err)
		return
	}

	log.Printf("[DEBUG] Monitoring stream started for %s", monitor.InterfaceName)

	// Process traffic data from the stream
	for {
		select {
		case <-ctx.Done():
			return
		case traffic, ok := <-trafficChan:
			if !ok {
				// Channel closed, interface might be disconnected
				log.Printf("[INFO] Traffic channel closed for %s (customer disconnected or connection reset)",
					monitor.InterfaceName)
				return
			}

			// Process and publish traffic data
			s.processTrafficData(monitor, traffic)
		}
	}
}

// processTrafficData processes and publishes traffic data
func (s *ContinuousTrafficService) processTrafficData(
	monitor *InterfaceMonitor,
	traffic mikrotik.InterfaceTraffic,
) {
	customerData := CustomerTrafficData{
		CustomerID:         monitor.Customer.ID,
		CustomerName:       monitor.Customer.Name,
		Username:           monitor.Customer.Username,
		ServiceType:        monitor.Customer.ServiceType,
		InterfaceName:      monitor.InterfaceName,
		RxBitsPerSecond:    traffic.RxBitsPerSecond,
		TxBitsPerSecond:    traffic.TxBitsPerSecond,
		RxPacketsPerSecond: traffic.RxPacketsPerSecond,
		TxPacketsPerSecond: traffic.TxPacketsPerSecond,
		DownloadSpeed:      formatSpeed(traffic.RxBitsPerSecond),
		UploadSpeed:        formatSpeed(traffic.TxBitsPerSecond),
		Timestamp:          time.Now(),
	}

	// Publish to Redis
	if err := s.publishTrafficData(customerData); err != nil {
		log.Printf("[ERROR] Failed to publish data for %s: %v", monitor.Customer.Name, err)
	}
}

// publishTrafficData publishes customer traffic data to Redis Stream
func (s *ContinuousTrafficService) publishTrafficData(data CustomerTrafficData) error {
	jsonData, err := json.Marshal(data)
	if err != nil {
		return fmt.Errorf("failed to marshal data: %w", err)
	}

	streamKey := "mikrotik:traffic:customers"
	return s.publisher.PublishStream(streamKey, string(jsonData))
}

// GetActiveInterfaces returns list of currently monitored interfaces
func (s *ContinuousTrafficService) GetActiveInterfaces() []string {
	s.mu.RLock()
	defer s.mu.RUnlock()

	interfaces := make([]string, 0, len(s.activeMonitors))
	for interfaceName := range s.activeMonitors {
		interfaces = append(interfaces, interfaceName)
	}
	return interfaces
}

// GetMonitorCount returns the number of active monitors
func (s *ContinuousTrafficService) GetMonitorCount() int {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return len(s.activeMonitors)
}

// GetCustomerCount returns the number of customers in database
func (s *ContinuousTrafficService) GetCustomerCount() int {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return len(s.customerMap)
}

// ReloadCustomers reloads customers from database and restarts monitoring
func (s *ContinuousTrafficService) ReloadCustomers() error {
	log.Println("[ContinuousTrafficService] Reloading customers...")

	// Stop all current monitors
	s.mu.Lock()
	for _, monitor := range s.activeMonitors {
		monitor.Cancel()
	}
	s.activeMonitors = make(map[string]*InterfaceMonitor)
	s.mu.Unlock()

	// Wait for goroutines to finish
	s.wg.Wait()

	// Reload customers
	if err := s.loadCustomers(); err != nil {
		return err
	}

	// Get active interfaces again
	activeInterfaces, err := s.getActivePPPoEInterfaces()
	if err != nil {
		return err
	}

	// Restart monitoring
	matched := s.matchAndStartMonitors(activeInterfaces)
	log.Printf("[ContinuousTrafficService] Reloaded: monitoring %d customers", matched)

	return nil
}

// extractPPPoEUsername extracts username from PPPoE interface name
// Format: <pppoe-username> -> username
func extractPPPoEUsername(interfaceName string) string {
	// Format: <pppoe-username>
	if len(interfaceName) < 9 {
		return ""
	}

	if interfaceName[:7] == "<pppoe-" && interfaceName[len(interfaceName)-1] == '>' {
		return interfaceName[7 : len(interfaceName)-1]
	}

	return ""
}

// formatSpeed converts bits per second to human-readable format
func formatSpeed(bps string) string {
	if bps == "" || bps == "0" {
		return "0 bps"
	}

	bits, err := strconv.ParseFloat(bps, 64)
	if err != nil {
		return bps + " bps"
	}

	units := []string{"bps", "Kbps", "Mbps", "Gbps"}
	unitIndex := 0

	for bits >= 1000 && unitIndex < len(units)-1 {
		bits /= 1000
		unitIndex++
	}

	return fmt.Sprintf("%.2f %s", bits, units[unitIndex])
}
