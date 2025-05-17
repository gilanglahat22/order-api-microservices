package service

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/order-api-microservices/services/order/internal/model"
	"github.com/order-api-microservices/services/order/internal/repository"
	pb "github.com/order-api-microservices/proto/order"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/timestamppb"
)

// BlockchainClient is an interface for interacting with the blockchain service
type BlockchainClient interface {
	RecordOrder(ctx context.Context, orderID, userID, providerID string, orderData interface{}) (string, error)
	VerifyOrder(ctx context.Context, orderID, txHash string) (bool, error)
}

// ProviderClient is an interface for interacting with the provider service
type ProviderClient interface {
	FindBestProviders(ctx context.Context, order *model.Order, count int) ([]Provider, error)
	NotifyProviders(ctx context.Context, order *model.Order, providers []Provider) error
}

// OrderService handles the business logic for orders
type OrderService struct {
	pb.UnimplementedOrderServiceServer
	repo               *repository.OrderRepository
	locationRepo       *repository.OrderLocationRepository
	blockchainClient   BlockchainClient
	providerClient     ProviderClient
	providerMatcher    *ProviderMatcher
}

// NewOrderService creates a new order service
func NewOrderService(
	repo *repository.OrderRepository,
	locationRepo *repository.OrderLocationRepository,
	blockchainClient BlockchainClient,
	providerClient ProviderClient,
) *OrderService {
	providerMatcher := NewProviderMatcher(providerClient)
	
	return &OrderService{
		repo:               repo,
		locationRepo:       locationRepo,
		blockchainClient:   blockchainClient,
		providerClient:     providerClient,
		providerMatcher:    providerMatcher,
	}
}

// CreateOrder creates a new order
func (s *OrderService) CreateOrder(ctx context.Context, req *pb.CreateOrderRequest) (*pb.OrderResponse, error) {
	// Validate the request
	if req.UserId == "" {
		return nil, status.Errorf(codes.InvalidArgument, "user ID is required")
	}
	if req.PickupLocation == nil || req.DestinationLocation == nil {
		return nil, status.Errorf(codes.InvalidArgument, "pickup and destination locations are required")
	}

	// Create new order
	orderID := uuid.New().String()
	now := time.Now()
	
	// Initialize order with data from request
	order := &model.Order{
		ID:                 orderID,
		UserID:             req.UserId,
		OrderType:          convertOrderType(req.OrderType),
		Status:             model.StatusCreated,
		PickupLocation:     convertLocation(req.PickupLocation),
		DestinationLocation: convertLocation(req.DestinationLocation),
		Items:              convertOrderItems(req.Items),
		PaymentMethod:      convertPaymentMethod(req.PaymentMethod),
		Notes:              req.Notes,
		CreatedAt:          now,
		UpdatedAt:          now,
	}

	// Calculate total price and fees
	order.TotalPrice = calculateTotalPrice(order.Items)
	order.CalculateFees()

	// Add initial status history
	order.StatusHistory = []model.StatusHistory{
		{
			Status:    model.StatusCreated,
			UpdatedBy: "system",
			Notes:     "Order created",
			Timestamp: now,
		},
	}

	// Store order in database
	err := s.repo.CreateOrder(ctx, order)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "failed to create order: %v", err)
	}

	// Record order on blockchain
	go func() {
		// Using background context for async operation
		bCtx := context.Background()
		txHash, err := s.blockchainClient.RecordOrder(bCtx, order.ID, order.UserID, order.ProviderID, order)
		if err != nil {
			// In production, would use a retry mechanism or queue
			fmt.Printf("Failed to record order on blockchain: %v\n", err)
			return
		}

		// Update order with blockchain transaction hash
		order.BlockchainTxHash = txHash
		if err := s.repo.UpdateOrder(bCtx, order); err != nil {
			fmt.Printf("Failed to update order with blockchain hash: %v\n", err)
		}
	}()

	// Build response
	response := &pb.OrderResponse{
		Order:   convertOrderToProto(order),
		Message: "Order created successfully",
		Success: true,
	}

	return response, nil
}

