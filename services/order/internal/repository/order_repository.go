package repository

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/order-api-microservices/pkg/database"
	"github.com/order-api-microservices/services/order/internal/model"
)

var (
	ErrOrderNotFound = errors.New("order not found")
	ErrInvalidData   = errors.New("invalid data")
)

// OrderRepository handles database operations for orders
type OrderRepository struct {
	db *database.PostgresDB
}

// NewOrderRepository creates a new order repository
func NewOrderRepository(db *database.PostgresDB) *OrderRepository {
	return &OrderRepository{
		db: db,
	}
}

// CreateOrder creates a new order in the database
func (r *OrderRepository) CreateOrder(ctx context.Context, order *model.Order) error {
	if order.ID == "" || order.UserID == "" {
		return ErrInvalidData
	}

	query := `
		INSERT INTO orders (
			id, user_id, provider_id, order_type, status, 
			pickup_location, destination_location, items, 
			total_price, platform_fee, provider_fee, 
			transaction_id, blockchain_tx_hash, payment_method, 
			notes, created_at, updated_at, status_history
		) VALUES (
			$1, $2, $3, $4, $5, 
			$6, $7, $8, 
			$9, $10, $11, 
			$12, $13, $14, 
			$15, $16, $17, $18
		)
	`

	_, err := r.db.ExecContext(
		ctx,
		query,
		order.ID,
		order.UserID,
		order.ProviderID,
		order.OrderType,
		order.Status,
		order.PickupLocation,
		order.DestinationLocation,
		order.Items,
		order.TotalPrice,
		order.PlatformFee,
		order.ProviderFee,
		order.TransactionID,
		order.BlockchainTxHash,
		order.PaymentMethod,
		order.Notes,
		order.CreatedAt,
		order.UpdatedAt,
		order.StatusHistory,
	)

	if err != nil {
		return fmt.Errorf("failed to create order: %w", err)
	}

	return nil
}

// GetOrderByID gets an order by its ID
func (r *OrderRepository) GetOrderByID(ctx context.Context, orderID string) (*model.Order, error) {
	query := `
		SELECT
			id, user_id, provider_id, order_type, status, 
			pickup_location, destination_location, items, 
			total_price, platform_fee, provider_fee, 
			transaction_id, blockchain_tx_hash, payment_method, 
			notes, created_at, updated_at, status_history
		FROM orders
		WHERE id = $1
	`

	order := &model.Order{}
	err := r.db.QueryRowContext(ctx, query, orderID).Scan(
		&order.ID,
		&order.UserID,
		&order.ProviderID,
		&order.OrderType,
		&order.Status,
		&order.PickupLocation,
		&order.DestinationLocation,
		&order.Items,
		&order.TotalPrice,
		&order.PlatformFee,
		&order.ProviderFee,
		&order.TransactionID,
		&order.BlockchainTxHash,
		&order.PaymentMethod,
		&order.Notes,
		&order.CreatedAt,
		&order.UpdatedAt,
		&order.StatusHistory,
	)

	if err != nil {
		if err == pgx.ErrNoRows {
			return nil, ErrOrderNotFound
		}
		return nil, fmt.Errorf("failed to get order: %w", err)
	}

	return order, nil
}

// UpdateOrder updates an existing order
func (r *OrderRepository) UpdateOrder(ctx context.Context, order *model.Order) error {
	if order.ID == "" {
		return ErrInvalidData
	}

	query := `
		UPDATE orders
		SET 
			user_id = $2,
			provider_id = $3,
			order_type = $4,
			status = $5,
			pickup_location = $6,
			destination_location = $7,
			items = $8,
			total_price = $9,
			platform_fee = $10,
			provider_fee = $11,
			transaction_id = $12,
			blockchain_tx_hash = $13,
			payment_method = $14,
			notes = $15,
			updated_at = $16,
			status_history = $17
		WHERE id = $1
	`

	order.UpdatedAt = time.Now()

	ct, err := r.db.ExecContext(
		ctx,
		query,
		order.ID,
		order.UserID,
		order.ProviderID,
		order.OrderType,
		order.Status,
		order.PickupLocation,
		order.DestinationLocation,
		order.Items,
		order.TotalPrice,
		order.PlatformFee,
		order.ProviderFee,
		order.TransactionID,
		order.BlockchainTxHash,
		order.PaymentMethod,
		order.Notes,
		order.UpdatedAt,
		order.StatusHistory,
	)

	if err != nil {
		return fmt.Errorf("failed to update order: %w", err)
	}

	if ct.RowsAffected() == 0 {
		return ErrOrderNotFound
	}

	return nil
}

