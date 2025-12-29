package main

import (
	"context"
	"encoding/json"
	"log"
	"time"

	"github.com/redis/go-redis/v9"
)

// RedisStreamConsumer consumes messages from Redis Stream and broadcasts to WebSocket
type RedisStreamConsumer struct {
	client    *redis.Client
	ctx       context.Context
	streamKey string
	broadcast chan<- []byte
}

// NewRedisStreamConsumer creates a new Redis Stream consumer
func NewRedisStreamConsumer(cfg *Config, broadcast chan<- []byte) *RedisStreamConsumer {
	client := redis.NewClient(&redis.Options{
		Addr:     cfg.RedisAddr,
		Password: cfg.RedisPassword,
		DB:       cfg.RedisDB,
	})

	ctx := context.Background()

	// Test connection
	if err := client.Ping(ctx).Err(); err != nil {
		log.Printf("WARNING: Redis connection failed in stream consumer: %v", err)
	} else {
		log.Printf("Redis Stream Consumer connected to %s", cfg.RedisAddr)
	}

	return &RedisStreamConsumer{
		client:    client,
		ctx:       ctx,
		streamKey: "mikrotik:traffic:customers",
		broadcast: broadcast,
	}
}

// Start begins consuming from Redis Stream
func (c *RedisStreamConsumer) Start() {
	// Create consumer group if it doesn't exist
	// Ignore error if group already exists
	c.client.XGroupCreateMkStream(c.ctx, c.streamKey, "websocket-broadcasters", "0")

	consumerName := "broadcaster-1"
	log.Printf("Starting Redis Stream consumer for stream: %s", c.streamKey)

	for {
		// Read from stream
		streams, err := c.client.XReadGroup(c.ctx, &redis.XReadGroupArgs{
			Group:    "websocket-broadcasters",
			Consumer: consumerName,
			Streams:  []string{c.streamKey, ">"},
			Count:    10,
			Block:    time.Second * 2,
		}).Result()

		if err != nil {
			if err == redis.Nil {
				// No new messages, continue
				continue
			}
			log.Printf("Error reading from stream: %v", err)
			time.Sleep(time.Second)
			continue
		}

		// Process messages
		for _, stream := range streams {
			for _, message := range stream.Messages {
				// Extract data field
				if data, ok := message.Values["data"].(string); ok {
					// Validate JSON before broadcasting
					var js json.RawMessage
					if err := json.Unmarshal([]byte(data), &js); err != nil {
						log.Printf("Invalid JSON in stream: %v", err)
						continue
					}

					// Broadcast to WebSocket clients
					c.broadcast <- []byte(data)
				}

				// Acknowledge the message
				c.client.XAck(c.ctx, c.streamKey, "websocket-broadcasters", message.ID)
			}
		}
	}
}

// Close closes the Redis connection
func (c *RedisStreamConsumer) Close() error {
	return c.client.Close()
}
