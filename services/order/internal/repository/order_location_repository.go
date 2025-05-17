package repository

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/order-api-microservices/pkg/database"
	"github.com/order-api-microservices/services/order/internal/model"
)

// OrderLocationRepository handles operations related to order locations
type OrderLocationRepository struct {
	db *database.PostgresDB
}

// NewOrderLocationRepository creates a new order location repository
func NewOrderLocationRepository(db *database.PostgresDB) *OrderLocationRepository {
	return &OrderLocationRepository{
		db: db,
	}
}

// CreateOrderLocation creates a new order location entry
func (r *OrderLocationRepository) CreateOrderLocation(ctx context.Context, orderLocation *model.OrderLocation) error {
	if orderLocation.ID == "" {
		orderLocation.ID = uuid.New().String()
	}

	orderLocation.Timestamp = time.Now()

	query := `
		INSERT INTO order_locations (id, order_id, provider_id, latitude, longitude, timestamp)
		VALUES ($1, $2, $3, $4, $5, $6)
	`

	_, err := r.db.ExecContext(ctx, query,
		orderLocation.ID,
		orderLocation.OrderID,
		orderLocation.ProviderID,
		orderLocation.Latitude,
		orderLocation.Longitude,
		orderLocation.Timestamp,
	)

	if err != nil {
		return fmt.Errorf("failed to create order location: %w", err)
	}

	return nil
}

// GetLatestOrderLocation gets the latest location for an order
func (r *OrderLocationRepository) GetLatestOrderLocation(ctx context.Context, orderID string) (*model.OrderLocation, error) {
	query := `
		SELECT id, order_id, provider_id, latitude, longitude, timestamp
		FROM order_locations
		WHERE order_id = $1
		ORDER BY timestamp DESC
		LIMIT 1
	`

	row := r.db.QueryRowContext(ctx, query, orderID)

	var location model.OrderLocation
	err := row.Scan(
		&location.ID,
		&location.OrderID,
		&location.ProviderID,
		&location.Latitude,
		&location.Longitude,
		&location.Timestamp,
	)

	if err != nil {
		if err == pgx.ErrNoRows {
			return nil, ErrOrderLocationNotFound
		}
		return nil, fmt.Errorf("failed to get latest order location: %w", err)
	}

	return &location, nil
}

// GetOrderLocationHistory gets the location history for an order
func (r *OrderLocationRepository) GetOrderLocationHistory(ctx context.Context, orderID string, limit int) ([]*model.OrderLocation, error) {
	query := `
		SELECT id, order_id, provider_id, latitude, longitude, timestamp
		FROM order_locations
		WHERE order_id = $1
		ORDER BY timestamp DESC
		LIMIT $2
	`

	rows, err := r.db.QueryContext(ctx, query, orderID, limit)
	if err != nil {
		return nil, fmt.Errorf("failed to get order location history: %w", err)
	}
	defer rows.Close()

	var locations []*model.OrderLocation
	for rows.Next() {
		var location model.OrderLocation
		err := rows.Scan(
			&location.ID,
			&location.OrderID,
			&location.ProviderID,
			&location.Latitude,
			&location.Longitude,
			&location.Timestamp,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan order location: %w", err)
		}

		locations = append(locations, &location)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating order locations: %w", err)
	}

	return locations, nil
}

// GetProviderCurrentOrders gets the current orders assigned to a provider
func (r *OrderLocationRepository) GetProviderCurrentOrders(ctx context.Context, providerID string) ([]string, error) {
	query := `
		SELECT DISTINCT order_id
		FROM order_locations
		WHERE provider_id = $1
		AND EXISTS (
			SELECT 1 FROM orders 
			WHERE orders.id = order_locations.order_id
			AND orders.status NOT IN ('COMPLETED', 'CANCELLED', 'REFUNDED')
		)
		ORDER BY order_id
	`

	rows, err := r.db.QueryContext(ctx, query, providerID)
	if err != nil {
		return nil, fmt.Errorf("failed to get provider current orders: %w", err)
	}
	defer rows.Close()

	var orderIDs []string
	for rows.Next() {
		var orderID string
		err := rows.Scan(&orderID)
		if err != nil {
			return nil, fmt.Errorf("failed to scan order ID: %w", err)
		}

		orderIDs = append(orderIDs, orderID)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating order IDs: %w", err)
	}

	return orderIDs, nil
}

// DeleteOrderLocations deletes all location entries for an order
func (r *OrderLocationRepository) DeleteOrderLocations(ctx context.Context, orderID string) error {
	query := `
		DELETE FROM order_locations
		WHERE order_id = $1
	`

	_, err := r.db.ExecContext(ctx, query, orderID)
	if err != nil {
		return fmt.Errorf("failed to delete order locations: %w", err)
	}

	return nil
}

// GetNearbyOrderLocations gets order locations near a given location
func (r *OrderLocationRepository) GetNearbyOrderLocations(ctx context.Context, latitude, longitude float64, radiusKm float64) ([]*model.OrderLocation, error) {
	// Postgres query using the Haversine formula to calculate distance
	query := `
		WITH latest_locations AS (
			SELECT DISTINCT ON (order_id) id, order_id, provider_id, latitude, longitude, timestamp
			FROM order_locations
			ORDER BY order_id, timestamp DESC
		)
		SELECT l.id, l.order_id, l.provider_id, l.latitude, l.longitude, l.timestamp,
			   6371 * acos(cos(radians($1)) * cos(radians(l.latitude)) * cos(radians(l.longitude) - radians($2)) + sin(radians($1)) * sin(radians(l.latitude))) AS distance
		FROM latest_locations l
		JOIN orders o ON l.order_id = o.id
		WHERE o.status NOT IN ('COMPLETED', 'CANCELLED', 'REFUNDED')
		AND 6371 * acos(cos(radians($1)) * cos(radians(l.latitude)) * cos(radians(l.longitude) - radians($2)) + sin(radians($1)) * sin(radians(l.latitude))) < $3
		ORDER BY distance
	`

	rows, err := r.db.QueryContext(ctx, query, latitude, longitude, radiusKm)
	if err != nil {
		return nil, fmt.Errorf("failed to get nearby order locations: %w", err)
	}
	defer rows.Close()

	var locations []*model.OrderLocation
	for rows.Next() {
		var location model.OrderLocation
		var distance float64
		err := rows.Scan(
			&location.ID,
			&location.OrderID,
			&location.ProviderID,
			&location.Latitude,
			&location.Longitude,
			&location.Timestamp,
			&distance,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan order location: %w", err)
		}

		locations = append(locations, &location)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating order locations: %w", err)
	}

	return locations, nil
} 