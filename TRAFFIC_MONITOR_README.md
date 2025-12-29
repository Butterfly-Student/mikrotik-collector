# MikroTik Customer Traffic Monitor

Real-time, on-demand customer bandwidth monitoring untuk MikroTik RouterOS menggunakan `/interface/monitor-traffic`.

## Features

✅ **On-Demand Monitoring** - Monitor traffic hanya untuk customer yang dipilih (bukan semua customer sekaligus)
✅ **Database Integration** - Customer data disimpan di PostgreSQL
✅ **Real-Time Traffic Stats** - Update setiap detik menggunakan MikroTik Listen API  
✅ **Redis Stream** - Reliable event delivery (at-least-once)
✅  **WebSocket Support** - Live dashboard updates
✅ **REST API** - HTTP endpoints untuk start/stop monitoring
✅ **Customer Metadata** - Traffic data includes customer info (name, username, package, etc)

## Prerequisites

- Go 1.21+
- PostgreSQL 12+
- Redis 6.0+
- MikroTik RouterOS dengan API access

## Quick Start

### 1. Setup Database

```bash
# Create database
createdb mikrobill_traffic

# Run migration
psql -d mikrobill_traffic -f migrations/001_create_customers.sql
```

### 2. Configure Environment

Copy `.env.example` to `.env` dan sesuaikan:

```env
# MikroTik
MIKROTIK_HOST=192.168.1.1
MIKROTIK_PORT=8728
MIKROTIK_USER=admin
MIKROTIK_PASS=yourpassword

# Database
DB_HOST=localhost
DB_PORT=5432
DB_USER=postgres
DB_PASSWORD=yourpassword
DB_NAME=mikrobill_traffic

# Redis
REDIS_ADDR=localhost:6379

# Traffic Monitor
ENABLE_TRAFFIC_MONITOR=true
MAX_CONCURRENT_MONITORS=50
```

### 3. Build & Run

```bash
go mod download
go build
./mikrotik-collector.exe
```

## API Endpoints

### Start Monitoring
```http
POST /api/monitor/start?customer_id=<uuid>
```

Response:
```json
{
  "status": "success",
  "message": "Monitoring started",
  "customer_id": "123e4567-e89b-12d3-a456-426614174000"
}
```

### Stop Monitoring
```http
POST /api/monitor/stop?customer_id=<uuid>
```

### Get Status
```http
GET /api/monitor/status
```

Response:
```json
{
  "status": "ok",
  "active_monitors": ["customer-id-1", "customer-id-2"],
  "monitor_count": 2,
  "max_concurrent": 50,
  "slots_available": 48
}
```

## WebSocket

Connect to `ws://localhost:8081/ws` untuk real-time traffic updates.

Message format:
```json
{
  "customer_id": "123e4567...",
  "customer_name": "Test Customer",
  "username": "test-user",
  "service_type": "pppoe",
  "interface": "<pppoe-test-user>",
  "rx_bits_per_second": "5234567",
  "tx_bits_per_second": "1234567",
  "download_speed": "5.23 Mbps",
  "upload_speed": "1.23 Mbps",
  "timestamp": "2025-12-28T23:15:00Z"
}
```

## Database Schema

Table `customers` dengan fields:
- `id` - UUID primary key
- `username` - Customer username
- `name` - Customer display name
- `service_type` - pppoe / hotspot / static_ip
- `pppoe_username` - PPPoE username (untuk mapping ke interface)
- `status` - active / suspended / inactive

See `migrations/001_create_customers.sql` untuk schema lengkap.

## Architecture

```
Customer Request (HTTP API)
    ↓
CustomerTrafficMonitor.StartMonitoring(customerID)
    ↓
Query customer dari database
    ↓
Get interface name (<pppoe-username>)
    ↓
MikroTik /interface/monitor-traffic (Listen API)
    ↓
Traffic data enriched dengan customer metadata
    ↓
Publish ke Redis Stream (mikrotik:traffic:customers)
    ↓
WebSocket broadcast ke connected clients
```

## Example: Add Sample Customer

```sql
INSERT INTO customers (
    mikrotik_id, username, name, service_type, 
    pppoe_username, pppoe_password, status
)
VALUES (
    'mikrotik-001',
    'customer1', 
    'Test Customer 1',
    'pppoe',
    'test-pppoe-1',
    'password123',
    'active'
);
```

## Testing

1. **Start monitoring:**
```bash
curl -X POST "http://localhost:8081/api/monitor/start?customer_id=<your-customer-id>"
```

2. **Open WebSocket test client:**
```
file:///c:/Users/masji/web/MikroBill-TA/mikrobill/telgraf-mikrotik/test-client.html
```

3. **Generate traffic** dari customer PPPoE connection

4. **Observe** real-time traffic updates di WebSocket client

## Troubleshooting

**Error: "customer not found"**
- Check customer exists di database dengan status 'active'
- Verify customer_id is valid UUID

**Error: "MikroTik client not connected"**
- Check MikroTik host/port/credentials di .env
- Test connection: `telnet 192.168.1.1 8728`

**Error: "failed to get interface name"**
- For PPPoE: customer harus punya `pppoe_username` field filled
- Interface name format: `<pppoe-username>`

**No traffic data received:**
- Ensure customer is actually connected (active PPPoE session)
- Check MikroTik `/ppp/active/print` for active session
- Generate traffic (download/upload) from customer

## License

MIT
