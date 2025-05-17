-- Create providers table
CREATE TABLE IF NOT EXISTS providers (
    id VARCHAR(36) PRIMARY KEY,
    name VARCHAR(100) NOT NULL,
    email VARCHAR(100) NOT NULL,
    phone VARCHAR(20) NOT NULL,
    rating FLOAT NOT NULL DEFAULT 0,
    service_types JSONB NOT NULL,
    location JSONB NOT NULL,
    is_available BOOLEAN NOT NULL DEFAULT false,
    profile_image VARCHAR(255),
    metadata JSONB,
    created_at TIMESTAMP NOT NULL,
    updated_at TIMESTAMP NOT NULL
);

-- Create provider_locations table for tracking
CREATE TABLE IF NOT EXISTS provider_locations (
    id VARCHAR(36) PRIMARY KEY,
    provider_id VARCHAR(36) NOT NULL,
    latitude DOUBLE PRECISION NOT NULL,
    longitude DOUBLE PRECISION NOT NULL,
    address VARCHAR(255),
    timestamp TIMESTAMP NOT NULL,
    FOREIGN KEY (provider_id) REFERENCES providers(id) ON DELETE CASCADE
);

-- Create indexes for faster queries
CREATE INDEX IF NOT EXISTS idx_providers_service_types ON providers USING GIN(service_types);
CREATE INDEX IF NOT EXISTS idx_providers_is_available ON providers(is_available);
CREATE INDEX IF NOT EXISTS idx_provider_locations_provider_id ON provider_locations(provider_id);
CREATE INDEX IF NOT EXISTS idx_provider_locations_timestamp ON provider_locations(timestamp);

-- Create spatial index if PostGIS extension is available
DO $$
BEGIN
    IF EXISTS (
        SELECT 1 FROM pg_extension WHERE extname = 'postgis'
    ) THEN
        -- Create a geometry column for spatial search
        ALTER TABLE provider_locations ADD COLUMN IF NOT EXISTS location GEOMETRY(Point, 4326);
        UPDATE provider_locations SET location = ST_SetSRID(ST_MakePoint(longitude, latitude), 4326);
        
        -- Add a trigger to automatically update the geometry column
        CREATE OR REPLACE FUNCTION update_provider_location_geometry()
        RETURNS TRIGGER AS $$
        BEGIN
            NEW.location = ST_SetSRID(ST_MakePoint(NEW.longitude, NEW.latitude), 4326);
            RETURN NEW;
        END;
        $$ LANGUAGE plpgsql;
        
        DROP TRIGGER IF EXISTS trig_update_provider_location_geometry ON provider_locations;
        CREATE TRIGGER trig_update_provider_location_geometry
        BEFORE INSERT OR UPDATE ON provider_locations
        FOR EACH ROW EXECUTE FUNCTION update_provider_location_geometry();
        
        -- Create a spatial index
        CREATE INDEX IF NOT EXISTS idx_provider_locations_spatial ON provider_locations USING GIST(location);
    END IF;
END
$$;

-- Insert sample data
INSERT INTO providers (id, name, email, phone, rating, service_types, location, is_available, profile_image, metadata, created_at, updated_at)
VALUES 
    ('d290f1ee-6c54-4b01-90e6-d701748f0851', 'John Driver', 'john@example.com', '+1234567890', 4.8, 
     '["ride", "package_delivery"]'::jsonb, 
     '{"latitude": 37.7749, "longitude": -122.4194, "address": "San Francisco, CA"}'::jsonb,
     true, 'https://example.com/profile/john.jpg', 
     '{"vehicle_type": "sedan", "license_plate": "ABC123"}'::jsonb, 
     NOW(), NOW()),
     
    ('d290f1ee-6c54-4b01-90e6-d701748f0852', 'Jane Food', 'jane@example.com', '+1987654321', 4.9, 
     '["food_delivery", "grocery_delivery"]'::jsonb, 
     '{"latitude": 37.7833, "longitude": -122.4167, "address": "San Francisco, CA"}'::jsonb,
     true, 'https://example.com/profile/jane.jpg', 
     '{"delivery_type": "bicycle"}'::jsonb, 
     NOW(), NOW()),
     
    ('d290f1ee-6c54-4b01-90e6-d701748f0853', 'Sam Service', 'sam@example.com', '+1122334455', 4.7, 
     '["service_booking"]'::jsonb, 
     '{"latitude": 37.7694, "longitude": -122.4862, "address": "San Francisco, CA"}'::jsonb,
     false, 'https://example.com/profile/sam.jpg', 
     '{"specialty": "plumbing", "experience_years": "10"}'::jsonb, 
     NOW(), NOW());

-- Insert sample location history
INSERT INTO provider_locations (id, provider_id, latitude, longitude, address, timestamp)
VALUES
    (uuid_generate_v4(), 'd290f1ee-6c54-4b01-90e6-d701748f0851', 37.7749, -122.4194, 'San Francisco, CA', NOW() - INTERVAL '1 hour'),
    (uuid_generate_v4(), 'd290f1ee-6c54-4b01-90e6-d701748f0851', 37.7833, -122.4167, 'San Francisco, CA', NOW() - INTERVAL '30 minutes'),
    (uuid_generate_v4(), 'd290f1ee-6c54-4b01-90e6-d701748f0851', 37.7694, -122.4862, 'San Francisco, CA', NOW()),
    (uuid_generate_v4(), 'd290f1ee-6c54-4b01-90e6-d701748f0852', 37.7833, -122.4167, 'San Francisco, CA', NOW() - INTERVAL '2 hours'),
    (uuid_generate_v4(), 'd290f1ee-6c54-4b01-90e6-d701748f0852', 37.7694, -122.4862, 'San Francisco, CA', NOW() - INTERVAL '1 hour'),
    (uuid_generate_v4(), 'd290f1ee-6c54-4b01-90e6-d701748f0852', 37.7749, -122.4194, 'San Francisco, CA', NOW()); 