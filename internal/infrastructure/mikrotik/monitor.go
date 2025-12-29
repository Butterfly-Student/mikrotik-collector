package mikrotik

import (
	"context"
)

// InterfaceTraffic represents traffic data from MikroTik monitor-traffic command
type InterfaceTraffic struct {
	Name string

	RxPacketsPerSecond   string
	RxBitsPerSecond      string
	FpRxPacketsPerSecond string
	FpRxBitsPerSecond    string

	RxDropsPerSecond  string
	RxErrorsPerSecond string

	TxPacketsPerSecond   string
	TxBitsPerSecond      string
	FpTxPacketsPerSecond string
	FpTxBitsPerSecond    string

	TxDropsPerSecond      string
	TxQueueDropsPerSecond string
	TxErrorsPerSecond     string

	Section string
}

// MonitorTraffic starts monitoring traffic for a specific interface
// Returns a channel that receives traffic data continuously until context is cancelled
func MonitorTraffic(
	ctx context.Context,
	client *Client,
	iface string,
) (<-chan InterfaceTraffic, error) {

	reply, err := client.ListenArgsContext(ctx, []string{
		"/interface/monitor-traffic",
		"=interface=" + iface,
	})
	if err != nil {
		if isConnectionError(err) {
			// Try to reconnect
			if recErr := client.Reconnect(); recErr == nil {
				// Retry command
				reply, err = client.ListenArgsContext(ctx, []string{
					"/interface/monitor-traffic",
					"=interface=" + iface,
				})
			}
		}
	}
	if err != nil {
		return nil, err
	}

	out := make(chan InterfaceTraffic)

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

				out <- mapToInterfaceTraffic(r.Map)
			}
		}
	}()

	return out, nil
}

// mapToInterfaceTraffic converts MikroTik response map to InterfaceTraffic struct
func mapToInterfaceTraffic(m map[string]string) InterfaceTraffic {
	return InterfaceTraffic{
		Name: m["name"],

		RxPacketsPerSecond:   m["rx-packets-per-second"],
		RxBitsPerSecond:      m["rx-bits-per-second"],
		FpRxPacketsPerSecond: m["fp-rx-packets-per-second"],
		FpRxBitsPerSecond:    m["fp-rx-bits-per-second"],

		RxDropsPerSecond:  m["rx-drops-per-second"],
		RxErrorsPerSecond: m["rx-errors-per-second"],

		TxPacketsPerSecond:   m["tx-packets-per-second"],
		TxBitsPerSecond:      m["tx-bits-per-second"],
		FpTxPacketsPerSecond: m["fp-tx-packets-per-second"],
		FpTxBitsPerSecond:    m["fp-tx-bits-per-second"],

		TxDropsPerSecond:      m["tx-drops-per-second"],
		TxQueueDropsPerSecond: m["tx-queue-drops-per-second"],
		TxErrorsPerSecond:     m["tx-errors-per-second"],

		Section: m[".section"],
	}
}