// GetOrder retrieves an order by ID
func (s *OrderService) GetOrder(ctx context.Context, req *pb.GetOrderRequest) (*pb.OrderResponse, error) {
	if req.OrderId == "" {
		return nil, status.Errorf(codes.InvalidArgument, "order ID is required")
	}

	order, err := s.repo.GetOrderByID(ctx, req.OrderId)
	if err != nil {
		if errors.Is(err, repository.ErrOrderNotFound) {
			return nil, status.Errorf(codes.NotFound, "order not found")
		}
		return nil, status.Errorf(codes.Internal, "failed to get order: %v", err)
	}

	return &pb.OrderResponse{
		Order:   convertOrderToProto(order),
		Message: "Order retrieved successfully",
		Success: true,
	}, nil
}

// UpdateOrderStatus updates the status of an order
func (s *OrderService) UpdateOrderStatus(ctx context.Context, req *pb.UpdateOrderStatusRequest) (*pb.OrderResponse, error) {
	if req.OrderId == "" {
		return nil, status.Errorf(codes.InvalidArgument, "order ID is required")
	}

	// Get current order
	order, err := s.repo.GetOrderByID(ctx, req.OrderId)
	if err != nil {
		if errors.Is(err, repository.ErrOrderNotFound) {
			return nil, status.Errorf(codes.NotFound, "order not found")
		}
		return nil, status.Errorf(codes.Internal, "failed to get order: %v", err)
	}

	// Update order status
	newStatus := convertOrderStatusFromProto(req.Status)
	err = s.repo.UpdateOrderStatus(ctx, req.OrderId, newStatus, req.UpdatedBy, req.Notes)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "failed to update order status: %v", err)
	}

	// Get updated order
	updatedOrder, err := s.repo.GetOrderByID(ctx, req.OrderId)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "failed to get updated order: %v", err)
	}

	// Record status change on blockchain
	go func() {
		bCtx := context.Background()
		txHash, err := s.blockchainClient.RecordOrder(bCtx, updatedOrder.ID, updatedOrder.UserID, updatedOrder.ProviderID, updatedOrder)
		if err != nil {
			fmt.Printf("Failed to record order status change on blockchain: %v\n", err)
			return
		}

		// Update order with new blockchain transaction hash
		updatedOrder.BlockchainTxHash = txHash
		if err := s.repo.UpdateOrder(bCtx, updatedOrder); err != nil {
			fmt.Printf("Failed to update order with blockchain hash: %v\n", err)
		}
	}()

	return &pb.OrderResponse{
		Order:   convertOrderToProto(updatedOrder),
		Message: "Order status updated successfully",
		Success: true,
	}, nil
}

// CancelOrder cancels an order
func (s *OrderService) CancelOrder(ctx context.Context, req *pb.CancelOrderRequest) (*pb.OrderResponse, error) {
	if req.OrderId == "" {
		return nil, status.Errorf(codes.InvalidArgument, "order ID is required")
	}

	// Get current order
	order, err := s.repo.GetOrderByID(ctx, req.OrderId)
	if err != nil {
		if errors.Is(err, repository.ErrOrderNotFound) {
			return nil, status.Errorf(codes.NotFound, "order not found")
		}
		return nil, status.Errorf(codes.Internal, "failed to get order: %v", err)
	}

	// Check if order can be cancelled
	if order.Status == model.StatusCompleted || 
	   order.Status == model.StatusCancelled || 
	   order.Status == model.StatusRefunded {
		return nil, status.Errorf(codes.FailedPrecondition, "order cannot be cancelled in its current state")
	}

	// Update order status to cancelled
	err = s.repo.UpdateOrderStatus(ctx, req.OrderId, model.StatusCancelled, req.CancelledBy, req.Reason)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "failed to cancel order: %v", err)
	}

	// Get updated order
	updatedOrder, err := s.repo.GetOrderByID(ctx, req.OrderId)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "failed to get updated order: %v", err)
	}

	// Record cancellation on blockchain
	go func() {
		bCtx := context.Background()
		txHash, err := s.blockchainClient.RecordOrder(bCtx, updatedOrder.ID, updatedOrder.UserID, updatedOrder.ProviderID, updatedOrder)
		if err != nil {
			fmt.Printf("Failed to record order cancellation on blockchain: %v\n", err)
			return
		}

		// Update order with new blockchain transaction hash
		updatedOrder.BlockchainTxHash = txHash
		if err := s.repo.UpdateOrder(bCtx, updatedOrder); err != nil {
			fmt.Printf("Failed to update order with blockchain hash: %v\n", err)
		}
	}()

	return &pb.OrderResponse{
		Order:   convertOrderToProto(updatedOrder),
		Message: "Order cancelled successfully",
		Success: true,
	}, nil
}

