# Quick Start Guide - Customer Monitor

## Prasyarat

1. **Database PostgreSQL** dengan tabel `customers` yang sudah terisi
2. **Redis Server** yang running
3. **MikroTik Router** dengan PPPoE server aktif
4. **.env** file sudah dikonfigurasi dengan benar

## Langkah 1: Persiapan Database

Pastikan Anda punya customer di database dengan `service_type = 'pppoe'` dan `pppoe_username` terisi:

```sql
-- Contoh insert customer untuk testing
INSERT INTO customers (id, username, name, service_type, pppoe_username, status)
VALUES (
    '123e4567-e89b-12d3-a456-426614174000',
    'customer1',
    'Test Customer 1',
    'pppoe',
    'tes',
    'active'
);
```

> **Note:** `pppoe_username` harus sama dengan username yang ada di MikroTik PPPoE Secret

## Langkah 2: Start Aplikasi

```bash
cd c:\Users\masji\web\MikroBill-TA\mikrobill\telgraf-mikrotik

# Jalankan aplikasi
go run .

# Atau gunakan binary yang sudah di-build
.\mikrotik-collector.exe
```

**Expected Output:**
```
=== MikroTik Traffic Monitor ===
Config: MikroTik=192.168.100.1:8728, Redis=localhost:6379, WS Port=8081, DB=localhost:5432
Connected to MikroTik successfully
Redis Stream Consumer connected to localhost:6379
Database connected successfully
Customer traffic service initialized (max concurrent: 50)
Starting Redis Stream consumer for stream: mikrotik:traffic:customers
Traffic monitor API routes registered:
  POST /api/monitor/start?customer_id=xxx
  POST /api/monitor/stop?customer_id=xxx
  GET  /api/monitor/status
WebSocket server started on :8081
- ws://localhost:8081/ws
- http://localhost:8081/health
```

## Langkah 3: Buka Dashboard HTML

1. Buka file `customer-monitor.html` di browser Anda
2. WebSocket akan otomatis connect ke `ws://localhost:8081/ws`

![Dashboard Initial State](file:///C:/Users/masji/.gemini/antigravity/brain/c2ca4130-97de-4099-bd36-8dd03bdb0bd3/initial_dashboard_state_1766944176269.png)

**Status Indicators:**
- ✅ **WebSocket Status**: Connected (hijau) / Disconnected (merah)
- 📊 **Active Monitors**: Jumlah customer yang sedang dimonitor
- 🎯 **Available Slots**: Slot monitoring yang masih tersedia

## Langkah 4: Pilih Customer

Klik pada customer di list sebelah kiri. Customer yang dipilih akan menampilkan informasi detail:

![Selected Customer](file:///C:/Users/masji/.gemini/antigravity/brain/c2ca4130-97de-4099-bd36-8dd03bdb0bd3/selected_customer_view_1766944204886.png)

**Customer Information:**
- Name
- Username
- Service Type (PPPoE)
- PPPoE Username
- Status

## Langkah 5: Start Monitoring

Klik tombol **"Start Monitoring"**

**What happens:**
1. API call ke `POST /api/monitor/start?customer_id=xxx`
2. Service akan:
   - Query customer dari database
   - Mendapat `pppoe_username` (contoh: `tes`)
   - Mulai monitor interface `<tes>` di MikroTik
   - Stream data ke Redis Stream
3. WebSocket menerima data dan update UI real-time

**Expected Display:**
```
⬇️ Download Speed: 2.34 Mbps
⬆️ Upload Speed: 1.12 Mbps
📥 RX Packets/s: 245
📤 TX Packets/s: 189
Last updated: 11:23:45 PM
```

## Langkah 6: Stop Monitoring

Klik tombol **"Stop Monitoring"** untuk menghentikan monitoring customer tersebut.

## Testing dengan cURL

### Start Monitoring
```bash
curl -X POST "http://localhost:8081/api/monitor/start?customer_id=123e4567-e89b-12d3-a456-426614174000"
```

**Response:**
```json
{
  "status": "success",
  "message": "Monitoring started",
  "customer_id": "123e4567-e89b-12d3-a456-426614174000"
}
```

### Check Status
```bash
curl http://localhost:8081/api/monitor/status
```

**Response:**
```json
{
  "status": "ok",
  "active_monitors": ["123e4567-e89b-12d3-a456-426614174000"],
  "monitor_count": 1,
  "max_concurrent": 50,
  "slots_available": 49
}
```

### Stop Monitoring
```bash
curl -X POST "http://localhost:8081/api/monitor/stop?customer_id=123e4567-e89b-12d3-a456-426614174000"
```

### Health Check
```bash
curl http://localhost:8081/health
```

## Troubleshooting

### Problem: "customer not found"
**Solution:**
- Pastikan customer ID ada di database
- Query: `SELECT id, name, pppoe_username FROM customers WHERE id = '...'`

### Problem: "pppoe username not set"
**Solution:**
- Customer harus punya `pppoe_username` di database
- Update: `UPDATE customers SET pppoe_username = 'username' WHERE id = '...'`

### Problem: WebSocket tidak connect
**Solution:**
1. Check aplikasi sudah running
2. Check port 8081 tidak dipakai aplikasi lain
3. Buka browser console (F12) untuk lihat error

### Problem: Data tidak muncul di dashboard
**Solution:**
1. Check customer sudah login ke PPPoE MikroTik
2. Verify interface `<pppoe_username>` ada di MikroTik:
   ```bash
   /interface print where name="<username>"
   ```
3. Check Redis Stream ada data:
   ```bash
   redis-cli XREAD COUNT 10 STREAMS mikrotik:traffic:customers 0
   ```

### Problem: "failed to start monitor-traffic"
**Solution:**
- PPPoE user belum login/connect ke MikroTik
- Interface `<pppoe_username>` belum ada
- MikroTik connection terputus

## Cara Mudah Load Customer dari Database

Edit file `customer-monitor.html`, ganti fungsi `loadCustomers()` untuk fetch dari API database Anda:

```javascript
async function loadCustomers() {
    try {
        // Ganti dengan endpoint API database Anda
        const response = await fetch('http://your-api/customers?service_type=pppoe&status=active');
        customers = await response.json();
        renderCustomerList();
    } catch (error) {
        console.error('Error loading customers:', error);
        // Fallback to mock data
        customers = [
            // ...
        ];
        renderCustomerList();
    }
}
```

## Monitor Multiple Customers

Anda bisa monitor hingga 50 customer secara bersamaan (configurable via `MAX_CONCURRENT_MONITORS`):

1. Select customer pertama → Start monitoring
2. Select customer kedua → Start monitoring
3. Dst...

Dashboard akan menampilkan traffic untuk customer yang **sedang dipilih**. Semua customer yang dimonitor tetap streaming data ke Redis, tapi UI hanya show yang selected.

## Video Demo

Lihat recording lengkap cara menggunakan dashboard:

![Dashboard Demo](file:///C:/Users/masji/.gemini/antigravity/brain/c2ca4130-97de-4099-bd36-8dd03bdb0bd3/html_monitor_demo_1766944160888.webp)

## Next Steps

- [ ] Integrate dengan API customer list Anda
- [ ] Tambahkan filter/search customers
- [ ] Tambahkan grafik historical data
- [ ] Export data ke CSV
- [ ] Notifikasi jika bandwidth melebihi limit
