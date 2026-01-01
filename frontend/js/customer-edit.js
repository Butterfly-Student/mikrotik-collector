// Customer edit form logic for Edit Customer page

const form = document.getElementById('edit-customer-form');
const typeSelect = document.getElementById('service-type-select');
const pppoeFields = document.getElementById('pppoe-fields');
const loading = document.getElementById('loading-msg');

// Get ID from URL query parameter
const urlParams = new URLSearchParams(window.location.search);
const customerId = urlParams.get('id');

if (!customerId) {
    loading.innerHTML = '<span style="color:red">No customer ID provided</span>';
} else {
    document.getElementById('customer-id').value = customerId;

    typeSelect.addEventListener('change', () => {
        if (typeSelect.value === 'pppoe') {
            pppoeFields.style.display = 'block';
        } else {
            pppoeFields.style.display = 'none';
        }
    });

    // Fetch customer data
    fetch(`${API_BASE}/api/customers/${customerId}`)
        .then(r => r.json())
        .then(resp => {
            if (resp.status === 'success') {
                const c = resp.data;
                document.getElementById('f-name').value = c.name;
                document.getElementById('f-username').value = c.username;
                document.getElementById('service-type-select').value = c.service_type;
                document.getElementById('f-pppoe-username').value = c.pppoe_username || '';
                document.getElementById('f-pppoe-profile').value = c.pppoe_profile || '';
                document.getElementById('f-phone').value = c.phone || '';
                document.getElementById('f-email').value = c.email || '';

                if (c.service_type === 'pppoe') {
                    pppoeFields.style.display = 'block';
                } else {
                    pppoeFields.style.display = 'none';
                }

                form.style.display = 'block';
                loading.style.display = 'none';
            } else {
                loading.innerHTML = '<span style="color:red">Customer not found</span>';
            }
        })
        .catch(err => {
            console.error(err);
            loading.innerHTML = '<span style="color:red">Error loading customer</span>';
        });

    form.addEventListener('submit', async (e) => {
        e.preventDefault();

        const formData = new FormData(form);
        const data = Object.fromEntries(formData.entries());

        const payload = {
            name: data.name,
            username: data.username,
            service_type: data.service_type,
            phone: data.phone || null,
            email: data.email || null,
            pppoe_username: data.pppoe_username || null,
            pppoe_profile: data.pppoe_profile || null
        };

        if (data.pppoe_password) {
            payload.pppoe_password = data.pppoe_password;
        }

        try {
            const res = await fetch(`${API_BASE}/api/customers/${customerId}`, {
                method: 'PUT',
                headers: {
                    'Content-Type': 'application/json'
                },
                body: JSON.stringify(payload)
            });

            const result = await res.json();

            if (res.ok && result.status === 'success') {
                alert('Customer updated successfully!');
                window.location.href = '/?customer_id=' + customerId;
            } else {
                alert('Error: ' + (result.message || 'Unknown error'));
            }
        } catch (err) {
            console.error(err);
            alert('Failed to update form');
        }
    });

    document.getElementById('delete-btn').addEventListener('click', async () => {
        if (confirm('Are you sure you want to delete this customer? This action cannot be undone.')) {
            try {
                const res = await fetch(`${API_BASE}/api/customers/${customerId}`, {
                    method: 'DELETE'
                });

                if (res.ok) {
                    alert('Customer deleted');
                    window.location.href = '/';
                } else {
                    alert('Failed to delete');
                }
            } catch (e) {
                alert('Error deleting');
            }
        }
    });
}