// ListUserOrders lists orders for a specific user
func (s *OrderService) ListUserOrders(ctx context.Context, req *pb.ListUserOrdersRequest) (*pb.ListOrdersResponse, error) {
	if req.UserId == "" {
		return nil, status.Errorf(codes.InvalidArgument, "user ID is required")
	}

	var status model.OrderStatus
	if req.Status != pb.OrderStatus_ORDER_STATUS_UNSPECIFIED {
		status = convertOrderStatusFromProto(req.Status)
	}

	orders, total, err := s.repo.ListUserOrders(ctx, req.UserId, int(req.Page), int(req.Limit), status)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "failed to list user orders: %v", err)
	}

	// Convert orders to protobuf format
	protoOrders := []*pb.Order{}
	for _, order := range orders {
		protoOrders = append(protoOrders, convertOrderToProto(order))
	}

	return &pb.ListOrdersResponse{
		Orders: protoOrders,
		Total:  int32(total),
		Page:   req.Page,
		Limit:  req.Limit,
	}, nil
}

// ListProviderOrders lists orders for a specific provider
func (s *OrderService) ListProviderOrders(ctx context.Context, req *pb.ListProviderOrdersRequest) (*pb.ListOrdersResponse, error) {
	if req.ProviderId == "" {
		return nil, status.Errorf(codes.InvalidArgument, "provider ID is required")
	}

	var status model.OrderStatus
	if req.Status != pb.OrderStatus_ORDER_STATUS_UNSPECIFIED {
		status = convertOrderStatusFromProto(req.Status)
	}

	orders, total, err := s.repo.ListProviderOrders(ctx, req.ProviderId, int(req.Page), int(req.Limit), status)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "failed to list provider orders: %v", err)
	}

	// Convert orders to protobuf format
	protoOrders := []*pb.Order{}
	for _, order := range orders {
		protoOrders = append(protoOrders, convertOrderToProto(order))
	}

	return &pb.ListOrdersResponse{
		Orders: protoOrders,
		Total:  int32(total),
		Page:   req.Page,
		Limit:  req.Limit,
	}, nil
}

// TrackOrder streams real-time updates of an order's location
func (s *OrderService) TrackOrder(req *pb.TrackOrderRequest, stream pb.OrderService_TrackOrderServer) error {
	if req.OrderId == "" {
		return status.Errorf(codes.InvalidArgument, "order ID is required")
	}
	
	// Get order to verify it exists
	order, err := s.repo.GetOrderByID(stream.Context(), req.OrderId)
	if err != nil {
		if errors.Is(err, repository.ErrOrderNotFound) {
			return status.Errorf(codes.NotFound, "order not found")
		}
		return status.Errorf(codes.Internal, "failed to get order: %v", err)
	}
	
	// Create a ticker to poll for updates
	ticker := time.NewTicker(5 * time.Second)
	defer ticker.Stop()
	
	// Keep track of the last location sent to avoid duplicates
	var lastLocationID string
	
	for {
		select {
		case <-ticker.C:
			// Get latest location
			location, err := s.locationRepo.GetLatestOrderLocation(stream.Context(), req.OrderId)
			if err != nil {
				if errors.Is(err, repository.ErrOrderLocationNotFound) {
					// No location updates yet, just continue
					continue
				}
				fmt.Printf("Error getting latest location: %v\n", err)
				continue
			}
			
			// Skip if this is the same location we already sent
			if location.ID == lastLocationID {
				continue
			}
			
			// Update last location ID
			lastLocationID = location.ID
			
			// Get latest order status
			currentOrder, err := s.repo.GetOrderByID(stream.Context(), req.OrderId)
			if err != nil {
				fmt.Printf("Error getting current order: %v\n", err)
				continue
			}
			
			// Calculate ETA
			var estimatedArrivalMinutes float32
			if currentOrder.Status == model.StatusInTransit || currentOrder.Status == model.StatusPickedUp {
				estimatedArrivalMinutes = estimateArrivalMinutes(location, currentOrder.DestinationLocation)
			} else {
				estimatedArrivalMinutes = estimateArrivalMinutes(location, currentOrder.PickupLocation)
			}
			
			// Create update
			update := &pb.OrderLocationUpdate{
				OrderId:    req.OrderId,
				ProviderId: location.ProviderID,
				CurrentLocation: &pb.Location{
					Latitude:  location.Latitude,
					Longitude: location.Longitude,
				},
				EstimatedArrivalMinutes: estimatedArrivalMinutes,
				Timestamp:              timestamppb.New(location.Timestamp),
			}
			
			// Send update to client
			if err := stream.Send(update); err != nil {
				return status.Errorf(codes.Internal, "failed to send update: %v", err)
			}
			
		case <-stream.Context().Done():
			return nil
		}
	}
}

