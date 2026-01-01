// Dashboard Logic
let ws = null;
let trafficWs = null;
let selectedCustomerLocal = null;
let currentTrafficData = {};

// Hardcoded for development to avoid port mismatch with Live Server
const WS_URL = 'ws://localhost:8082/ws';
const API_BASE_URL = 'http://localhost:8082';

// Global select function for sidebar to call
window.selectCustomer = function (customer) {
    selectedCustomerLocal = customer;

    // Close existing traffic WebSocket if any
    if (trafficWs) {
        trafficWs.close();
        trafficWs = null;
    }

    // Render initial view
    renderInitialDetailView();

    // Start traffic monitoring for this customer if they're active
    if (customer.status === 'active') {
        connectTrafficWebSocket(customer.id);
    }
};

function connectWebSocket() {
    ws = new WebSocket(WS_URL);

    ws.onopen = () => {
        console.log('Connected to WebSocket');
        updateConnectionStatus(true);
    };

    ws.onclose = () => {
        console.log('WebSocket disconnected');
        updateConnectionStatus(false);
        setTimeout(connectWebSocket, 3000);
    };

    ws.onmessage = (event) => {
        try {
            const data = JSON.parse(event.data);

            // Handle different message types
            if (data.type === 'customer_list') {
                console.log('Received customer list:', data.data);
            } else if (data.type === 'traffic_update') {
                handleTrafficUpdate(data.data);
            }
        } catch (e) {
            console.error('Error parsing WS message', e);
        }
    };
}

function connectTrafficWebSocket(customerId) {
    const trafficWsUrl = `ws://localhost:8082/api/customers/${customerId}/traffic/ws`;
    trafficWs = new WebSocket(trafficWsUrl);

    trafficWs.onopen = () => {
        console.log('Connected to traffic stream for customer:', customerId);
    };

    trafficWs.onclose = () => {
        console.log('Traffic WebSocket closed');
    };

    trafficWs.onmessage = (event) => {
        try {
            const msg = JSON.parse(event.data);
            if (msg.type === 'traffic_update' && msg.data) {
                handleTrafficUpdate(msg.data);
            } else if (msg.type === 'error') {
                console.error('Traffic stream error:', msg.message);
                showTrafficError(msg.message);
            }
        } catch (e) {
            console.error('Error parsing traffic WS message', e);
        }
    };

    trafficWs.onerror = (error) => {
        console.error('Traffic WebSocket error:', error);
    };
}

function handleTrafficUpdate(data) {
    if (!data.customer_id) return;

    currentTrafficData[data.customer_id] = data;

    if (selectedCustomerLocal && selectedCustomerLocal.id === data.customer_id) {
        renderTrafficView(data);
    }
}

function showTrafficError(message) {
    const container = document.getElementById('main-display');
    if (!container) return;

    const c = selectedCustomerLocal;
    const initials = c.name.substring(0, 2).toUpperCase();

    container.innerHTML = `
        <div class="detail-view">
            <div class="customer-header">
                <div class="profile-card">
                    <div class="profile-avatar-large">${initials}</div>
                    <div class="profile-info">
                        <h1>${c.name}</h1>
                        <div class="profile-badges">
                            <span class="badge badge-type">${c.service_type}</span>
                            <span class="badge badge-status-offline">Error</span>
                        </div>
                    </div>
                </div>
                <div class="action-buttons">
                    <a href="/edit-customer.html?id=${c.id}" class="btn btn-secondary">
                        <i class="fa-solid fa-pen"></i> Edit
                    </a>
                    <button class="btn btn-primary" onclick="pingCustomer('${c.id}')">
                        <i class="fa-solid fa-satellite-dish"></i> Ping
                    </button>
                </div>
            </div>

            <div class="empty-state" style="height: 40vh;">
                <i class="fa-solid fa-exclamation-triangle" style="font-size: 3rem; color: var(--danger); margin-bottom: 16px;"></i>
                <h3>Monitoring Error</h3>
                <p>${message}</p>
                <p style="margin-top: 12px; font-size: 0.9rem; color: var(--gray-600);">
                    Make sure the customer has a valid PPPoE username and is currently connected.
                </p>
            </div>
        </div>
    `;
}

