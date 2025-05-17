package service

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/order-api-microservices/services/provider/internal/model"
	"github.com/order-api-microservices/services/provider/internal/repository"
	pb "github.com/order-api-microservices/proto/provider"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/timestamppb"
)

// NotificationClient is an interface for interacting with the notification service
type NotificationClient interface {
	SendNotification(ctx context.Context, recipientID, notificationType string, payload interface{}) error
}

// ProviderService handles the business logic for providers
type ProviderService struct {
	pb.UnimplementedProviderServiceServer
	repo               *repository.ProviderRepository
	notificationClient NotificationClient
}

// NewProviderService creates a new provider service
func NewProviderService(repo *repository.ProviderRepository, notificationClient NotificationClient) *ProviderService {
	return &ProviderService{
		repo:               repo,
		notificationClient: notificationClient,
	}
}

// FindProviders finds providers near a location with specified service type
func (s *ProviderService) FindProviders(ctx context.Context, req *pb.FindProvidersRequest) (*pb.FindProvidersResponse, error) {
	if req.Location == nil {
		return nil, status.Errorf(codes.InvalidArgument, "location is required")
	}

	providers, err := s.repo.FindNearbyProviders(
		ctx,
		req.Location.Latitude,
		req.Location.Longitude,
		float64(req.Radius),
		req.ServiceType,
	)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "failed to find providers: %v", err)
	}

	// Convert providers to protobuf format
	protoProviders := make([]*pb.Provider, 0, len(providers))
	for _, provider := range providers {
		protoProviders = append(protoProviders, convertProviderToProto(provider))
	}

	return &pb.FindProvidersResponse{
		Providers: protoProviders,
		Success:   true,
		Message:   fmt.Sprintf("Found %d providers", len(protoProviders)),
	}, nil
}

// GetProvider gets a provider by ID
func (s *ProviderService) GetProvider(ctx context.Context, req *pb.GetProviderRequest) (*pb.GetProviderResponse, error) {
	if req.ProviderId == "" {
		return nil, status.Errorf(codes.InvalidArgument, "provider ID is required")
	}

	provider, err := s.repo.GetProviderByID(ctx, req.ProviderId)
	if err != nil {
		if errors.Is(err, repository.ErrProviderNotFound) {
			return nil, status.Errorf(codes.NotFound, "provider not found")
		}
		return nil, status.Errorf(codes.Internal, "failed to get provider: %v", err)
	}

	return &pb.GetProviderResponse{
		Provider: convertProviderToProto(provider),
		Success:  true,
		Message:  "Provider retrieved successfully",
	}, nil
}

// UpdateLocation updates a provider's location
func (s *ProviderService) UpdateLocation(ctx context.Context, req *pb.UpdateLocationRequest) (*pb.UpdateLocationResponse, error) {
	if req.ProviderId == "" {
		return nil, status.Errorf(codes.InvalidArgument, "provider ID is required")
	}
	if req.Location == nil {
		return nil, status.Errorf(codes.InvalidArgument, "location is required")
	}

	location := model.Location{
		Latitude:  req.Location.Latitude,
		Longitude: req.Location.Longitude,
		Address:   req.Location.Address,
	}

	err := s.repo.UpdateProviderLocation(ctx, req.ProviderId, location)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "failed to update location: %v", err)
	}

	return &pb.UpdateLocationResponse{
		Success: true,
		Message: "Location updated successfully",
	}, nil
}

// NotifyProvider sends a notification to a provider
func (s *ProviderService) NotifyProvider(ctx context.Context, req *pb.NotifyProviderRequest) (*pb.NotifyProviderResponse, error) {
	if req.ProviderId == "" || req.OrderId == "" {
		return nil, status.Errorf(codes.InvalidArgument, "provider ID and order ID are required")
	}

	// Verify the provider exists
	_, err := s.repo.GetProviderByID(ctx, req.ProviderId)
	if err != nil {
		if errors.Is(err, repository.ErrProviderNotFound) {
			return nil, status.Errorf(codes.NotFound, "provider not found")
		}
		return nil, status.Errorf(codes.Internal, "failed to get provider: %v", err)
	}

	// Parse the details
	var details map[string]interface{}
	if req.Details != "" {
		if err := json.Unmarshal([]byte(req.Details), &details); err != nil {
			return nil, status.Errorf(codes.InvalidArgument, "invalid details format: %v", err)
		}
	}

	// Add order ID and notification type to details
	if details == nil {
		details = make(map[string]interface{})
	}
	details["order_id"] = req.OrderId
	details["notification_type"] = req.NotificationType

	// Send notification through notification service if available
	if s.notificationClient != nil {
		err := s.notificationClient.SendNotification(ctx, req.ProviderId, req.NotificationType, details)
		if err != nil {
			// Log error but continue - this should not fail the API call
			fmt.Printf("Failed to send notification to provider %s: %v\n", req.ProviderId, err)
		}
	}

	return &pb.NotifyProviderResponse{
		Success: true,
		Message: "Notification sent successfully",
	}, nil
}