// Helper functions for conversions between domain models and protocol buffer messages

func convertOrderType(ot pb.OrderType) model.OrderType {
	switch ot {
	case pb.OrderType_ORDER_TYPE_RIDE:
		return model.TypeRide
	case pb.OrderType_ORDER_TYPE_FOOD_DELIVERY:
		return model.TypeFoodDelivery
	case pb.OrderType_ORDER_TYPE_PACKAGE_DELIVERY:
		return model.TypePackageDelivery
	case pb.OrderType_ORDER_TYPE_GROCERY_DELIVERY:
		return model.TypeGroceryDelivery
	case pb.OrderType_ORDER_TYPE_SERVICE_BOOKING:
		return model.TypeServiceBooking
	default:
		return model.TypeRide
	}
}

func convertOrderTypeToProto(ot model.OrderType) pb.OrderType {
	switch ot {
	case model.TypeRide:
		return pb.OrderType_ORDER_TYPE_RIDE
	case model.TypeFoodDelivery:
		return pb.OrderType_ORDER_TYPE_FOOD_DELIVERY
	case model.TypePackageDelivery:
		return pb.OrderType_ORDER_TYPE_PACKAGE_DELIVERY
	case model.TypeGroceryDelivery:
		return pb.OrderType_ORDER_TYPE_GROCERY_DELIVERY
	case model.TypeServiceBooking:
		return pb.OrderType_ORDER_TYPE_SERVICE_BOOKING
	default:
		return pb.OrderType_ORDER_TYPE_UNSPECIFIED
	}
}

func convertOrderStatusFromProto(os pb.OrderStatus) model.OrderStatus {
	switch os {
	case pb.OrderStatus_ORDER_STATUS_CREATED:
		return model.StatusCreated
	case pb.OrderStatus_ORDER_STATUS_PAYMENT_PENDING:
		return model.StatusPaymentPending
	case pb.OrderStatus_ORDER_STATUS_PAYMENT_COMPLETED:
		return model.StatusPaymentComplete
	case pb.OrderStatus_ORDER_STATUS_PROVIDER_ASSIGNED:
		return model.StatusProviderAssigned
	case pb.OrderStatus_ORDER_STATUS_PROVIDER_ACCEPTED:
		return model.StatusProviderAccepted
	case pb.OrderStatus_ORDER_STATUS_PROVIDER_REJECTED:
		return model.StatusProviderRejected
	case pb.OrderStatus_ORDER_STATUS_IN_PROGRESS:
		return model.StatusInProgress
	case pb.OrderStatus_ORDER_STATUS_PICKED_UP:
		return model.StatusPickedUp
	case pb.OrderStatus_ORDER_STATUS_IN_TRANSIT:
		return model.StatusInTransit
	case pb.OrderStatus_ORDER_STATUS_ARRIVED:
		return model.StatusArrived
	case pb.OrderStatus_ORDER_STATUS_DELIVERED:
		return model.StatusDelivered
	case pb.OrderStatus_ORDER_STATUS_COMPLETED:
		return model.StatusCompleted
	case pb.OrderStatus_ORDER_STATUS_CANCELLED:
		return model.StatusCancelled
	case pb.OrderStatus_ORDER_STATUS_REFUNDED:
		return model.StatusRefunded
	case pb.OrderStatus_ORDER_STATUS_DISPUTED:
		return model.StatusDisputed
	default:
		return model.StatusCreated
	}
}