// UpdateOrderStatus updates just the status of an order
func (r *OrderRepository) UpdateOrderStatus(ctx context.Context, orderID string, status model.OrderStatus, updatedBy, notes string) error {
	// Start a transaction
	tx, err := r.db.BeginTx(ctx)
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer tx.Rollback(ctx)

	// Get the current order
	query := `
		SELECT status_history, status
		FROM orders
		WHERE id = $1
		FOR UPDATE
	`
	var statusHistory model.StatusHistories
	var currentStatus model.OrderStatus
	err = tx.QueryRow(ctx, query, orderID).Scan(&statusHistory, &currentStatus)
	if err != nil {
		if err == pgx.ErrNoRows {
			return ErrOrderNotFound
		}
		return fmt.Errorf("failed to get order: %w", err)
	}

	// Add the new status history entry
	newEntry := model.StatusHistory{
		Status:    status,
		UpdatedBy: updatedBy,
		Notes:     notes,
		Timestamp: time.Now(),
	}
	statusHistory = append(statusHistory, newEntry)

	// Update the order
	updateQuery := `
		UPDATE orders
		SET status = $2, status_history = $3, updated_at = $4
		WHERE id = $1
	`
	_, err = tx.Exec(ctx, updateQuery, orderID, status, statusHistory, time.Now())
	if err != nil {
		return fmt.Errorf("failed to update order status: %w", err)
	}

	// Commit the transaction
	err = tx.Commit(ctx)
	if err != nil {
		return fmt.Errorf("failed to commit transaction: %w", err)
	}

	return nil
}

// ListUserOrders gets all orders for a specific user
func (r *OrderRepository) ListUserOrders(ctx context.Context, userID string, page, limit int, status model.OrderStatus) ([]*model.Order, int, error) {
	var whereClause string
	var args []interface{}
	args = append(args, userID)

	if status != "" {
		whereClause = " AND status = $2"
		args = append(args, status)
	}

	// Count total orders
	countQuery := fmt.Sprintf(`SELECT COUNT(*) FROM orders WHERE user_id = $1%s`, whereClause)
	var total int
	err := r.db.QueryRowContext(ctx, countQuery, args...).Scan(&total)
	if err != nil {
		return nil, 0, fmt.Errorf("failed to count orders: %w", err)
	}

	// Set reasonable defaults and boundaries
	if page < 1 {
		page = 1
	}
	if limit < 1 || limit > 100 {
		limit = 10
	}

	offset := (page - 1) * limit
	args = append(args, limit, offset)

	// Get paginated orders
	query := fmt.Sprintf(`
		SELECT
			id, user_id, provider_id, order_type, status, 
			pickup_location, destination_location, items, 
			total_price, platform_fee, provider_fee, 
			transaction_id, blockchain_tx_hash, payment_method, 
			notes, created_at, updated_at, status_history
		FROM orders
		WHERE user_id = $1%s
		ORDER BY created_at DESC
		LIMIT $%d OFFSET $%d
	`, whereClause, len(args)-1, len(args))

	rows, err := r.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, 0, fmt.Errorf("failed to query orders: %w", err)
	}
	defer rows.Close()

	orders := []*model.Order{}
	for rows.Next() {
		order := &model.Order{}
		err := rows.Scan(
			&order.ID,
			&order.UserID,
			&order.ProviderID,
			&order.OrderType,
			&order.Status,
			&order.PickupLocation,
			&order.DestinationLocation,
			&order.Items,
			&order.TotalPrice,
			&order.PlatformFee,
			&order.ProviderFee,
			&order.TransactionID,
			&order.BlockchainTxHash,
			&order.PaymentMethod,
			&order.Notes,
			&order.CreatedAt,
			&order.UpdatedAt,
			&order.StatusHistory,
		)
		if err != nil {
			return nil, 0, fmt.Errorf("failed to scan order: %w", err)
		}
		orders = append(orders, order)
	}

	if err := rows.Err(); err != nil {
		return nil, 0, fmt.Errorf("error iterating orders: %w", err)
	}

	return orders, total, nil
}

