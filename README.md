# MikroTik PPPoE Traffic Monitor - Refactored

## Overview

Aplikasi ini memonitor traffic MikroTik PPPoE secara real-time menggunakan:
- **MikroTik API** dengan `/interface/monitor-traffic` (tanpa polling)
- **PostgreSQL** untuk metadata pelanggan
- **Redis Stream** untuk publish data traffic yang sudah diperkaya
- **WebSocket** untuk update real-time ke browser

## Arsitektur Baru

```
telgraf-mikrotik/
├── internal/
│   ├── infrastructure/
│   │   └── mikrotik/
│   │       ├── client.go       # MikroTik client wrapper
│   │       └── monitor.go      # Traffic monitoring logic
│   └── application/
│       └── services/
│           └── customer_traffic_service.go  # Business logic
├── main.go                     # Application entry point
├── config.go                   # Configuration management
├── database.go                 # Database operations
├── redis_publisher.go          # Redis Stream publisher
├── redis_stream_consumer.go    # Redis Stream consumer
├── traffic_monitor_handler.go  # HTTP API handlers
└── customer-monitor.html       # Frontend dashboard
```

## Cara Kerja

### 1. On-Demand Monitoring
- Start monitoring via API: `POST /api/monitor/start?customer_id=xxx`
- Service akan:
  1. Query customer dari database
  2. Dapatkan PPPoE username
  3. Start `/interface/monitor-traffic` untuk interface `<pppoe-username>`
  4. Enrich data traffic dengan metadata customer
  5. Publish ke Redis Stream

### 2. Data Flow
```
MikroTik API → MonitorTraffic() → Customer Service → Redis Stream → WebSocket → Browser
```

### 3. Redis Stream
- Stream key: `mikrotik:traffic:customers`
- Consumer group: `websocket-broadcasters`
- Data format: JSON dengan customer metadata + traffic stats

## Setup

### 1. Environment Variables

Sesuaikan `.env`:
```bash
# MikroTik
MIKROTIK_HOST=192.168.100.1
MIKROTIK_PORT=8728
MIKROTIK_USER=admin
MIKROTIK_PASS=r00t

# Redis
REDIS_ADDR=localhost:6379
REDIS_PASS=
REDIS_DB=0

# WebSocket
WS_PORT=8081

# Database
DB_HOST=localhost
DB_PORT=5432
DB_USER=root
DB_PASSWORD=r00t
DB_NAME=mikrobill_test
DB_SSLMODE=disable

# Traffic Monitor
ENABLE_TRAFFIC_MONITOR=true
MAX_CONCURRENT_MONITORS=50
```

### 2. Database Schema

Tabel `customers` harus memiliki kolom:
```sql
CREATE TABLE customers (
    id VARCHAR PRIMARY KEY,
    username VARCHAR,
    name VARCHAR,
    service_type VARCHAR,  -- 'pppoe', 'hotspot', 'static_ip'
    pppoe_username VARCHAR,
    ...
);
```

### 3. Build & Run

```bash
# Build
go mod tidy
go build -o mikrotik-collector.exe .

# Run
.\mikrotik-collector.exe
```

## API Endpoints

### Start Monitoring
```bash
POST http://localhost:8081/api/monitor/start?customer_id=<ID>
```

Response:
```json
{
  "status": "success",
  "message": "Monitoring started",
  "customer_id": "123"
}
```

### Stop Monitoring
```bash
POST http://localhost:8081/api/monitor/stop?customer_id=<ID>
```

### Monitor Status
```bash
GET http://localhost:8081/api/monitor/status
```

Response:
```json
{
  "status": "ok",
  "active_monitors": ["123", "456"],
  "monitor_count": 2,
  "max_concurrent": 50,
  "slots_available": 48
}
```

### Health Check
```bash
GET http://localhost:8081/health
```

### WebSocket
```
ws://localhost:8081/ws
```

## Perubahan dari Versi Lama

### Dihapus
- ❌ `mikrotik_client.go` - Diganti dengan `internal/infrastructure/mikrotik/client.go`
- ❌ `customer_traffic_monitor.go` - Diganti dengan service pattern
- ❌ `redis_subscriber.go` - Diganti dengan Redis Stream consumer
- ❌ `telegraf.conf` - Tidak digunakan
- ❌ `docker-compose.yaml` - Setup Telegraf tidak diperlukan
- ❌ Interval-based polling - Sekarang menggunakan streaming

### Ditambahkan
- ✅ Clean architecture dengan `internal/` packages
- ✅ MikroTik client yang lebih sederhana (sesuai contoh user)
- ✅ Redis Stream untuk reliable messaging
- ✅ Service pattern untuk business logic
- ✅ Repository pattern untuk database access

### Disederhanakan
- Config: Hapus interval settings yang tidak terpakai
- Main: Lebih fokus, tanpa auto-reconnect logic yang kompleks
- Error handling: Lebih jelas dan konsisten

## Monitoring PPPoE Interface

Untuk PPPoE customer, interface name format:
```
<pppoe-username>
```

Contoh:
- Customer dengan `pppoe_username = "user123"`
- Interface yang dimonitor: `<user123>`
- MikroTik command: `/interface/monitor-traffic interface=<user123>`

## Tips Development

### Test MikroTik Connection
```go
client, err := mikrotik.NewClient(mikrotik.Config{...})
if err != nil {
    log.Fatal(err)
}
defer client.Close()
```

### Test Monitor Traffic
```go
ctx := context.Background()
trafficChan, err := mikrotik.MonitorTraffic(ctx, client, "<username>")
for traffic := range trafficChan {
    fmt.Printf("RX: %s bps, TX: %s bps\n", 
        traffic.RxBitsPerSecond, 
        traffic.TxBitsPerSecond)
}
```

### Test Redis Stream
```bash
# Read from stream
redis-cli XREAD COUNT 10 STREAMS mikrotik:traffic:customers 0

# Check consumer group
redis-cli XINFO GROUPS mikrotik:traffic:customers
```

## Troubleshooting

### Error: "customer not found"
- Pastikan customer ada di database
- Check customer ID yang dikirim ke API

### Error: "pppoe username not set"
- Customer harus memiliki `pppoe_username` di database
- Hanya support PPPoE untuk saat ini

### Error: "max concurrent monitors reached"
- Tingkatkan `MAX_CONCURRENT_MONITORS` di .env
- Atau stop beberapa monitor yang tidak diperlukan

### WebSocket tidak menerima data
- Check Redis Stream consumer sudah running
- Verify data ada di stream: `redis-cli XREAD ...`
- Check browser console untuk error WebSocket

## Lisensi

Internal project untuk MikroBill.
