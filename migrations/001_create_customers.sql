-- Migration: Create customers table for MikroTik traffic monitoring
-- Description: Simplified customer table for traffic monitoring service

-- Create enum types
CREATE TYPE service_type AS ENUM ('pppoe', 'hotspot', 'static_ip');
CREATE TYPE customer_status AS ENUM ('active', 'suspended', 'inactive', 'pending');

-- Create customers table
CREATE TABLE IF NOT EXISTS customers (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    mikrotik_id VARCHAR(100) NOT NULL,
    
    -- Basic info
    username VARCHAR(100) NOT NULL,
    name VARCHAR(255) NOT NULL,
    phone VARCHAR(20),
    email VARCHAR(255),
    
    -- Service configuration
    service_type service_type NOT NULL,
    
    -- PPPoE specific
    pppoe_username VARCHAR(100),
    pppoe_password VARCHAR(100),
    pppoe_profile VARCHAR(100),
    
    -- Hotspot specific
    hotspot_username VARCHAR(100),
    hotspot_password VARCHAR(100),
    hotspot_mac_address MACADDR,
    
    -- Static IP specific
    static_ip INET,
    
    -- Network info
    assigned_ip INET,
    mac_address MACADDR,
    last_online TIMESTAMPTZ,
    
    -- Status
    status customer_status DEFAULT 'active',
    
    -- Timestamps
    created_at TIMESTAMPTZ DEFAULT NOW(),
    updated_at TIMESTAMPTZ DEFAULT NOW(),
    
    -- Constraints
    UNIQUE (mikrotik_id, username),
    UNIQUE (mikrotik_id, pppoe_username),
    UNIQUE (mikrotik_id, hotspot_username)
);

-- Create indexes for better query performance
CREATE INDEX IF NOT EXISTS idx_customers_mikrotik_id ON customers(mikrotik_id);
CREATE INDEX IF NOT EXISTS idx_customers_username ON customers(username);
CREATE INDEX IF NOT EXISTS idx_customers_pppoe ON customers(pppoe_username);
CREATE INDEX IF NOT EXISTS idx_customers_hotspot ON customers(hotspot_username);
CREATE INDEX IF NOT EXISTS idx_customers_service_type ON customers(service_type);
CREATE INDEX IF NOT EXISTS idx_customers_status ON customers(status);
CREATE INDEX IF NOT EXISTS idx_customers_mac ON customers(mac_address);

-- Create function for updated_at trigger
CREATE OR REPLACE FUNCTION update_updated_at_column()
RETURNS TRIGGER AS $$
BEGIN
    NEW.updated_at = NOW();
    RETURN NEW;
END;
$$ LANGUAGE plpgsql;

-- Create trigger for automatic updated_at
CREATE TRIGGER update_customers_updated_at
    BEFORE UPDATE ON customers
    FOR EACH ROW
    EXECUTE FUNCTION update_updated_at_column();

-- Insert sample data for testing
INSERT INTO customers (mikrotik_id, username, name, service_type, pppoe_username, pppoe_password, status)
VALUES 
    ('mikrotik-001', 'customer1', 'Test Customer 1', 'pppoe', 'tes', '1122', 'active'),
    ('mikrotik-001', 'customer2', 'Test Customer 2', 'pppoe', 'test-pppoe-2', 'password456', 'active'),
    ('mikrotik-001', 'customer3', 'Test Customer 3', 'hotspot', NULL, NULL, 'active')
ON CONFLICT DO NOTHING;
