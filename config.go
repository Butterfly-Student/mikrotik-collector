package main

import (
	"log"
	"os"
	"strconv"
)

// Config holds all application configuration
type Config struct {
	// MikroTik settings
	MikroTikHost     string
	MikroTikPort     string
	MikroTikUsername string
	MikroTikPassword string

	// Redis settings
	RedisAddr     string
	RedisPassword string
	RedisDB       int

	// WebSocket settings
	WSPort string

	// Database settings
	DBHost         string
	DBPort         int
	DBUser         string
	DBPassword     string
	DBName         string
	DBSSLMode      string
	DBMaxIdleConns int
	DBMaxOpenConns int

	// Traffic Monitor settings
	EnableTrafficMonitor  bool
	MaxConcurrentMonitors int
	AutoStartMonitoring   bool // NEW
}

// LoadConfig loads configuration from environment variables with defaults
func LoadConfig() *Config {
	return &Config{
		// MikroTik
		MikroTikHost:     getEnv("MIKROTIK_HOST", "192.168.100.1"),
		MikroTikPort:     getEnv("MIKROTIK_PORT", "8728"),
		MikroTikUsername: getEnv("MIKROTIK_USER", "admin"),
		MikroTikPassword: getEnv("MIKROTIK_PASS", "r00t"),

		// Redis
		RedisAddr:     getEnv("REDIS_ADDR", "localhost:6379"),
		RedisPassword: getEnv("REDIS_PASS", ""),
		RedisDB:       getEnvInt("REDIS_DB", 0),

		// WebSocket
		WSPort: getEnv("WS_PORT", "8080"),

		// Database
		DBHost:         getEnv("DB_HOST", "localhost"),
		DBPort:         getEnvInt("DB_PORT", 5432),
		DBUser:         getEnv("DB_USER", "root"),
		DBPassword:     getEnv("DB_PASSWORD", "r00t"),
		DBName:         getEnv("DB_NAME", "mikrobill-tes"),
		DBSSLMode:      getEnv("DB_SSLMODE", "disable"),
		DBMaxIdleConns: getEnvInt("DB_MAX_IDLE_CONNS", 5),
		DBMaxOpenConns: getEnvInt("DB_MAX_OPEN_CONNS", 10),

		// Traffic Monitor
		EnableTrafficMonitor:  getEnvBool("ENABLE_TRAFFIC_MONITOR", true),
		MaxConcurrentMonitors: getEnvInt("MAX_CONCURRENT_MONITORS", 50),
		AutoStartMonitoring:   getEnvBool("AUTO_START_MONITORING", false), // NEW
		
	}
}

// Validate checks if required configuration is present
func (c *Config) Validate() error {
	if c.MikroTikPassword == "" {
		log.Println("WARNING: MIKROTIK_PASS is not set!")
	}
	return nil
}

// MikroTikPortInt converts port string to int
func (c *Config) MikroTikPortInt() int {
	port := getEnvInt("MIKROTIK_PORT", 8728)
	return port
}

func getEnv(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}

func getEnvInt(key string, defaultValue int) int {
	if value := os.Getenv(key); value != "" {
		if intVal, err := strconv.Atoi(value); err == nil {
			return intVal
		}
	}
	return defaultValue
}

func getEnvBool(key string, defaultValue bool) bool {
	if value := os.Getenv(key); value != "" {
		if value == "true" || value == "1" || value == "yes" {
			return true
		}
		if value == "false" || value == "0" || value == "no" {
			return false
		}
	}
	return defaultValue
}