func convertOrderStatusToProto(os model.OrderStatus) pb.OrderStatus {
	switch os {
	case model.StatusCreated:
		return pb.OrderStatus_ORDER_STATUS_CREATED
	case model.StatusPaymentPending:
		return pb.OrderStatus_ORDER_STATUS_PAYMENT_PENDING
	case model.StatusPaymentComplete:
		return pb.OrderStatus_ORDER_STATUS_PAYMENT_COMPLETED
	case model.StatusProviderAssigned:
		return pb.OrderStatus_ORDER_STATUS_PROVIDER_ASSIGNED
	case model.StatusProviderAccepted:
		return pb.OrderStatus_ORDER_STATUS_PROVIDER_ACCEPTED
	case model.StatusProviderRejected:
		return pb.OrderStatus_ORDER_STATUS_PROVIDER_REJECTED
	case model.StatusInProgress:
		return pb.OrderStatus_ORDER_STATUS_IN_PROGRESS
	case model.StatusPickedUp:
		return pb.OrderStatus_ORDER_STATUS_PICKED_UP
	case model.StatusInTransit:
		return pb.OrderStatus_ORDER_STATUS_IN_TRANSIT
	case model.StatusArrived:
		return pb.OrderStatus_ORDER_STATUS_ARRIVED
	case model.StatusDelivered:
		return pb.OrderStatus_ORDER_STATUS_DELIVERED
	case model.StatusCompleted:
		return pb.OrderStatus_ORDER_STATUS_COMPLETED
	case model.StatusCancelled:
		return pb.OrderStatus_ORDER_STATUS_CANCELLED
	case model.StatusRefunded:
		return pb.OrderStatus_ORDER_STATUS_REFUNDED
	case model.StatusDisputed:
		return pb.OrderStatus_ORDER_STATUS_DISPUTED
	default:
		return pb.OrderStatus_ORDER_STATUS_UNSPECIFIED
	}
}

func convertPaymentMethod(pm pb.PaymentMethod) model.PaymentMethod {
	switch pm {
	case pb.PaymentMethod_PAYMENT_METHOD_CREDIT_CARD:
		return model.PaymentCreditCard
	case pb.PaymentMethod_PAYMENT_METHOD_DEBIT_CARD:
		return model.PaymentDebitCard
	case pb.PaymentMethod_PAYMENT_METHOD_DIGITAL_WALLET:
		return model.PaymentDigitalWallet
	case pb.PaymentMethod_PAYMENT_METHOD_CASH:
		return model.PaymentCash
	case pb.PaymentMethod_PAYMENT_METHOD_CRYPTO:
		return model.PaymentCrypto
	default:
		return model.PaymentCreditCard
	}
}

func convertPaymentMethodToProto(pm model.PaymentMethod) pb.PaymentMethod {
	switch pm {
	case model.PaymentCreditCard:
		return pb.PaymentMethod_PAYMENT_METHOD_CREDIT_CARD
	case model.PaymentDebitCard:
		return pb.PaymentMethod_PAYMENT_METHOD_DEBIT_CARD
	case model.PaymentDigitalWallet:
		return pb.PaymentMethod_PAYMENT_METHOD_DIGITAL_WALLET
	case model.PaymentCash:
		return pb.PaymentMethod_PAYMENT_METHOD_CASH
	case model.PaymentCrypto:
		return pb.PaymentMethod_PAYMENT_METHOD_CRYPTO
	default:
		return pb.PaymentMethod_PAYMENT_METHOD_UNSPECIFIED
	}
}

func convertLocation(loc *pb.Location) model.Location {
	if loc == nil {
		return model.Location{}
	}

	additionalInfo := make(map[string]string)
	for k, v := range loc.AdditionalInfo {
		additionalInfo[k] = v
	}

	return model.Location{
		Latitude:      loc.Latitude,
		Longitude:     loc.Longitude,
		Address:       loc.Address,
		PostalCode:    loc.PostalCode,
		City:          loc.City,
		Country:       loc.Country,
		AdditionalInfo: additionalInfo,
	}
}

func convertLocationToProto(loc model.Location) *pb.Location {
	additionalInfo := make(map[string]string)
	for k, v := range loc.AdditionalInfo {
		additionalInfo[k] = v
	}

	return &pb.Location{
		Latitude:      loc.Latitude,
		Longitude:     loc.Longitude,
		Address:       loc.Address,
		PostalCode:    loc.PostalCode,
		City:          loc.City,
		Country:       loc.Country,
		AdditionalInfo: additionalInfo,
	}
}

func convertOrderItems(items []*pb.OrderItem) model.OrderItems {
	orderItems := model.OrderItems{}
	for _, item := range items {
		properties := make(map[string]string)
		for k, v := range item.Properties {
			properties[k] = v
		}

		orderItems = append(orderItems, model.OrderItem{
			ItemID:     item.ItemId,
			Name:       item.Name,
			Quantity:   int(item.Quantity),
			Price:      float64(item.Price),
			Properties: properties,
		})
	}
	return orderItems
}

