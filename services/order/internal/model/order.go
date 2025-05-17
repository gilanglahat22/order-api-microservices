package model

import (
	"database/sql/driver"
	"encoding/json"
	"errors"
	"time"
)

// OrderStatus represents the status of an order
type OrderStatus string

const (
	StatusCreated         OrderStatus = "CREATED"
	StatusPaymentPending  OrderStatus = "PAYMENT_PENDING"
	StatusPaymentComplete OrderStatus = "PAYMENT_COMPLETED"
	StatusProviderAssigned OrderStatus = "PROVIDER_ASSIGNED"
	StatusProviderAccepted OrderStatus = "PROVIDER_ACCEPTED"
	StatusProviderRejected OrderStatus = "PROVIDER_REJECTED"
	StatusInProgress      OrderStatus = "IN_PROGRESS"
	StatusPickedUp        OrderStatus = "PICKED_UP"
	StatusInTransit       OrderStatus = "IN_TRANSIT"
	StatusArrived         OrderStatus = "ARRIVED"
	StatusDelivered       OrderStatus = "DELIVERED"
	StatusCompleted       OrderStatus = "COMPLETED"
	StatusCancelled       OrderStatus = "CANCELLED"
	StatusRefunded        OrderStatus = "REFUNDED"
	StatusDisputed        OrderStatus = "DISPUTED"
)

// OrderType represents the type of order
type OrderType string

const (
	TypeRide            OrderType = "RIDE"
	TypeFoodDelivery    OrderType = "FOOD_DELIVERY"
	TypePackageDelivery OrderType = "PACKAGE_DELIVERY"
	TypeGroceryDelivery OrderType = "GROCERY_DELIVERY"
	TypeServiceBooking  OrderType = "SERVICE_BOOKING"
)

// PaymentMethod represents the payment method for an order
type PaymentMethod string

const (
	PaymentCreditCard   PaymentMethod = "CREDIT_CARD"
	PaymentDebitCard    PaymentMethod = "DEBIT_CARD"
	PaymentDigitalWallet PaymentMethod = "DIGITAL_WALLET"
	PaymentCash         PaymentMethod = "CASH"
	PaymentCrypto       PaymentMethod = "CRYPTO"
)

// Location represents a geographical location
type Location struct {
	Latitude     float64           `json:"latitude"`
	Longitude    float64           `json:"longitude"`
	Address      string            `json:"address"`
	PostalCode   string            `json:"postal_code,omitempty"`
	City         string            `json:"city,omitempty"`
	Country      string            `json:"country,omitempty"`
	AdditionalInfo map[string]string `json:"additional_info,omitempty"`
}

// Value implements the driver.Valuer interface for JSON serialization
func (l Location) Value() (driver.Value, error) {
	return json.Marshal(l)
}

// Scan implements the sql.Scanner interface for JSON deserialization
func (l *Location) Scan(value interface{}) error {
	b, ok := value.([]byte)
	if !ok {
		return errors.New("type assertion to []byte failed")
	}
	return json.Unmarshal(b, l)
}

// OrderItem represents an item in an order
type OrderItem struct {
	ItemID     string            `json:"item_id"`
	Name       string            `json:"name"`
	Quantity   int               `json:"quantity"`
	Price      float64           `json:"price"`
	Properties map[string]string `json:"properties,omitempty"`
}

// Value implements the driver.Valuer interface for JSON serialization
func (i OrderItems) Value() (driver.Value, error) {
	return json.Marshal(i)
}

// Scan implements the sql.Scanner interface for JSON deserialization
func (i *OrderItems) Scan(value interface{}) error {
	b, ok := value.([]byte)
	if !ok {
		return errors.New("type assertion to []byte failed")
	}
	return json.Unmarshal(b, i)
}

// OrderItems is a slice of OrderItem
type OrderItems []OrderItem

// StatusHistory represents a status change in the order's lifecycle
type StatusHistory struct {
	Status    OrderStatus `json:"status"`
	UpdatedBy string      `json:"updated_by"`
	Notes     string      `json:"notes,omitempty"`
	Timestamp time.Time   `json:"timestamp"`
}

// Value implements the driver.Valuer interface for JSON serialization
func (sh StatusHistories) Value() (driver.Value, error) {
	return json.Marshal(sh)
}

// Scan implements the sql.Scanner interface for JSON deserialization
func (sh *StatusHistories) Scan(value interface{}) error {
	b, ok := value.([]byte)
	if !ok {
		return errors.New("type assertion to []byte failed")
	}
	return json.Unmarshal(b, sh)
}

// StatusHistories is a slice of StatusHistory
type StatusHistories []StatusHistory

// Order represents an order in the system
type Order struct {
	ID                 string          `json:"id"`
	UserID             string          `json:"user_id"`
	ProviderID         string          `json:"provider_id,omitempty"`
	OrderType          OrderType       `json:"order_type"`
	Status             OrderStatus     `json:"status"`
	PickupLocation     Location        `json:"pickup_location"`
	DestinationLocation Location        `json:"destination_location"`
	Items              OrderItems      `json:"items"`
	TotalPrice         float64         `json:"total_price"`
	PlatformFee        float64         `json:"platform_fee"`
	ProviderFee        float64         `json:"provider_fee"`
	TransactionID      string          `json:"transaction_id,omitempty"`
	BlockchainTxHash   string          `json:"blockchain_tx_hash,omitempty"`
	PaymentMethod      PaymentMethod   `json:"payment_method"`
	Notes              string          `json:"notes,omitempty"`
	CreatedAt          time.Time       `json:"created_at"`
	UpdatedAt          time.Time       `json:"updated_at"`
	StatusHistory      StatusHistories `json:"status_history"`
}

// TableName returns the table name for the Order model
func (Order) TableName() string {
	return "orders"
}

// AddStatusHistory adds a new status history entry
func (o *Order) AddStatusHistory(status OrderStatus, updatedBy, notes string) {
	o.Status = status
	o.UpdatedAt = time.Now()
	
	historyEntry := StatusHistory{
		Status:    status,
		UpdatedBy: updatedBy,
		Notes:     notes,
		Timestamp: time.Now(),
	}
	
	o.StatusHistory = append(o.StatusHistory, historyEntry)
}

// CalculateFees calculates platform and provider fees
func (o *Order) CalculateFees() {
	// Basic fee calculation (would be more complex in production)
	o.PlatformFee = o.TotalPrice * 0.1  // 10% platform fee
	o.ProviderFee = o.TotalPrice * 0.8  // 80% goes to provider
}

// Location represents a row in the locations table for tracking order movements
type OrderLocation struct {
	ID         string    `json:"id"`
	OrderID    string    `json:"order_id"`
	ProviderID string    `json:"provider_id"`
	Latitude   float64   `json:"latitude"`
	Longitude  float64   `json:"longitude"`
	Timestamp  time.Time `json:"timestamp"`
}

// TableName returns the table name for the OrderLocation model
func (OrderLocation) TableName() string {
	return "order_locations"
} 