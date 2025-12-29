package main

import (
	"context"
	"fmt"
	"log"

	"github.com/redis/go-redis/v9"
)

// RedisPublisher handles publishing to Redis
type RedisPublisher struct {
	client *redis.Client
	ctx    context.Context
}

// NewRedisPublisher creates a new Redis publisher
func NewRedisPublisher(cfg *Config) *RedisPublisher {
	client := redis.NewClient(&redis.Options{
		Addr:     cfg.RedisAddr,
		Password: cfg.RedisPassword,
		DB:       cfg.RedisDB,
	})

	ctx := context.Background()

	// Test connection
	if err := client.Ping(ctx).Err(); err != nil {
		log.Printf("WARNING: Redis connection failed: %v", err)
	} else {
		log.Printf("Connected to Redis at %s", cfg.RedisAddr)
	}

	return &RedisPublisher{
		client: client,
		ctx:    ctx,
	}
}

// Publish publishes a message to a Redis channel (Pub/Sub)
func (r *RedisPublisher) Publish(channel string, message string) error {
	err := r.client.Publish(r.ctx, channel, message).Err()
	if err != nil {
		return fmt.Errorf("failed to publish to channel %s: %w", channel, err)
	}
	return nil
}

// PublishStream publishes a message to a Redis Stream
func (r *RedisPublisher) PublishStream(streamKey string, message string) error {
	err := r.client.XAdd(r.ctx, &redis.XAddArgs{
		Stream: streamKey,
		MaxLen: 10000,
		Approx: true,
		Values: map[string]interface{}{
			"data": message,
		},
	}).Err()
	if err != nil {
		return fmt.Errorf("failed to publish to stream %s: %w", streamKey, err)
	}
	return nil
}

// Close closes the Redis connection
func (r *RedisPublisher) Close() error {
	return r.client.Close()
}

// IsConnected checks if Redis is connected
func (r *RedisPublisher) IsConnected() bool {
	return r.client.Ping(r.ctx).Err() == nil
}