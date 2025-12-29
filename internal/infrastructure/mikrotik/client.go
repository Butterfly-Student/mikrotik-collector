package mikrotik

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/go-routeros/routeros/v3"
)

// Config holds MikroTik connection configuration
type Config struct {
	Host     string
	Port     int
	Username string
	Password string
	Timeout  time.Duration
	UseTLS   bool
	Queue    int // optional: default 100
}

// Client wraps *routeros.Client to make it reusable and configurable.
type Client struct {
	*routeros.Client        // embedded → all default methods available!
	Config           Config // Expose config for creating new instances
}

// NewClient creates and returns a new MikroTik client.
func NewClient(cfg Config) (*Client, error) {
	client := &Client{Config: cfg}
	if err := client.connect(); err != nil {
		return nil, err
	}
	return client, nil
}

func (c *Client) connect() error {
	address := fmt.Sprintf("%s:%d", c.Config.Host, c.Config.Port)

	var (
		conn *routeros.Client
		err  error
	)

	if c.Config.Timeout > 0 {
		ctx, cancel := context.WithTimeout(context.Background(), c.Config.Timeout)
		defer cancel()

		if c.Config.UseTLS {
			conn, err = routeros.DialTLSContext(ctx, address, c.Config.Username, c.Config.Password, nil)
		} else {
			conn, err = routeros.DialContext(ctx, address, c.Config.Username, c.Config.Password)
		}
	} else {
		// WITHOUT CONTEXT (SAFE)
		if c.Config.UseTLS {
			conn, err = routeros.DialTLS(address, c.Config.Username, c.Config.Password, nil)
		} else {
			conn, err = routeros.Dial(address, c.Config.Username, c.Config.Password)
		}
	}

	if err != nil {
		return fmt.Errorf("failed to connect to MikroTik: %w", err)
	}

	if c.Config.Queue > 0 {
		conn.Queue = c.Config.Queue
	}

	c.Client = conn
	return nil
}

// Reconnect attempts to re-establish the connection
func (c *Client) Reconnect() error {
	if c.Client != nil {
		c.Client.Close()
	}
	return c.connect()
}

// Run overrides routeros.Client.Run with auto-reconnection support
func (c *Client) Run(sentence ...string) (*routeros.Reply, error) {
	reply, err := c.Client.Run(sentence...)
	if err != nil {
		if isConnectionError(err) {
			// Try to reconnect
			if recErr := c.Reconnect(); recErr == nil {
				// Retry command
				return c.Client.Run(sentence...)
			}
		}
		return nil, err
	}
	return reply, nil
}

func isConnectionError(err error) bool {
	msg := err.Error()
	return strings.Contains(msg, "loop has ended") ||
		strings.Contains(msg, "closed network connection") ||
		strings.Contains(msg, "broken pipe") ||
		strings.Contains(msg, "use of closed network connection") ||
		strings.Contains(msg, "EOF")
}
