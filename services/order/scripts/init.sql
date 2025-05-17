-- Create orders table
CREATE TABLE IF NOT EXISTS orders (
    id VARCHAR(36) PRIMARY KEY,
    user_id VARCHAR(36) NOT NULL,
    provider_id VARCHAR(36),
    order_type VARCHAR(20) NOT NULL,
    status VARCHAR(20) NOT NULL,
    pickup_location JSONB NOT NULL,
    destination_location JSONB NOT NULL,
    items JSONB NOT NULL,
    total_price NUMERIC(10, 2) NOT NULL,
    platform_fee NUMERIC(10, 2) NOT NULL,
    provider_fee NUMERIC(10, 2) NOT NULL,
    transaction_id VARCHAR(100),
    blockchain_tx_hash VARCHAR(100),
    payment_method VARCHAR(20) NOT NULL,
    notes TEXT,
    created_at TIMESTAMP NOT NULL,
    updated_at TIMESTAMP NOT NULL,
    status_history JSONB NOT NULL
);

-- Create order_locations table for tracking
CREATE TABLE IF NOT EXISTS order_locations (
    id VARCHAR(36) PRIMARY KEY,
    order_id VARCHAR(36) NOT NULL,
    provider_id VARCHAR(36) NOT NULL,
    latitude DOUBLE PRECISION NOT NULL,
    longitude DOUBLE PRECISION NOT NULL,
    timestamp TIMESTAMP NOT NULL,
    FOREIGN KEY (order_id) REFERENCES orders(id) ON DELETE CASCADE
);

-- Create indexes for faster queries
CREATE INDEX IF NOT EXISTS idx_orders_user_id ON orders(user_id);
CREATE INDEX IF NOT EXISTS idx_orders_provider_id ON orders(provider_id);
CREATE INDEX IF NOT EXISTS idx_orders_status ON orders(status);
CREATE INDEX IF NOT EXISTS idx_orders_created_at ON orders(created_at);
CREATE INDEX IF NOT EXISTS idx_orders_updated_at ON orders(updated_at);

-- Create indexes for order_locations
CREATE INDEX IF NOT EXISTS idx_order_locations_order_id ON order_locations(order_id);
CREATE INDEX IF NOT EXISTS idx_order_locations_provider_id ON order_locations(provider_id);
CREATE INDEX IF NOT EXISTS idx_order_locations_timestamp ON order_locations(timestamp);

-- Create spatial index if PostGIS extension is available
DO $$
BEGIN
    IF EXISTS (
        SELECT 1 FROM pg_extension WHERE extname = 'postgis'
    ) THEN
        -- Create a geometry column for spatial search
        ALTER TABLE order_locations ADD COLUMN IF NOT EXISTS location GEOMETRY(Point, 4326);
        UPDATE order_locations SET location = ST_SetSRID(ST_MakePoint(longitude, latitude), 4326);
        
        -- Add a trigger to automatically update the geometry column
        CREATE OR REPLACE FUNCTION update_order_location_geometry()
        RETURNS TRIGGER AS $$
        BEGIN
            NEW.location = ST_SetSRID(ST_MakePoint(NEW.longitude, NEW.latitude), 4326);
            RETURN NEW;
        END;
        $$ LANGUAGE plpgsql;
        
        DROP TRIGGER IF EXISTS trig_update_order_location_geometry ON order_locations;
        CREATE TRIGGER trig_update_order_location_geometry
        BEFORE INSERT OR UPDATE ON order_locations
        FOR EACH ROW EXECUTE FUNCTION update_order_location_geometry();
        
        -- Create a spatial index
        CREATE INDEX IF NOT EXISTS idx_order_locations_spatial ON order_locations USING GIST(location);
    END IF;
END
$$; 