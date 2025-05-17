package repository

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/order-api-microservices/pkg/database"
	"github.com/order-api-microservices/services/provider/internal/model"
)

// ProviderRepository handles operations related to providers
type ProviderRepository struct {
	db *database.PostgresDB
}

// NewProviderRepository creates a new provider repository
func NewProviderRepository(db *database.PostgresDB) *ProviderRepository {
	return &ProviderRepository{
		db: db,
	}
}

// CreateProvider creates a new provider
func (r *ProviderRepository) CreateProvider(ctx context.Context, provider *model.Provider) error {
	if provider.ID == "" {
		provider.ID = uuid.New().String()
	}

	now := time.Now()
	provider.CreatedAt = now
	provider.UpdatedAt = now

	query := `
		INSERT INTO providers (
			id, name, email, phone, rating, service_types, location, is_available, 
			profile_image, metadata, created_at, updated_at
		)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12)
	`

	_, err := r.db.ExecContext(ctx, query,
		provider.ID,
		provider.Name,
		provider.Email,
		provider.Phone,
		provider.Rating,
		model.ServiceTypes(provider.ServiceTypes),
		provider.Location,
		provider.IsAvailable,
		provider.ProfileImage,
		model.Metadata(provider.Metadata),
		provider.CreatedAt,
		provider.UpdatedAt,
	)

	if err != nil {
		return fmt.Errorf("failed to create provider: %w", err)
	}

	return nil
}

// GetProviderByID gets a provider by ID
func (r *ProviderRepository) GetProviderByID(ctx context.Context, providerID string) (*model.Provider, error) {
	query := `
		SELECT id, name, email, phone, rating, service_types, location, is_available, 
		       profile_image, metadata, created_at, updated_at
		FROM providers
		WHERE id = $1
	`

	row := r.db.QueryRowContext(ctx, query, providerID)

	var provider model.Provider
	var serviceTypes model.ServiceTypes
	var metadata model.Metadata

	err := row.Scan(
		&provider.ID,
		&provider.Name,
		&provider.Email,
		&provider.Phone,
		&provider.Rating,
		&serviceTypes,
		&provider.Location,
		&provider.IsAvailable,
		&provider.ProfileImage,
		&metadata,
		&provider.CreatedAt,
		&provider.UpdatedAt,
	)

	if err != nil {
		if err == pgx.ErrNoRows {
			return nil, ErrProviderNotFound
		}
		return nil, fmt.Errorf("failed to get provider: %w", err)
	}

	provider.ServiceTypes = serviceTypes
	provider.Metadata = metadata

	return &provider, nil
}

// UpdateProvider updates an existing provider
func (r *ProviderRepository) UpdateProvider(ctx context.Context, provider *model.Provider) error {
	provider.UpdatedAt = time.Now()

	query := `
		UPDATE providers
		SET name = $2, email = $3, phone = $4, rating = $5, service_types = $6, 
		    location = $7, is_available = $8, profile_image = $9, metadata = $10, updated_at = $11
		WHERE id = $1
	`

	_, err := r.db.ExecContext(ctx, query,
		provider.ID,
		provider.Name,
		provider.Email,
		provider.Phone,
		provider.Rating,
		model.ServiceTypes(provider.ServiceTypes),
		provider.Location,
		provider.IsAvailable,
		provider.ProfileImage,
		model.Metadata(provider.Metadata),
		provider.UpdatedAt,
	)

	if err != nil {
		return fmt.Errorf("failed to update provider: %w", err)
	}

	return nil
}

// UpdateProviderLocation updates a provider's location
func (r *ProviderRepository) UpdateProviderLocation(ctx context.Context, providerID string, location model.Location) error {
	// Update the location in the provider record
	query1 := `
		UPDATE providers
		SET location = $2, updated_at = $3
		WHERE id = $1
	`

	_, err := r.db.ExecContext(ctx, query1, providerID, location, time.Now())
	if err != nil {
		return fmt.Errorf("failed to update provider location: %w", err)
	}

	// Create a new location history entry
	locationID := uuid.New().String()
	now := time.Now()

	query2 := `
		INSERT INTO provider_locations (id, provider_id, latitude, longitude, address, timestamp)
		VALUES ($1, $2, $3, $4, $5, $6)
	`

	_, err = r.db.ExecContext(ctx, query2,
		locationID,
		providerID,
		location.Latitude,
		location.Longitude,
		location.Address,
		now,
	)

	if err != nil {
		return fmt.Errorf("failed to create provider location history: %w", err)
	}

	return nil
}

// UpdateProviderAvailability updates a provider's availability status
func (r *ProviderRepository) UpdateProviderAvailability(ctx context.Context, providerID string, isAvailable bool) error {
	query := `
		UPDATE providers
		SET is_available = $2, updated_at = $3
		WHERE id = $1
	`

	_, err := r.db.ExecContext(ctx, query, providerID, isAvailable, time.Now())
	if err != nil {
		return fmt.Errorf("failed to update provider availability: %w", err)
	}

	return nil
}

// FindNearbyProviders finds providers near a location with specified service type
func (r *ProviderRepository) FindNearbyProviders(ctx context.Context, latitude, longitude float64, radiusKm float64, serviceType string) ([]*model.Provider, error) {
	// Query using Haversine formula to calculate distance in kilometers
	query := `
		SELECT 
			p.id, p.name, p.email, p.phone, p.rating, p.service_types, p.location, 
			p.is_available, p.profile_image, p.metadata, p.created_at, p.updated_at,
			6371 * acos(cos(radians($1)) * cos(radians((p.location->>'latitude')::float)) * 
			cos(radians((p.location->>'longitude')::float) - radians($2)) + 
			sin(radians($1)) * sin(radians((p.location->>'latitude')::float))) AS distance
		FROM providers p
		WHERE p.is_available = true
		AND CASE 
			WHEN $3 <> '' THEN $3 = ANY(p.service_types)
			ELSE true
		END
		AND 6371 * acos(cos(radians($1)) * cos(radians((p.location->>'latitude')::float)) * 
			cos(radians((p.location->>'longitude')::float) - radians($2)) + 
			sin(radians($1)) * sin(radians((p.location->>'latitude')::float))) < $4
		ORDER BY distance
	`

	rows, err := r.db.QueryContext(ctx, query, latitude, longitude, serviceType, radiusKm)
	if err != nil {
		return nil, fmt.Errorf("failed to find nearby providers: %w", err)
	}
	defer rows.Close()

	var providers []*model.Provider
	for rows.Next() {
		var provider model.Provider
		var serviceTypes model.ServiceTypes
		var metadata model.Metadata
		var distance float64

		err := rows.Scan(
			&provider.ID,
			&provider.Name,
			&provider.Email,
			&provider.Phone,
			&provider.Rating,
			&serviceTypes,
			&provider.Location,
			&provider.IsAvailable,
			&provider.ProfileImage,
			&metadata,
			&provider.CreatedAt,
			&provider.UpdatedAt,
			&distance,
		)

		if err != nil {
			return nil, fmt.Errorf("failed to scan provider: %w", err)
		}

		provider.ServiceTypes = serviceTypes
		provider.Metadata = metadata

		// Add the provider to the result set
		providers = append(providers, &provider)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating providers rows: %w", err)
	}

	return providers, nil
} 