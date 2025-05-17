package service

import (
	"context"
	"fmt"
	"math"
	"sort"
	"time"

	"github.com/order-api-microservices/services/order/internal/model"
)

// ProviderClient is an interface for interacting with the provider service
type ProviderClient interface {
	FindAvailableProviders(ctx context.Context, location model.Location, radius float64, serviceType string) ([]Provider, error)
	NotifyProvider(ctx context.Context, providerID string, orderID string, details interface{}) error
}

// Provider represents a service provider in the system
type Provider struct {
	ID           string         `json:"id"`
	Name         string         `json:"name"`
	Rating       float64        `json:"rating"`
	ServiceTypes []string       `json:"service_types"`
	Location     model.Location `json:"location"`
	IsAvailable  bool           `json:"is_available"`
	Distance     float64        `json:"distance,omitempty"` // Distance from requested location
}

// ProviderMatcher handles the matching of orders to providers
type ProviderMatcher struct {
	providerClient ProviderClient
}

// NewProviderMatcher creates a new provider matcher
func NewProviderMatcher(providerClient ProviderClient) *ProviderMatcher {
	return &ProviderMatcher{
		providerClient: providerClient,
	}
}

// FindBestProviders finds the best providers for an order based on location and service type
func (m *ProviderMatcher) FindBestProviders(ctx context.Context, order *model.Order, maxProviders int) ([]Provider, error) {
	// Convert order type to service type
	serviceType := orderTypeToServiceType(order.OrderType)
	
	// Search for providers within a 5km radius initially
	radius := 5.0 // kilometers
	
	// Get location from order (pickup location most of the time)
	location := order.PickupLocation
	
	// Find available providers from the provider service
	providers, err := m.providerClient.FindAvailableProviders(ctx, location, radius, serviceType)
	if err != nil {
		return nil, fmt.Errorf("failed to find providers: %w", err)
	}
	
	// If we don't have enough providers, increase the search radius
	if len(providers) < maxProviders {
		radius = 10.0 // kilometers
		providers, err = m.providerClient.FindAvailableProviders(ctx, location, radius, serviceType)
		if err != nil {
			return nil, fmt.Errorf("failed to find providers with increased radius: %w", err)
		}
	}
	
	// Sort providers by a weighted score of distance and rating
	sortProvidersByScore(providers)
	
	// Limit the number of providers
	if len(providers) > maxProviders {
		providers = providers[:maxProviders]
	}
	
	return providers, nil
}

// NotifyProviders sends notifications to providers about a new order
func (m *ProviderMatcher) NotifyProviders(ctx context.Context, order *model.Order, providers []Provider) error {
	for _, provider := range providers {
		// Create order details to send to provider
		orderDetails := map[string]interface{}{
			"order_id":             order.ID,
			"order_type":           order.OrderType,
			"pickup_location":      order.PickupLocation,
			"destination_location": order.DestinationLocation,
			"items_count":          len(order.Items),
			"total_price":          order.TotalPrice,
			"provider_fee":         order.ProviderFee,
			"created_at":           order.CreatedAt,
		}
		
		// Send notification to provider
		err := m.providerClient.NotifyProvider(ctx, provider.ID, order.ID, orderDetails)
		if err != nil {
			// Log error but continue with other providers
			fmt.Printf("Failed to notify provider %s: %v\n", provider.ID, err)
		}
	}
	
	return nil
}

// AssignProvider assigns a provider to an order
func (m *ProviderMatcher) AssignProvider(ctx context.Context, order *model.Order, providerID string) (*model.Order, error) {
	// Update order with provider ID
	order.ProviderID = providerID
	order.AddStatusHistory(model.StatusProviderAssigned, "system", fmt.Sprintf("Provider %s assigned", providerID))
	order.UpdatedAt = time.Now()
	
	return order, nil
}

// Helper functions

// orderTypeToServiceType converts an order type to a service type string
func orderTypeToServiceType(orderType model.OrderType) string {
	switch orderType {
	case model.TypeRide:
		return "ride"
	case model.TypeFoodDelivery:
		return "food_delivery"
	case model.TypePackageDelivery:
		return "package_delivery"
	case model.TypeGroceryDelivery:
		return "grocery_delivery"
	case model.TypeServiceBooking:
		return "service_booking"
	default:
		return "general"
	}
}

// sortProvidersByScore sorts providers by a weighted score of distance and rating
func sortProvidersByScore(providers []Provider) {
	// Sort providers by a weighted score of distance and rating
	sort.Slice(providers, func(i, j int) bool {
		// Calculate scores (lower is better for distance, higher is better for rating)
		// We normalize distance by assuming max 10km and rating is 0-5
		distanceScoreI := 1.0 - math.Min(providers[i].Distance/10.0, 1.0)
		distanceScoreJ := 1.0 - math.Min(providers[j].Distance/10.0, 1.0)
		
		ratingScoreI := providers[i].Rating / 5.0
		ratingScoreJ := providers[j].Rating / 5.0
		
		// Weighted score (70% distance, 30% rating)
		scoreI := 0.7*distanceScoreI + 0.3*ratingScoreI
		scoreJ := 0.7*distanceScoreJ + 0.3*ratingScoreJ
		
		return scoreI > scoreJ
	})
} 