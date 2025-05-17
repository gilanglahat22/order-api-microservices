package clients

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/order-api-microservices/services/order/internal/model"
	"github.com/order-api-microservices/services/order/internal/service"
	pb "github.com/order-api-microservices/proto/provider"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

// ProviderGRPCClient is a client for the provider service
type ProviderGRPCClient struct {
	client pb.ProviderServiceClient
	conn   *grpc.ClientConn
}

// NewProviderGRPCClient creates a new provider service client
func NewProviderGRPCClient(address string) (*ProviderGRPCClient, error) {
	conn, err := grpc.Dial(address, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		return nil, fmt.Errorf("failed to connect to provider service: %v", err)
	}

	client := pb.NewProviderServiceClient(conn)
	return &ProviderGRPCClient{
		client: client,
		conn:   conn,
	}, nil
}

// Close closes the connection to the provider service
func (c *ProviderGRPCClient) Close() error {
	if c.conn != nil {
		return c.conn.Close()
	}
	return nil
}

// FindAvailableProviders finds available providers near a location
func (c *ProviderGRPCClient) FindAvailableProviders(ctx context.Context, location model.Location, radius float64, serviceType string) ([]service.Provider, error) {
	// Create the request
	req := &pb.FindProvidersRequest{
		Location: &pb.Location{
			Latitude:  location.Latitude,
			Longitude: location.Longitude,
			Address:   location.Address,
		},
		Radius:      float32(radius),
		ServiceType: serviceType,
	}

	// Set context with timeout
	ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()

	// Call the service
	resp, err := c.client.FindProviders(ctx, req)
	if err != nil {
		return nil, fmt.Errorf("failed to find providers: %v", err)
	}

	if !resp.Success {
		return nil, fmt.Errorf("provider service failed to find providers: %s", resp.Message)
	}

	// Convert response to providers
	providers := make([]service.Provider, 0, len(resp.Providers))
	for _, p := range resp.Providers {
		serviceTypes := make([]string, len(p.ServiceTypes))
		for i, st := range p.ServiceTypes {
			serviceTypes[i] = st
		}

		provider := service.Provider{
			ID:           p.Id,
			Name:         p.Name,
			Rating:       float64(p.Rating),
			ServiceTypes: serviceTypes,
			Location: model.Location{
				Latitude:  p.Location.Latitude,
				Longitude: p.Location.Longitude,
				Address:   p.Location.Address,
			},
			IsAvailable: p.IsAvailable,
			Distance:    float64(p.Distance),
		}
		providers = append(providers, provider)
	}

	return providers, nil
}

// NotifyProvider notifies a provider about a new order
func (c *ProviderGRPCClient) NotifyProvider(ctx context.Context, providerID string, orderID string, details interface{}) error {
	// Convert details to JSON
	detailsBytes, err := json.Marshal(details)
	if err != nil {
		return fmt.Errorf("failed to marshal order details: %v", err)
	}

	// Create the request
	req := &pb.NotifyProviderRequest{
		ProviderId: providerID,
		OrderId:    orderID,
		Details:    string(detailsBytes),
		NotificationType: "NEW_ORDER",
	}

	// Set context with timeout
	ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()

	// Call the service
	resp, err := c.client.NotifyProvider(ctx, req)
	if err != nil {
		return fmt.Errorf("failed to notify provider: %v", err)
	}

	if !resp.Success {
		return fmt.Errorf("provider service failed to notify provider: %s", resp.Message)
	}

	return nil
}

// UpdateProviderLocation updates the location of a provider
func (c *ProviderGRPCClient) UpdateProviderLocation(ctx context.Context, providerID string, location model.Location) error {
	// Create the request
	req := &pb.UpdateLocationRequest{
		ProviderId: providerID,
		Location: &pb.Location{
			Latitude:  location.Latitude,
			Longitude: location.Longitude,
			Address:   location.Address,
		},
	}

	// Set context with timeout
	ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()

	// Call the service
	resp, err := c.client.UpdateLocation(ctx, req)
	if err != nil {
		return fmt.Errorf("failed to update provider location: %v", err)
	}

	if !resp.Success {
		return fmt.Errorf("provider service failed to update location: %s", resp.Message)
	}

	return nil
}

// GetProviderDetails gets details about a provider
func (c *ProviderGRPCClient) GetProviderDetails(ctx context.Context, providerID string) (*service.Provider, error) {
	// Create the request
	req := &pb.GetProviderRequest{
		ProviderId: providerID,
	}

	// Set context with timeout
	ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()

	// Call the service
	resp, err := c.client.GetProvider(ctx, req)
	if err != nil {
		return nil, fmt.Errorf("failed to get provider details: %v", err)
	}

	serviceTypes := make([]string, len(resp.Provider.ServiceTypes))
	for i, st := range resp.Provider.ServiceTypes {
		serviceTypes[i] = st
	}

	// Convert response to provider
	provider := &service.Provider{
		ID:           resp.Provider.Id,
		Name:         resp.Provider.Name,
		Rating:       float64(resp.Provider.Rating),
		ServiceTypes: serviceTypes,
		Location: model.Location{
			Latitude:  resp.Provider.Location.Latitude,
			Longitude: resp.Provider.Location.Longitude,
			Address:   resp.Provider.Location.Address,
		},
		IsAvailable: resp.Provider.IsAvailable,
	}

	return provider, nil
} 