func convertOrderItemsToProto(items model.OrderItems) []*pb.OrderItem {
	protoItems := []*pb.OrderItem{}
	for _, item := range items {
		properties := make(map[string]string)
		for k, v := range item.Properties {
			properties[k] = v
		}

		protoItems = append(protoItems, &pb.OrderItem{
			ItemId:     item.ItemID,
			Name:       item.Name,
			Quantity:   int32(item.Quantity),
			Price:      float32(item.Price),
			Properties: properties,
		})
	}
	return protoItems
}

func convertStatusHistoryToProto(history model.StatusHistories) []*pb.OrderStatusHistory {
	protoHistory := []*pb.OrderStatusHistory{}
	for _, h := range history {
		protoHistory = append(protoHistory, &pb.OrderStatusHistory{
			Status:    convertOrderStatusToProto(h.Status),
			UpdatedBy: h.UpdatedBy,
			Notes:     h.Notes,
			Timestamp: timestamppb.New(h.Timestamp),
		})
	}
	return protoHistory
}

func convertOrderToProto(order *model.Order) *pb.Order {
	return &pb.Order{
		Id:                  order.ID,
		UserId:              order.UserID,
		ProviderId:          order.ProviderID,
		OrderType:           convertOrderTypeToProto(order.OrderType),
		Status:              convertOrderStatusToProto(order.Status),
		PickupLocation:      convertLocationToProto(order.PickupLocation),
		DestinationLocation: convertLocationToProto(order.DestinationLocation),
		Items:               convertOrderItemsToProto(order.Items),
		TotalPrice:          float32(order.TotalPrice),
		PlatformFee:         float32(order.PlatformFee),
		ProviderFee:         float32(order.ProviderFee),
		TransactionId:       order.TransactionID,
		BlockchainTxHash:    order.BlockchainTxHash,
		PaymentMethod:       convertPaymentMethodToProto(order.PaymentMethod),
		Notes:               order.Notes,
		CreatedAt:           timestamppb.New(order.CreatedAt),
		UpdatedAt:           timestamppb.New(order.UpdatedAt),
		StatusHistory:       convertStatusHistoryToProto(order.StatusHistory),
	}
}

func calculateTotalPrice(items model.OrderItems) float64 {
	var total float64
	for _, item := range items {
		total += item.Price * float64(item.Quantity)
	}
	return total
}

// estimateArrivalMinutes is a simplified function that estimates arrival time
// In a real implementation, this would use a routing service or algorithm
func estimateArrivalMinutes(location *model.OrderLocation, destination model.Location) float32 {
	// This is a very simplified estimation
	// In reality, you would use a distance matrix API or routing engine
	
	// Haversine distance (simplified)
	dLat := destination.Latitude - location.Latitude
	dLon := destination.Longitude - location.Longitude
	
	// Simplified distance calculation (not accurate for large distances)
	distance := (dLat*dLat + dLon*dLon) * 111.0 // Approximate km per degree at the equator
	
	// Assume average speed of 30 km/h
	averageSpeed := 30.0 
	
	// Calculate estimated time in minutes
	estimatedMinutes := (distance / averageSpeed) * 60.0
	
	return float32(estimatedMinutes)
}