// ListProviderOrders gets all orders for a specific provider
func (r *OrderRepository) ListProviderOrders(ctx context.Context, providerID string, page, limit int, status model.OrderStatus) ([]*model.Order, int, error) {
	var whereClause string
	var args []interface{}
	args = append(args, providerID)

	if status != "" {
		whereClause = " AND status = $2"
		args = append(args, status)
	}

	// Count total orders
	countQuery := fmt.Sprintf(`SELECT COUNT(*) FROM orders WHERE provider_id = $1%s`, whereClause)
	var total int
	err := r.db.QueryRowContext(ctx, countQuery, args...).Scan(&total)
	if err != nil {
		return nil, 0, fmt.Errorf("failed to count orders: %w", err)
	}

	// Set reasonable defaults and boundaries
	if page < 1 {
		page = 1
	}
	if limit < 1 || limit > 100 {
		limit = 10
	}

	offset := (page - 1) * limit
	args = append(args, limit, offset)

	// Get paginated orders
	query := fmt.Sprintf(`
		SELECT
			id, user_id, provider_id, order_type, status, 
			pickup_location, destination_location, items, 
			total_price, platform_fee, provider_fee, 
			transaction_id, blockchain_tx_hash, payment_method, 
			notes, created_at, updated_at, status_history
		FROM orders
		WHERE provider_id = $1%s
		ORDER BY created_at DESC
		LIMIT $%d OFFSET $%d
	`, whereClause, len(args)-1, len(args))

	rows, err := r.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, 0, fmt.Errorf("failed to query orders: %w", err)
	}
	defer rows.Close()

	orders := []*model.Order{}
	for rows.Next() {
		order := &model.Order{}
		err := rows.Scan(
			&order.ID,
			&order.UserID,
			&order.ProviderID,
			&order.OrderType,
			&order.Status,
			&order.PickupLocation,
			&order.DestinationLocation,
			&order.Items,
			&order.TotalPrice,
			&order.PlatformFee,
			&order.ProviderFee,
			&order.TransactionID,
			&order.BlockchainTxHash,
			&order.PaymentMethod,
			&order.Notes,
			&order.CreatedAt,
			&order.UpdatedAt,
			&order.StatusHistory,
		)
		if err != nil {
			return nil, 0, fmt.Errorf("failed to scan order: %w", err)
		}
		orders = append(orders, order)
	}

	if err := rows.Err(); err != nil {
		return nil, 0, fmt.Errorf("error iterating orders: %w", err)
	}

	return orders, total, nil
}

// AddOrderLocation adds a location update for an order
func (r *OrderRepository) AddOrderLocation(ctx context.Context, location *model.OrderLocation) error {
	query := `
		INSERT INTO order_locations (
			id, order_id, provider_id, latitude, longitude, timestamp
		) VALUES (
			$1, $2, $3, $4, $5, $6
		)
	`

	_, err := r.db.ExecContext(
		ctx,
		query,
		location.ID,
		location.OrderID,
		location.ProviderID,
		location.Latitude,
		location.Longitude,
		location.Timestamp,
	)

	if err != nil {
		return fmt.Errorf("failed to add order location: %w", err)
	}

	return nil
}

// GetLatestOrderLocation gets the latest location update for an order
func (r *OrderRepository) GetLatestOrderLocation(ctx context.Context, orderID string) (*model.OrderLocation, error) {
	query := `
		SELECT id, order_id, provider_id, latitude, longitude, timestamp
		FROM order_locations
		WHERE order_id = $1
		ORDER BY timestamp DESC
		LIMIT 1
	`

	location := &model.OrderLocation{}
	err := r.db.QueryRowContext(ctx, query, orderID).Scan(
		&location.ID,
		&location.OrderID,
		&location.ProviderID,
		&location.Latitude,
		&location.Longitude,
		&location.Timestamp,
	)

	if err != nil {
		if err == pgx.ErrNoRows {
			return nil, ErrOrderNotFound
		}
		return nil, fmt.Errorf("failed to get latest order location: %w", err)
	}

	return location, nil
}

// GetOrderLocationsHistory gets the location history for an order
func (r *OrderRepository) GetOrderLocationsHistory(ctx context.Context, orderID string, limit int) ([]*model.OrderLocation, error) {
	if limit <= 0 || limit > 100 {
		limit = 20 // Default limit
	}

	query := `
		SELECT id, order_id, provider_id, latitude, longitude, timestamp
		FROM order_locations
		WHERE order_id = $1
		ORDER BY timestamp DESC
		LIMIT $2
	`

	rows, err := r.db.QueryContext(ctx, query, orderID, limit)
	if err != nil {
		return nil, fmt.Errorf("failed to query order locations: %w", err)
	}
	defer rows.Close()

	locations := []*model.OrderLocation{}
	for rows.Next() {
		location := &model.OrderLocation{}
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
		locations = append(locations, location)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating order locations: %w", err)
	}

	return locations, nil
} 