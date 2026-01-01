// Customer form logic for Add Customer page

const form = document.getElementById('add-customer-form');
const typeSelect = document.getElementById('service-type-select');
const pppoeFields = document.getElementById('pppoe-fields');

typeSelect.addEventListener('change', () => {
    if (typeSelect.value === 'pppoe') {
        pppoeFields.style.display = 'block';
    } else {
        pppoeFields.style.display = 'none';
    }
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
        pppoe_password: data.pppoe_password || null,
        pppoe_profile: data.pppoe_profile || null
    };

    try {
        const res = await fetch(`${API_BASE}/api/customers`, {
            method: 'POST',
            headers: {
                'Content-Type': 'application/json'
            },
            body: JSON.stringify(payload)
        });

        const result = await res.json();

        if (res.ok && result.status === 'success') {
            alert('Customer created successfully!');
            window.location.href = '/';
        } else {
            alert('Error: ' + (result.message || 'Unknown error'));
        }
    } catch (err) {
        console.error(err);
        alert('Failed to submit form');
    }
});