// AssignProvider assigns a provider to an order
func (s *OrderService) AssignProvider(ctx context.Context, req *pb.AssignProviderRequest) (*pb.OrderResponse, error) {
	if req.OrderId == "" {
		return nil, status.Errorf(codes.InvalidArgument, "order ID is required")
	}
	
	// Get current order
	order, err := s.repo.GetOrderByID(ctx, req.OrderId)
	if err != nil {
		if errors.Is(err, repository.ErrOrderNotFound) {
			return nil, status.Errorf(codes.NotFound, "order not found")
		}
		return nil, status.Errorf(codes.Internal, "failed to get order: %v", err)
	}
	
	var providers []Provider
	var selectedProviderID string
	
	if req.ProviderId != "" {
		// Manual provider assignment
		selectedProviderID = req.ProviderId
	} else {
		// Auto-match providers
		providers, err = s.providerMatcher.FindBestProviders(ctx, order, 3)
		if err != nil {
			return nil, status.Errorf(codes.Internal, "failed to find providers: %v", err)
		}
		
		if len(providers) == 0 {
			return nil, status.Errorf(codes.NotFound, "no available providers found")
		}
		
		// Notify all providers about the order
		err = s.providerMatcher.NotifyProviders(ctx, order, providers)
		if err != nil {
			// Log but continue - we still want to assign the order
			fmt.Printf("Failed to notify providers: %v\n", err)
		}
		
		// For automatic matching, we'll select the first provider
		selectedProviderID = providers[0].ID
	}
	
	// Update order with provider
	updatedOrder, err := s.providerMatcher.AssignProvider(ctx, order, selectedProviderID)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "failed to assign provider: %v", err)
	}
	
	// Save to database
	err = s.repo.UpdateOrder(ctx, updatedOrder)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "failed to update order: %v", err)
	}
	
	// Record on blockchain asynchronously
	go func() {
		bCtx := context.Background()
		txHash, err := s.blockchainClient.RecordOrder(bCtx, updatedOrder.ID, updatedOrder.UserID, updatedOrder.ProviderID, updatedOrder)
		if err != nil {
			fmt.Printf("Failed to record provider assignment on blockchain: %v\n", err)
			return
		}

		// Update order with blockchain transaction hash
		updatedOrder.BlockchainTxHash = txHash
		if err := s.repo.UpdateOrder(bCtx, updatedOrder); err != nil {
			fmt.Printf("Failed to update order with blockchain hash: %v\n", err)
		}
	}()
	
	return &pb.OrderResponse{
		Order:   convertOrderToProto(updatedOrder),
		Message: "Provider assigned successfully",
		Success: true,
	}, nil
}

// AcceptOrder is called when a provider accepts an order
func (s *OrderService) AcceptOrder(ctx context.Context, req *pb.AcceptOrderRequest) (*pb.OrderResponse, error) {
	if req.OrderId == "" || req.ProviderId == "" {
		return nil, status.Errorf(codes.InvalidArgument, "order ID and provider ID are required")
	}
	
	// Get current order
	order, err := s.repo.GetOrderByID(ctx, req.OrderId)
	if err != nil {
		if errors.Is(err, repository.ErrOrderNotFound) {
			return nil, status.Errorf(codes.NotFound, "order not found")
		}
		return nil, status.Errorf(codes.Internal, "failed to get order: %v", err)
	}
	
	// Verify the provider is assigned to this order
	if order.ProviderID != req.ProviderId {
		return nil, status.Errorf(codes.PermissionDenied, "provider is not assigned to this order")
	}
	
	// Update order status
	order.AddStatusHistory(model.StatusProviderAccepted, req.ProviderId, "Provider accepted the order")
	order.UpdatedAt = time.Now()
	
	// Save to database
	err = s.repo.UpdateOrder(ctx, order)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "failed to update order: %v", err)
	}
	
	// Save initial provider location if provided
	if req.CurrentLocation != nil {
		orderLocation := &model.OrderLocation{
			OrderID:    order.ID,
			ProviderID: req.ProviderId,
			Latitude:   req.CurrentLocation.Latitude,
			Longitude:  req.CurrentLocation.Longitude,
			Timestamp:  time.Now(),
		}
		
		err = s.locationRepo.CreateOrderLocation(ctx, orderLocation)
		if err != nil {
			// Log but continue - this is not critical
			fmt.Printf("Failed to save initial provider location: %v\n", err)
		}
	}
	
	// Record on blockchain asynchronously
	go func() {
		bCtx := context.Background()
		txHash, err := s.blockchainClient.RecordOrder(bCtx, order.ID, order.UserID, order.ProviderID, order)
		if err != nil {
			fmt.Printf("Failed to record provider acceptance on blockchain: %v\n", err)
			return
		}

		// Update order with blockchain transaction hash
		order.BlockchainTxHash = txHash
		if err := s.repo.UpdateOrder(bCtx, order); err != nil {
			fmt.Printf("Failed to update order with blockchain hash: %v\n", err)
		}
	}()
	
	return &pb.OrderResponse{
		Order:   convertOrderToProto(order),
		Message: "Order accepted successfully",
		Success: true,
	}, nil
}

