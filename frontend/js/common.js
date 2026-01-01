// Common JavaScript utilities and sidebar logic shared across all pages

// API Base URL
const API_BASE = 'http://localhost:8082';

// Load customers in sidebar on every page
document.addEventListener('DOMContentLoaded', () => {
    fetchCustomers();
    updateSystemStats();
});

// Search functionality
let searchTimeout;
const searchInput = document.getElementById('search-input');
if (searchInput) {
    searchInput.addEventListener('input', (e) => {
        clearTimeout(searchTimeout);
        const query = e.target.value.toLowerCase();
        searchTimeout = setTimeout(() => {
            const items = document.querySelectorAll('.customer-item');
            items.forEach(item => {
                const name = item.getAttribute('data-name').toLowerCase();
                if (name.includes(query)) {
                    item.style.display = 'flex';
                } else {
                    item.style.display = 'none';
                }
            });
        }, 300);
    });
}

// Fetch customers from API
function fetchCustomers() {
    const listContainer = document.getElementById('customer-list');
    if (!listContainer) return;

    fetch(`${API_BASE}/api/customers`)
        .then(res => res.json())
        .then(data => {
            if (data.status === 'success') {
                renderSidebarCustomers(data.data);
            }
        })
        .catch(err => {
            console.error("Failed to fetch customers", err);
            listContainer.innerHTML = '<div style="padding: 20px; text-align: center; color: var(--gray-500);">Error loading</div>';
        });
}

// Render customers in sidebar
function renderSidebarCustomers(customers) {
    const container = document.getElementById('customer-list');
    if (!container) return;

    container.innerHTML = '';

    if (!customers || customers.length === 0) {
        container.innerHTML = '<div style="padding: 20px; text-align: center; color: var(--gray-500);">No customers found</div>';
        return;
    }

    customers.forEach(c => {
        // Safety check
        if (!c || !c.id || !c.name) return;

        const el = document.createElement('a');
        el.className = 'customer-item';
        el.href = 'index.html?customer_id=' + c.id;
        el.setAttribute('data-id', c.id);
        el.setAttribute('data-name', c.name);

        // Check if active (if we are on dashboard with this ID)
        const urlParams = new URLSearchParams(window.location.search);
        if (urlParams.get('customer_id') === c.id) {
            el.classList.add('active');
        }

        el.innerHTML = `
            <div class="customer-avatar">
               ${c.name.substring(0, 2).toUpperCase()}
            </div>
            <div class="customer-info">
                <div class="customer-name">${c.name}</div>
                <div class="customer-details">
                   <span class="status-dot ${c.status === 'active' ? 'online' : 'offline'}"></span>
                   ${c.status || 'Unknown'} - ${c.service_type || 'N/A'}
                </div>
            </div>
        `;

        // Allow dashboard to intercept if we are on dashboard
        el.addEventListener('click', (e) => {
            if (window.location.pathname === '/' || window.location.pathname === '/index.html') {
                e.preventDefault();
                // Check if 'selectCustomer' is globally defined (dashboard logic)
                if (typeof selectCustomer === 'function') {
                    selectCustomer(c);
                    // Update active state manually
                    document.querySelectorAll('.customer-item').forEach(i => i.classList.remove('active'));
                    el.classList.add('active');

                    // Update URL without reload
                    const newUrl = new URL(window.location);
                    newUrl.searchParams.set('customer_id', c.id);
                    window.history.pushState({}, '', newUrl);
                } else {
                    window.location.href = '/?customer_id=' + c.id;
                }
            }
        });

        container.appendChild(el);
    });
}

// Update system stats in topbar
async function updateSystemStats() {
    try {
        const response = await fetch(`${API_BASE}/api/monitor/status`);
        if (!response.ok) throw new Error(`HTTP error! status: ${response.status}`);
        const data = await response.json();

        const cCount = document.getElementById('total-customers');
        const mCount = document.getElementById('active-monitors');
        if (cCount) cCount.textContent = data.customer_count || 0;
        if (mCount) mCount.textContent = data.monitor_count || 0;
    } catch (error) {
        console.error('Failed to load stats', error);
    }
}

// Update specific customer status in sidebar (Real-time)
function updateCustomerStatusInSidebar(customerId, status) {
    const item = document.querySelector(`.customer-item[data-id="${customerId}"]`);
    if (!item) return;

    // Map backend status to frontend status if needed
    // Backend sends: "connected", "disconnected"
    // Frontend uses: "active", "inactive"
    let displayStatus = status;
    let cssClass = 'offline';

    if (status === 'connected' || status === 'active') {
        displayStatus = 'active';
        cssClass = 'online';
    } else if (status === 'disconnected' || status === 'inactive') {
        displayStatus = 'inactive';
        cssClass = 'offline';
    }

    // Update status dot
    const dot = item.querySelector('.status-dot');
    if (dot) {
        dot.className = `status-dot ${cssClass}`;
    }

    // Update status text
    const details = item.querySelector('.customer-details');
    if (details) {
        // preserve service type if possible, or just update status
        // Text is likely: "active - pppoe"
        const text = details.textContent;
        const parts = text.split('-');
        let serviceType = parts.length > 1 ? parts[1].trim() : 'N/A';
        details.innerHTML = `<span class="status-dot ${cssClass}"></span> ${displayStatus} - ${serviceType}`;
    }
}
window.updateCustomerStatusInSidebar = updateCustomerStatusInSidebar;