// UpdateAvailability updates a provider's availability status
func (s *ProviderService) UpdateAvailability(ctx context.Context, req *pb.UpdateAvailabilityRequest) (*pb.UpdateAvailabilityResponse, error) {
	if req.ProviderId == "" {
		return nil, status.Errorf(codes.InvalidArgument, "provider ID is required")
	}

	err := s.repo.UpdateProviderAvailability(ctx, req.ProviderId, req.IsAvailable)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "failed to update availability: %v", err)
	}

	return &pb.UpdateAvailabilityResponse{
		Success: true,
		Message: fmt.Sprintf("Provider is now %s", availabilityStatusString(req.IsAvailable)),
	}, nil
}

// UpdateProfile updates a provider's profile information
func (s *ProviderService) UpdateProfile(ctx context.Context, req *pb.UpdateProfileRequest) (*pb.UpdateProfileResponse, error) {
	if req.ProviderId == "" || req.Profile == nil {
		return nil, status.Errorf(codes.InvalidArgument, "provider ID and profile are required")
	}

	// Get current provider
	provider, err := s.repo.GetProviderByID(ctx, req.ProviderId)
	if err != nil {
		if errors.Is(err, repository.ErrProviderNotFound) {
			return nil, status.Errorf(codes.NotFound, "provider not found")
		}
		return nil, status.Errorf(codes.Internal, "failed to get provider: %v", err)
	}

	// Update the provider with new information
	provider.Name = req.Profile.Name
	provider.Email = req.Profile.Email
	provider.Phone = req.Profile.Phone
	if req.Profile.ServiceTypes != nil {
		provider.ServiceTypes = req.Profile.ServiceTypes
	}
	provider.ProfileImage = req.Profile.ProfileImage

	// Convert metadata from protobuf to model
	if req.Profile.Metadata != nil {
		metadata := make(model.Metadata)
		for k, v := range req.Profile.Metadata {
			metadata[k] = v
		}
		provider.Metadata = metadata
	}

	// Save changes
	err = s.repo.UpdateProvider(ctx, provider)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "failed to update provider profile: %v", err)
	}

	return &pb.UpdateProfileResponse{
		Success: true,
		Message: "Provider profile updated successfully",
	}, nil
}

// ListOrders lists orders for a specific provider (stub implementation)
func (s *ProviderService) ListOrders(ctx context.Context, req *pb.ListOrdersRequest) (*pb.ListOrdersResponse, error) {
	// This would typically call the order service or query a local orders cache
	// For now, return a minimal response
	return &pb.ListOrdersResponse{
		Orders:  []*pb.OrderSummary{},
		Total:   0,
		Page:    req.Page,
		Limit:   req.Limit,
		Success: true,
		Message: "No orders found",
	}, nil
}

// Helper functions

// Convert provider model to protobuf
func convertProviderToProto(provider *model.Provider) *pb.Provider {
	metadata := make(map[string]string)
	for k, v := range provider.Metadata {
		metadata[k] = v
	}

	return &pb.Provider{
		Id:           provider.ID,
		Name:         provider.Name,
		Rating:       float32(provider.Rating),
		ServiceTypes: provider.ServiceTypes,
		Location: &pb.Location{
			Latitude:  provider.Location.Latitude,
			Longitude: provider.Location.Longitude,
			Address:   provider.Location.Address,
		},
		IsAvailable:  provider.IsAvailable,
		Email:        provider.Email,
		Phone:        provider.Phone,
		ProfileImage: provider.ProfileImage,
		Metadata:     metadata,
		CreatedAt:    timestamppb.New(provider.CreatedAt),
		UpdatedAt:    timestamppb.New(provider.UpdatedAt),
	}
}

// Helper to convert availability boolean to string
func availabilityStatusString(isAvailable bool) string {
	if isAvailable {
		return "available"
	}
	return "unavailable"
} 