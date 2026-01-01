package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"

	"mikrotik-collector/internal/infrastructure/mikrotik"
)

func main() {
	// Konfigurasi koneksi MikroTik
	cfg := mikrotik.Config{
		Host:     "192.168.100.1", // Ganti dengan IP MikroTik Anda
		Port:     8728,            // Port API MikroTik (default: 8728)
		Username: "admin",         // Ganti dengan username Anda
		Password: "r00t",      // Ganti dengan password Anda
		Timeout:  10 * time.Second,
		UseTLS:   false, // Set true jika menggunakan TLS
		Queue:    100,
	}

	// Membuat client MikroTik
	fmt.Println("Connecting to MikroTik...")
	client, err := mikrotik.NewClient(cfg)
	if err != nil {
		log.Fatalf("Failed to connect: %v", err)
	}
	defer client.Close()
	fmt.Println("Connected successfully!")

	// Test basic command - mendapatkan identitas router
	fmt.Println("\n=== Testing Basic Command ===")
	reply, err := client.Run("/system/identity/print")
	if err != nil {
		log.Fatalf("Failed to get identity: %v", err)
	}
	fmt.Printf("Router Identity: %v\n", reply.Re[0].Map["name"])

	// List interfaces
	fmt.Println("\n=== Available Interfaces ===")
	reply, err = client.Run("/interface/print")
	if err != nil {
		log.Fatalf("Failed to get interfaces: %v", err)
	}
	
	var iface string
	for i, re := range reply.Re {
		name := re.Map["name"]
		ifaceType := re.Map["type"]
		fmt.Printf("%d. %s (%s)\n", i+1, name, ifaceType)
		if i == 0 {
			iface = name // Gunakan interface pertama sebagai default
		}
	}

	if iface == "" {
		log.Fatal("No interfaces found")
	}

	// Monitor traffic untuk interface yang dipilih
	fmt.Printf("\n=== Monitoring Traffic on '%s' ===\n", iface)
	fmt.Println("Press Ctrl+C to stop...")
	fmt.Println()

	// Setup context dengan cancellation
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Handle interrupt signal
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)
	go func() {
		<-sigChan
		fmt.Println("\n\nReceived interrupt signal, stopping...")
		cancel()
	}()

	// Mulai monitoring traffic
	trafficChan, err := mikrotik.MonitorTraffic(ctx, client, iface)
	if err != nil {
		log.Fatalf("Failed to start monitoring: %v", err)
	}

	// Counter untuk menampilkan data
	count := 0
	for traffic := range trafficChan {
		count++
		
		// Tampilkan header setiap 20 baris
		if count%20 == 1 {
			fmt.Println("┌─────────────────────────────────────────────────────────────────────┐")
			fmt.Printf("│ %-67s │\n", "Interface: "+traffic.Name)
			fmt.Println("├─────────────────────────────────────────────────────────────────────┤")
			fmt.Printf("│ %-20s │ %-20s │ %-20s │\n", "Metric", "RX", "TX")
			fmt.Println("├─────────────────────────────────────────────────────────────────────┤")
		}

		// Tampilkan data traffic
		fmt.Printf("│ %-20s │ %-20s │ %-20s │\n", 
			"Bits/sec", 
			formatValue(traffic.RxBitsPerSecond), 
			formatValue(traffic.TxBitsPerSecond))
		
		fmt.Printf("│ %-20s │ %-20s │ %-20s │\n", 
			"Packets/sec", 
			formatValue(traffic.RxPacketsPerSecond), 
			formatValue(traffic.TxPacketsPerSecond))
		
		if traffic.RxDropsPerSecond != "0" || traffic.TxDropsPerSecond != "0" {
			fmt.Printf("│ %-20s │ %-20s │ %-20s │\n", 
				"Drops/sec", 
				formatValue(traffic.RxDropsPerSecond), 
				formatValue(traffic.TxDropsPerSecond))
		}
		
		if traffic.RxErrorsPerSecond != "0" || traffic.TxErrorsPerSecond != "0" {
			fmt.Printf("│ %-20s │ %-20s │ %-20s │\n", 
				"Errors/sec", 
				formatValue(traffic.RxErrorsPerSecond), 
				formatValue(traffic.TxErrorsPerSecond))
		}
		
		fmt.Println("└─────────────────────────────────────────────────────────────────────┘")
		fmt.Println()
		
		time.Sleep(1 * time.Second)
	}

	fmt.Println("Monitoring stopped.")
}

// formatValue memformat nilai untuk tampilan yang lebih baik
func formatValue(val string) string {
	if val == "" {
		return "0"
	}
	return val
}