function updateConnectionStatus(connected) {
    const text = document.getElementById('ws-status-text');
    const indicator = document.getElementById('ws-indicator');
    if (!text || !indicator) return;

    if (connected) {
        text.textContent = 'Live Connected';
        text.style.color = 'var(--success)';
        indicator.className = 'ws-dot connected';
    } else {
        text.textContent = 'Reconnecting...';
        text.style.color = 'var(--gray-500)';
        indicator.className = 'ws-dot disconnected';
    }
}

async function updateSystemStats() {
    try {
        const baseUrl = typeof API_BASE !== 'undefined' ? API_BASE : 'http://localhost:8082';
        const response = await fetch(`${baseUrl}/api/monitor/status`);
        if (!response.ok) throw new Error(`HTTP error! status: ${response.status}`);
        const data = await response.json();

        const cCount = document.getElementById('total-customers');
        const mCount = document.getElementById('active-monitors');
        if (cCount) cCount.textContent = data.customer_count || 0;
        if (mCount) mCount.textContent = data.monitor_count || 0;
    } catch (error) {
        console.error('Failed to load stats', error);
        const cCount = document.getElementById('total-customers');
        const mCount = document.getElementById('active-monitors');
        if (cCount) cCount.textContent = '0';
        if (mCount) mCount.textContent = '0';
    }
}

// --- Rendering Logic ---

function renderInitialDetailView() {
    if (!selectedCustomerLocal) return;

    const c = selectedCustomerLocal;
    const container = document.getElementById('main-display');
    const initials = c.name.substring(0, 2).toUpperCase();
    const isOnline = c.status === 'active';

    container.innerHTML = `
        <div class="detail-view">
            <div class="customer-header">
                <div class="profile-card">
                    <div class="profile-avatar-large">${initials}</div>
                    <div class="profile-info">
                        <h1>${c.name}</h1>
                        <div class="profile-badges">
                            <span class="badge badge-type">${c.service_type}</span>
                            <span class="badge ${c.status === 'active' ? 'badge-status-online' : 'badge-status-offline'}">
                                ${c.status}
                            </span>
                            ${c.pppoe_username ? `<span class="badge badge-interface">pppoe-${c.pppoe_username}</span>` : ''}
                        </div>
                    </div>
                </div>
                <div class="action-buttons">
                     <a href="/edit-customer.html?id=${c.id}" class="btn btn-secondary">
                        <i class="fa-solid fa-pen"></i> Edit
                     </a>
                     <button class="btn btn-primary" onclick="pingCustomer('${c.id}')">
                        <i class="fa-solid fa-satellite-dish"></i> Ping
                    </button>
                </div>
            </div>

            <div id="ping-result-container"></div>

            ${isOnline ? `
                <div class="empty-state" style="height: 40vh;">
                    <i class="fa-solid fa-circle-notch fa-spin" style="font-size: 2rem; color: var(--primary); margin-bottom: 16px;"></i>
                    <h3>Waiting for Data...</h3>
                    <p>Connecting to live traffic stream for this customer.</p>
                </div>
            ` : `
                <div class="empty-state" style="height: 40vh;">
                    <i class="fa-solid fa-power-off" style="font-size: 3rem; color: var(--gray-300); margin-bottom: 16px;"></i>
                    <h3>Customer Offline</h3>
                    <p>This customer is not currently connected to the network.</p>
                </div>
            `}
        </div>
    `;
}

