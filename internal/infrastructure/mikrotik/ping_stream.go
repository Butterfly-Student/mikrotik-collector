package mikrotik

import (
	"context"
)

// PingResponse represents a single ping response from MikroTik
type PingResponse struct {
	Seq        string `json:"seq"`
	Host       string `json:"host"`
	Size       string `json:"size"`
	TTL        string `json:"ttl"`
	Time       string `json:"time"`
	Status     string `json:"status"` // "timeout", "net-unreachable", etc.
	Sent       string `json:"sent"`
	Received   string `json:"received"`
	PacketLoss string `json:"packet_loss"`
	AvgRtt     string `json:"avg_rtt"`
	MinRtt     string `json:"min_rtt"`
	MaxRtt     string `json:"max_rtt"`
	IsSummary  bool   `json:"is_summary"` // Helper to identify summary packet if any
}

// StreamPing starts a continuous ping to the specified address
// Returns a channel that receives ping data continuously until context is cancelled
func (c *Client) StreamPing(
	ctx context.Context,
	address string,
	size string,
	interval string,
) (<-chan PingResponse, error) {

	args := []string{
		"/ping",
		"=address=" + address,
	}

	if size != "" {
		args = append(args, "=size="+size)
	}
	if interval != "" {
		args = append(args, "=interval="+interval)
	}

	reply, err := c.ListenArgsContext(ctx, args)
	if err != nil {
		if isConnectionError(err) {
			// Try to reconnect
			if recErr := c.Reconnect(); recErr == nil {
				// Retry command
				reply, err = c.ListenArgsContext(ctx, args)
			}
		}
	}

	if err != nil {
		return nil, err
	}

	out := make(chan PingResponse)

	go func() {
		defer close(out)

		for {
			select {
			case <-ctx.Done():
				return
			case r, ok := <-reply.Chan():
				if !ok {
					return
				}
				if r == nil || r.Map == nil {
					continue
				}
				out <- mapToPingResponse(r.Map)
			}
		}
	}()

	return out, nil
}

func mapToPingResponse(m map[string]string) PingResponse {
	// Check if it's a summary packet (usually has 'sent' and 'received' but no 'seq')
	// Normal ping response has 'seq'
	isSummary := false
	if _, hasSeq := m["seq"]; !hasSeq {
		if _, hasSent := m["sent"]; hasSent {
			isSummary = true
		}
	}

	return PingResponse{
		Seq:        m["seq"],
		Host:       m["host"],
		Size:       m["size"],
		TTL:        m["ttl"],
		Time:       m["time"],
		Status:     m["status"],
		Sent:       m["sent"],
		Received:   m["received"],
		PacketLoss: m["packet-loss"],
		AvgRtt:     m["avg-rtt"],
		MinRtt:     m["min-rtt"],
		MaxRtt:     m["max-rtt"],
		IsSummary:  isSummary,
	}
}
