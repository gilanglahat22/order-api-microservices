package model

import (
	"database/sql/driver"
	"encoding/json"
	"errors"
	"time"
)

// RecipientType defines the type of notification recipient
type RecipientType string

const (
	// RecipientTypeUser represents a user recipient
	RecipientTypeUser RecipientType = "USER"
	
	// RecipientTypeProvider represents a provider recipient
	RecipientTypeProvider RecipientType = "PROVIDER"
)

// NotificationType defines the type of notification
type NotificationType string

const (
	// NotificationTypeOrderCreated represents an order created notification
	NotificationTypeOrderCreated NotificationType = "ORDER_CREATED"
	
	// NotificationTypeOrderCancelled represents an order cancelled notification
	NotificationTypeOrderCancelled NotificationType = "ORDER_CANCELLED"
	
	// NotificationTypeOrderUpdated represents an order updated notification
	NotificationTypeOrderUpdated NotificationType = "ORDER_STATUS_UPDATED"
	
	// NotificationTypeProviderAssigned represents a provider assigned notification
	NotificationTypeProviderAssigned NotificationType = "PROVIDER_ASSIGNED"
	
	// NotificationTypeProviderArrived represents a provider arrived notification
	NotificationTypeProviderArrived NotificationType = "PROVIDER_ARRIVED"
	
	// NotificationTypePaymentProcessed represents a payment processed notification
	NotificationTypePaymentProcessed NotificationType = "PAYMENT_PROCESSED"
)

// Notification represents a notification in the system
type Notification struct {
	ID             string          `json:"id"`
	RecipientID    string          `json:"recipient_id"`
	RecipientType  RecipientType   `json:"recipient_type"`
	NotificationType NotificationType `json:"notification_type"`
	Title          string          `json:"title"`
	Message        string          `json:"message"`
	Payload        Payload         `json:"payload"`
	ReferenceID    string          `json:"reference_id"`
	Read           bool            `json:"read"`
	CreatedAt      time.Time       `json:"created_at"`
	ReadAt         *time.Time      `json:"read_at"`
}

// Payload is a map of string keys to interface{} values for flexible notification payloads
type Payload map[string]interface{}

// Value implements the driver.Valuer interface for JSON serialization
func (p Payload) Value() (driver.Value, error) {
	return json.Marshal(p)
}

// Scan implements the sql.Scanner interface for JSON deserialization
func (p *Payload) Scan(value interface{}) error {
	b, ok := value.([]byte)
	if !ok {
		return errors.New("type assertion to []byte failed")
	}
	return json.Unmarshal(b, p)
}

// TableName returns the table name for the Notification model
func (Notification) TableName() string {
	return "notifications"
} 