function renderTrafficView(data) {
    if (!selectedCustomerLocal) return;
    if (data.customer_id && selectedCustomerLocal.id !== data.customer_id) return;

    if (!data.download_speed) {
        renderInitialDetailView();
        return;
    }

    const container = document.getElementById('main-display');

    const dl = data.download_speed.split(' ');
    const ul = data.upload_speed.split(' ');
    const dlVal = dl[0];
    const dlUnit = dl[1] || 'bps';
    const ulVal = ul[0];
    const ulUnit = ul[1] || 'bps';
    const initials = (data.customer_name || selectedCustomerLocal.name).substring(0, 2).toUpperCase();

    // Check if we are already in view (simple check)
    const existingDl = document.getElementById('val-dl');
    if (existingDl && document.getElementById('ping-result-container')) {
        document.getElementById('val-dl').textContent = dlVal;
        document.getElementById('unit-dl').textContent = dlUnit;
        document.getElementById('val-ul').textContent = ulVal;
        document.getElementById('unit-ul').textContent = ulUnit;
        document.getElementById('val-rx').textContent = data.rx_packets_per_second || '0';
        document.getElementById('val-tx').textContent = data.tx_packets_per_second || '0';
        document.getElementById('last-updated').textContent = new Date().toLocaleTimeString();
        return;
    }

    // Full Render
    container.innerHTML = `
        <div class="detail-view">
            <div class="customer-header">
                <div class="profile-card">
                    <div class="profile-avatar-large">${initials}</div>
                    <div class="profile-info">
                        <h1>${data.customer_name || selectedCustomerLocal.name}</h1>
                        <div class="profile-badges">
                            <span class="badge badge-type">${data.service_type || selectedCustomerLocal.service_type}</span>
                            <span class="badge badge-status-online">LIVE MONITORING</span>
                            <span class="badge badge-interface">${data.interface_name || 'N/A'}</span>
                        </div>
                    </div>
                </div>
                <div class="action-buttons">
                    <a href="/edit-customer.html?id=${selectedCustomerLocal.id}" class="btn btn-secondary">
                        <i class="fa-solid fa-pen"></i> Edit
                    </a>
                    <button class="btn btn-primary" onclick="pingCustomer('${data.customer_id}')">
                        <i class="fa-solid fa-satellite-dish"></i> Ping
                    </button>
                </div>
            </div>

            <div id="ping-result-container"></div>

            <div class="traffic-grid">
                <div class="stat-card">
                    <div class="stat-icon icon-download">
                        <i class="fa-solid fa-arrow-down"></i>
                    </div>
                    <div class="stat-label">Download Speed</div>
                    <div class="stat-value">
                        <span id="val-dl">${dlVal}</span>
                        <span class="stat-unit" id="unit-dl">${dlUnit}</span>
                    </div>
                </div>

                <div class="stat-card">
                    <div class="stat-icon icon-upload">
                        <i class="fa-solid fa-arrow-up"></i>
                    </div>
                    <div class="stat-label">Upload Speed</div>
                    <div class="stat-value">
                        <span id="val-ul">${ulVal}</span>
                        <span class="stat-unit" id="unit-ul">${ulUnit}</span>
                    </div>
                </div>

                <div class="stat-card">
                    <div class="stat-icon icon-packet">
                        <i class="fa-solid fa-envelope-open-text"></i>
                    </div>
                    <div class="stat-label">Packets RX / sec</div>
                    <div class="stat-value">
                        <span id="val-rx">${data.rx_packets_per_second || '0'}</span>
                        <span class="stat-unit">pps</span>
                    </div>
                </div>

                <div class="stat-card">
                    <div class="stat-icon icon-packet">
                        <i class="fa-solid fa-paper-plane"></i>
                    </div>
                    <div class="stat-label">Packets TX / sec</div>
                    <div class="stat-value">
                        <span id="val-tx">${data.tx_packets_per_second || '0'}</span>
                        <span class="stat-unit">pps</span>
                    </div>
                </div>
            </div>

            <div class="update-pill">
                <div class="live-dot"></div>
                Last updated: <span id="last-updated">${new Date().toLocaleTimeString()}</span>
            </div>
        </div>
    `;
}

// --- Ping Logic ---
let pingWs = null;
let clientStats = { sent: 0, received: 0 };
let isPinging = false;

function pingCustomer(customerId) {
    const c = selectedCustomerLocal;
    if (!c) return;

    openPingModal(c);
    startPingWs(customerId);
}

window.pingCustomer = pingCustomer;

function openPingModal(customer) {
    document.getElementById('ping-modal').classList.add('active');
    document.getElementById('ping-target-name').textContent = customer.name;

    const out = document.getElementById('ping-output');
    out.innerHTML = `
        <div class="terminal-row terminal-header">
            <span>SEQ</span><span>HOST</span><span>SIZE</span><span>TTL</span><span>TIME</span><span>STATUS</span>
        </div>
    `;
    document.getElementById('ping-summary-container').innerHTML = '';

    const btn = document.getElementById('ping-action-btn');
    btn.textContent = 'Stop Ping';
    btn.className = 'btn btn-primary';
    btn.onclick = togglePing;
    isPinging = true;
}

