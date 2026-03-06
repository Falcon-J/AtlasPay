-- AtlasPay Database Schema
-- Run: psql -U atlaspay -d atlaspay -f init.sql

-- Users table
CREATE TABLE IF NOT EXISTS users (
    id VARCHAR(36) PRIMARY KEY,
    email VARCHAR(255) UNIQUE NOT NULL,
    password_hash VARCHAR(255) NOT NULL,
    first_name VARCHAR(100) NOT NULL,
    last_name VARCHAR(100) NOT NULL,
    role VARCHAR(20) NOT NULL DEFAULT 'user',
    active BOOLEAN NOT NULL DEFAULT true,
    created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX idx_users_email ON users(email);
CREATE INDEX idx_users_role ON users(role);

-- Refresh tokens table
CREATE TABLE IF NOT EXISTS refresh_tokens (
    id VARCHAR(36) PRIMARY KEY,
    user_id VARCHAR(36) NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    token TEXT NOT NULL UNIQUE,
    expires_at TIMESTAMP NOT NULL,
    created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    revoked BOOLEAN NOT NULL DEFAULT false
);

CREATE INDEX idx_refresh_tokens_user_id ON refresh_tokens(user_id);
CREATE INDEX idx_refresh_tokens_token ON refresh_tokens(token);
CREATE INDEX idx_refresh_tokens_expires ON refresh_tokens(expires_at);

-- Orders table
CREATE TABLE IF NOT EXISTS orders (
    id VARCHAR(36) PRIMARY KEY,
    user_id VARCHAR(36) NOT NULL REFERENCES users(id),
    status VARCHAR(20) NOT NULL DEFAULT 'pending',
    total_price DECIMAL(10, 2) NOT NULL DEFAULT 0,
    currency VARCHAR(3) NOT NULL DEFAULT 'USD',
    created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX idx_orders_user_id ON orders(user_id);
CREATE INDEX idx_orders_status ON orders(status);
CREATE INDEX idx_orders_created_at ON orders(created_at DESC);

-- Order items table
CREATE TABLE IF NOT EXISTS order_items (
    id VARCHAR(36) PRIMARY KEY,
    order_id VARCHAR(36) NOT NULL REFERENCES orders(id) ON DELETE CASCADE,
    sku VARCHAR(50) NOT NULL,
    name VARCHAR(255) NOT NULL,
    quantity INTEGER NOT NULL,
    unit_price DECIMAL(10, 2) NOT NULL,
    total_price DECIMAL(10, 2) NOT NULL
);

CREATE INDEX idx_order_items_order_id ON order_items(order_id);
CREATE INDEX idx_order_items_sku ON order_items(sku);

-- Payments table
CREATE TABLE IF NOT EXISTS payments (
    id VARCHAR(36) PRIMARY KEY,
    order_id VARCHAR(36) NOT NULL REFERENCES orders(id),
    user_id VARCHAR(36) NOT NULL REFERENCES users(id),
    amount DECIMAL(10, 2) NOT NULL,
    currency VARCHAR(3) NOT NULL DEFAULT 'USD',
    status VARCHAR(20) NOT NULL DEFAULT 'pending',
    payment_method VARCHAR(50) NOT NULL,
    idempotency_key VARCHAR(100) UNIQUE NOT NULL,
    failure_reason TEXT,
    created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX idx_payments_order_id ON payments(order_id);
CREATE INDEX idx_payments_user_id ON payments(user_id);
CREATE INDEX idx_payments_idempotency_key ON payments(idempotency_key);
CREATE INDEX idx_payments_status ON payments(status);

-- Inventory table
CREATE TABLE IF NOT EXISTS inventory (
    id VARCHAR(36) PRIMARY KEY,
    sku VARCHAR(50) UNIQUE NOT NULL,
    name VARCHAR(255) NOT NULL,
    description TEXT,
    quantity INTEGER NOT NULL DEFAULT 0,
    reserved_qty INTEGER NOT NULL DEFAULT 0,
    unit_price DECIMAL(10, 2) NOT NULL,
    version INTEGER NOT NULL DEFAULT 1,
    created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    CONSTRAINT positive_quantity CHECK (quantity >= 0),
    CONSTRAINT positive_reserved CHECK (reserved_qty >= 0),
    CONSTRAINT reserved_less_than_quantity CHECK (reserved_qty <= quantity)
);

CREATE INDEX idx_inventory_sku ON inventory(sku);

-- Reservations table
CREATE TABLE IF NOT EXISTS reservations (
    id VARCHAR(36) PRIMARY KEY,
    order_id VARCHAR(36) NOT NULL,
    sku VARCHAR(50) NOT NULL,
    quantity INTEGER NOT NULL,
    status VARCHAR(20) NOT NULL DEFAULT 'reserved',
    expires_at TIMESTAMP NOT NULL,
    created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX idx_reservations_order_id ON reservations(order_id);
CREATE INDEX idx_reservations_sku ON reservations(sku);
CREATE INDEX idx_reservations_status ON reservations(status);
CREATE INDEX idx_reservations_expires ON reservations(expires_at);

-- Saga execution logs (for debugging and recovery)
CREATE TABLE IF NOT EXISTS saga_logs (
    id VARCHAR(36) PRIMARY KEY,
    saga_id VARCHAR(36) NOT NULL,
    step_name VARCHAR(50) NOT NULL,
    status VARCHAR(20) NOT NULL,
    payload JSONB,
    error_message TEXT,
    created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX idx_saga_logs_saga_id ON saga_logs(saga_id);
CREATE INDEX idx_saga_logs_status ON saga_logs(status);

-- Insert sample inventory data for testing
INSERT INTO inventory (id, sku, name, description, quantity, unit_price) VALUES
    ('inv-001', 'LAPTOP-001', 'MacBook Pro 14"', 'Apple MacBook Pro with M3 chip', 100, 1999.99),
    ('inv-002', 'PHONE-001', 'iPhone 15 Pro', 'Apple iPhone 15 Pro 256GB', 250, 1199.99),
    ('inv-003', 'HEADPHONES-001', 'AirPods Pro', 'Apple AirPods Pro 2nd Gen', 500, 249.99),
    ('inv-004', 'WATCH-001', 'Apple Watch Ultra', 'Apple Watch Ultra 2', 150, 799.99),
    ('inv-005', 'TABLET-001', 'iPad Pro 12.9"', 'Apple iPad Pro M2 chip', 200, 1099.99)
ON CONFLICT (sku) DO NOTHING;

-- Insert admin user (password: admin123)
INSERT INTO users (id, email, password_hash, first_name, last_name, role) VALUES
    ('admin-001', 'admin@atlaspay.com', '$2a$10$N9qo8uLOickgx2ZMRZoMyeIjZRGdjGj/n3.RP5mbJ1PO.VYELPalm', 'Admin', 'User', 'admin')
ON CONFLICT (email) DO NOTHING;