// RejectOrder is called when a provider rejects an order
func (s *OrderService) RejectOrder(ctx context.Context, req *pb.RejectOrderRequest) (*pb.OrderResponse, error) {
	if req.OrderId == "" || req.ProviderId == "" {
		return nil, status.Errorf(codes.InvalidArgument, "order ID and provider ID are required")
	}
	
	// Get current order
	order, err := s.repo.GetOrderByID(ctx, req.OrderId)
	if err != nil {
		if errors.Is(err, repository.ErrOrderNotFound) {
			return nil, status.Errorf(codes.NotFound, "order not found")
		}
		return nil, status.Errorf(codes.Internal, "failed to get order: %v", err)
	}
	
	// Verify the provider is assigned to this order
	if order.ProviderID != req.ProviderId {
		return nil, status.Errorf(codes.PermissionDenied, "provider is not assigned to this order")
	}
	
	// Update order status
	order.AddStatusHistory(model.StatusProviderRejected, req.ProviderId, req.Reason)
	order.ProviderID = "" // Clear provider ID to allow reassignment
	order.UpdatedAt = time.Now()
	
	// Save to database
	err = s.repo.UpdateOrder(ctx, order)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "failed to update order: %v", err)
	}
	
	// Record on blockchain asynchronously
	go func() {
		bCtx := context.Background()
		txHash, err := s.blockchainClient.RecordOrder(bCtx, order.ID, order.UserID, order.ProviderID, order)
		if err != nil {
			fmt.Printf("Failed to record provider rejection on blockchain: %v\n", err)
			return
		}

		// Update order with blockchain transaction hash
		order.BlockchainTxHash = txHash
		if err := s.repo.UpdateOrder(bCtx, order); err != nil {
			fmt.Printf("Failed to update order with blockchain hash: %v\n", err)
		}
	}()
	
	// Try to find another provider asynchronously
	go func() {
		bCtx := context.Background()
		providers, err := s.providerMatcher.FindBestProviders(bCtx, order, 3)
		if err != nil {
			fmt.Printf("Failed to find new providers: %v\n", err)
			return
		}
		
		if len(providers) > 0 {
			// Notify providers and select the first one
			s.providerMatcher.NotifyProviders(bCtx, order, providers)
			
			// Auto-assign to the first provider
			updatedOrder, err := s.providerMatcher.AssignProvider(bCtx, order, providers[0].ID)
			if err != nil {
				fmt.Printf("Failed to auto-assign new provider: %v\n", err)
				return
			}
			
			err = s.repo.UpdateOrder(bCtx, updatedOrder)
			if err != nil {
				fmt.Printf("Failed to update order with new provider: %v\n", err)
			}
		}
	}()
	
	return &pb.OrderResponse{
		Order:   convertOrderToProto(order),
		Message: "Order rejected successfully",
		Success: true,
	}, nil
}

// UpdateLocation updates the location of a provider for an order
func (s *OrderService) UpdateLocation(ctx context.Context, req *pb.UpdateLocationRequest) (*pb.UpdateLocationResponse, error) {
	if req.OrderId == "" || req.ProviderId == "" || req.Location == nil {
		return nil, status.Errorf(codes.InvalidArgument, "order ID, provider ID, and location are required")
	}
	
	// Get current order
	order, err := s.repo.GetOrderByID(ctx, req.OrderId)
	if err != nil {
		if errors.Is(err, repository.ErrOrderNotFound) {
			return nil, status.Errorf(codes.NotFound, "order not found")
		}
		return nil, status.Errorf(codes.Internal, "failed to get order: %v", err)
	}
	
	// Verify the provider is assigned to this order
	if order.ProviderID != req.ProviderId {
		return nil, status.Errorf(codes.PermissionDenied, "provider is not assigned to this order")
	}
	
	// Create new location entry
	orderLocation := &model.OrderLocation{
		OrderID:    req.OrderId,
		ProviderID: req.ProviderId,
		Latitude:   req.Location.Latitude,
		Longitude:  req.Location.Longitude,
		Timestamp:  time.Now(),
	}
	
	// Save to database
	err = s.locationRepo.CreateOrderLocation(ctx, orderLocation)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "failed to update location: %v", err)
	}
	
	// Calculate estimated arrival time
	var estimatedArrivalMinutes float32
	if order.Status == model.StatusInTransit || order.Status == model.StatusPickedUp {
		// Use destination location for ETA calculation
		estimatedArrivalMinutes = estimateArrivalMinutes(orderLocation, order.DestinationLocation)
	} else {
		// Use pickup location for ETA calculation
		estimatedArrivalMinutes = estimateArrivalMinutes(orderLocation, order.PickupLocation)
	}
	
	return &pb.UpdateLocationResponse{
		Success:                true,
		Message:                "Location updated successfully",
		EstimatedArrivalMinutes: estimatedArrivalMinutes,
	}, nil
} 