function closePingModal() {
    if (pingWs) {
        pingWs.close();
        pingWs = null;
    }
    document.getElementById('ping-modal').classList.remove('active');
    isPinging = false;
}
window.closePingModal = closePingModal;

function startPingWs(customerId) {
    if (pingWs) pingWs.close();
    clientStats = { sent: 0, received: 0 };

    const wsUrl = `ws://localhost:8082/api/customers/${customerId}/ping/ws`;
    pingWs = new WebSocket(wsUrl);

    pingWs.onopen = () => console.log("Ping WS Connected");
    pingWs.onmessage = (event) => {
        try {
            const msg = JSON.parse(event.data);
            if (msg.type === 'update') appendPingLine(msg.data);
            else if (msg.type === 'summary') {
                showPingSummary(msg.summary);
                stopPingState();
            } else if (msg.type === 'error') {
                appendErrorLine(msg.error);
                stopPingState();
            }
        } catch (e) {
            console.error("Ping WS parse error", e);
        }
    };
    pingWs.onclose = () => {
        if (isPinging) stopPingState(true);
    };
}

function appendPingLine(data) {
    const out = document.getElementById('ping-output');
    const row = document.createElement('div');
    row.className = 'terminal-row';
    if (data.seq) {
        clientStats.sent++;
        if (data.time || (data.status === '' && data.size)) clientStats.received++;
    }
    let status = data.status || '';
    if (status === 'timeout') row.style.color = '#ff6b6b';
    row.innerHTML = `<span>${data.seq || ''}</span><span>${data.host || ''}</span><span>${data.size || ''}</span><span>${data.ttl || ''}</span><span>${data.time || ''}</span><span>${status}</span>`;
    out.appendChild(row);
    document.getElementById('ping-terminal-body').scrollTop = document.getElementById('ping-terminal-body').scrollHeight;
}

function appendErrorLine(err) {
    const out = document.getElementById('ping-output');
    const row = document.createElement('div');
    row.style.color = '#ef4444';
    row.textContent = `Error: ${err}`;
    out.appendChild(row);
}

function showPingSummary(summary) {
    document.getElementById('ping-summary-container').innerHTML = `
        <div class="ping-summary-text">sent=${summary.sent} received=${summary.received} packet-loss=${summary.packet_loss}</div>
    `;
}

function togglePing() {
    if (isPinging) {
        if (pingWs) pingWs.close();
    } else {
        closePingModal();
    }
}
window.togglePing = togglePing;

function stopPingState(showSummary) {
    isPinging = false;
    const btn = document.getElementById('ping-action-btn');
    if (btn) {
        btn.textContent = 'Close';
        btn.className = 'btn';
        btn.onclick = closePingModal;
    }
    if (showSummary) {
        let loss = 0;
        if (clientStats.sent > 0) loss = ((clientStats.sent - clientStats.received) / clientStats.sent) * 100;
        showPingSummary({
            sent: clientStats.sent,
            received: clientStats.received,
            packet_loss: loss.toFixed(0) + '%'
        });
    }
}

// --- Init ---
connectWebSocket();
updateSystemStats();
setInterval(updateSystemStats, 5000);

// Check specific customer from URL on load
const urlParams = new URLSearchParams(window.location.search);
const initialId = urlParams.get('customer_id');

if (initialId) {
    const baseUrl = typeof API_BASE !== 'undefined' ? API_BASE : 'http://localhost:8082';
    fetch(`${baseUrl}/api/customers/${initialId}`)
        .then(r => r.json())
        .then(d => {
            if (d.status === 'success') {
                selectedCustomerLocal = d.data;
                renderInitialDetailView();

                // Start traffic monitoring if active
                if (d.data.status === 'active') {
                    connectTrafficWebSocket(d.data.id);
                }
            }
        })
        .catch(err => console.error('Failed to load initial customer:', err));
}