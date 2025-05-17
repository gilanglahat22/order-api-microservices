package gateway

import (
	"context"
	"encoding/json"
	"net/http"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	pb "github.com/order-api-microservices/proto/order"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// OrderHandler handles order API endpoints
type OrderHandler struct {
	orderClient pb.OrderServiceClient
}

// NewOrderHandler creates a new order handler
func NewOrderHandler(orderClient pb.OrderServiceClient) *OrderHandler {
	return &OrderHandler{
		orderClient: orderClient,
	}
}

// RegisterRoutes registers the order API routes
func (h *OrderHandler) RegisterRoutes(router *gin.Engine) {
	orders := router.Group("/api/v1/orders")
	{
		orders.POST("", h.CreateOrder)
		orders.GET("/:id", h.GetOrder)
		orders.PUT("/:id/status", h.UpdateOrderStatus)
		orders.POST("/:id/cancel", h.CancelOrder)
		orders.GET("/user/:id", h.ListUserOrders)
		orders.GET("/provider/:id", h.ListProviderOrders)
		orders.GET("/:id/track", h.TrackOrder) // WebSocket endpoint for tracking
		
		// New endpoints for provider assignment and tracking
		orders.POST("/:id/assign", h.AssignProvider)
		orders.POST("/:id/accept", h.AcceptOrder)
		orders.POST("/:id/reject", h.RejectOrder)
		orders.POST("/:id/location", h.UpdateLocation)
	}
}

// CreateOrder creates a new order
func (h *OrderHandler) CreateOrder(c *gin.Context) {
	var request struct {
		UserID             string                 `json:"user_id" binding:"required"`
		OrderType          string                 `json:"order_type" binding:"required"`
		PickupLocation     map[string]interface{} `json:"pickup_location" binding:"required"`
		DestinationLocation map[string]interface{} `json:"destination_location" binding:"required"`
		Items              []map[string]interface{} `json:"items"`
		PaymentMethod      string                 `json:"payment_method" binding:"required"`
		Notes              string                 `json:"notes"`
	}

	if err := c.ShouldBindJSON(&request); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// Convert request to protobuf
	req := &pb.CreateOrderRequest{
		UserId:             request.UserID,
		OrderType:          convertOrderTypeFromString(request.OrderType),
		PickupLocation:     convertLocationFromMap(request.PickupLocation),
		DestinationLocation: convertLocationFromMap(request.DestinationLocation),
		Items:              convertOrderItemsFromSlice(request.Items),
		PaymentMethod:      convertPaymentMethodFromString(request.PaymentMethod),
		Notes:              request.Notes,
	}

	// Call the order service
	ctx, cancel := context.WithTimeout(c.Request.Context(), 10*time.Second)
	defer cancel()

	resp, err := h.orderClient.CreateOrder(ctx, req)
	if err != nil {
		st, ok := status.FromError(err)
		if ok {
			switch st.Code() {
			case codes.InvalidArgument:
				c.JSON(http.StatusBadRequest, gin.H{"error": st.Message()})
				return
			default:
				c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to create order"})
				return
			}
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusCreated, resp.Order)
}

// GetOrder gets an order by ID
func (h *OrderHandler) GetOrder(c *gin.Context) {
	orderID := c.Param("id")
	if orderID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "order ID is required"})
		return
	}

	// Call the order service
	ctx, cancel := context.WithTimeout(c.Request.Context(), 10*time.Second)
	defer cancel()

	resp, err := h.orderClient.GetOrder(ctx, &pb.GetOrderRequest{OrderId: orderID})
	if err != nil {
		st, ok := status.FromError(err)
		if ok {
			switch st.Code() {
			case codes.NotFound:
				c.JSON(http.StatusNotFound, gin.H{"error": "Order not found"})
				return
			default:
				c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to get order"})
				return
			}
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, resp.Order)
}

// UpdateOrderStatus updates the status of an order
func (h *OrderHandler) UpdateOrderStatus(c *gin.Context) {
	orderID := c.Param("id")
	if orderID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "order ID is required"})
		return
	}

	var request struct {
		Status    string `json:"status" binding:"required"`
		UpdatedBy string `json:"updated_by" binding:"required"`
		Notes     string `json:"notes"`
	}

	if err := c.ShouldBindJSON(&request); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// Convert request to protobuf
	req := &pb.UpdateOrderStatusRequest{
		OrderId:   orderID,
		Status:    convertOrderStatusFromString(request.Status),
		UpdatedBy: request.UpdatedBy,
		Notes:     request.Notes,
	}

	// Call the order service
	ctx, cancel := context.WithTimeout(c.Request.Context(), 10*time.Second)
	defer cancel()

	resp, err := h.orderClient.UpdateOrderStatus(ctx, req)
	if err != nil {
		st, ok := status.FromError(err)
		if ok {
			switch st.Code() {
			case codes.NotFound:
				c.JSON(http.StatusNotFound, gin.H{"error": "Order not found"})
				return
			case codes.InvalidArgument:
				c.JSON(http.StatusBadRequest, gin.H{"error": st.Message()})
				return
			default:
				c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to update order status"})
				return
			}
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, resp.Order)
}

// CancelOrder cancels an order
func (h *OrderHandler) CancelOrder(c *gin.Context) {
	orderID := c.Param("id")
	if orderID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "order ID is required"})
		return
	}

	var request struct {
		CancelledBy string `json:"cancelled_by" binding:"required"`
		Reason      string `json:"reason" binding:"required"`
	}

	if err := c.ShouldBindJSON(&request); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// Convert request to protobuf
	req := &pb.CancelOrderRequest{
		OrderId:     orderID,
		CancelledBy: request.CancelledBy,
		Reason:      request.Reason,
	}

	// Call the order service
	ctx, cancel := context.WithTimeout(c.Request.Context(), 10*time.Second)
	defer cancel()

	resp, err := h.orderClient.CancelOrder(ctx, req)
	if err != nil {
		st, ok := status.FromError(err)
		if ok {
			switch st.Code() {
			case codes.NotFound:
				c.JSON(http.StatusNotFound, gin.H{"error": "Order not found"})
				return
			case codes.FailedPrecondition:
				c.JSON(http.StatusBadRequest, gin.H{"error": st.Message()})
				return
			default:
				c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to cancel order"})
				return
			}
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, resp.Order)
}

// ListUserOrders lists orders for a specific user
func (h *OrderHandler) ListUserOrders(c *gin.Context) {
	userID := c.Param("id")
	if userID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "user ID is required"})
		return
	}

	// Get query parameters
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "10"))
	status := c.DefaultQuery("status", "")

	// Convert request to protobuf
	req := &pb.ListUserOrdersRequest{
		UserId: userID,
		Page:   int32(page),
		Limit:  int32(limit),
		Status: convertOrderStatusFromString(status),
	}

	// Call the order service
	ctx, cancel := context.WithTimeout(c.Request.Context(), 10*time.Second)
	defer cancel()

	resp, err := h.orderClient.ListUserOrders(ctx, req)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"orders": resp.Orders,
		"total":  resp.Total,
		"page":   resp.Page,
		"limit":  resp.Limit,
	})
}

// ListProviderOrders lists orders for a specific provider
func (h *OrderHandler) ListProviderOrders(c *gin.Context) {
	providerID := c.Param("id")
	if providerID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "provider ID is required"})
		return
	}

	// Get query parameters
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "10"))
	status := c.DefaultQuery("status", "")

	// Convert request to protobuf
	req := &pb.ListProviderOrdersRequest{
		ProviderId: providerID,
		Page:       int32(page),
		Limit:      int32(limit),
		Status:     convertOrderStatusFromString(status),
	}

	// Call the order service
	ctx, cancel := context.WithTimeout(c.Request.Context(), 10*time.Second)
	defer cancel()

	resp, err := h.orderClient.ListProviderOrders(ctx, req)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"orders": resp.Orders,
		"total":  resp.Total,
		"page":   resp.Page,
		"limit":  resp.Limit,
	})
}

// TrackOrder streams location updates for an order using Server-Sent Events
func (h *OrderHandler) TrackOrder(c *gin.Context) {
	orderID := c.Param("id")
	if orderID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "order ID is required"})
		return
	}

	// Set up SSE
	c.Header("Content-Type", "text/event-stream")
	c.Header("Cache-Control", "no-cache")
	c.Header("Connection", "keep-alive")
	c.Header("Transfer-Encoding", "chunked")

	// Create a channel for client disconnect
	clientGone := c.Writer.CloseNotify()

	// Call the order service
	ctx, cancel := context.WithCancel(c.Request.Context())
	defer cancel()

	stream, err := h.orderClient.TrackOrder(ctx, &pb.TrackOrderRequest{OrderId: orderID})
	if err != nil {
		c.Status(http.StatusInternalServerError)
		return
	}

	// Stream location updates
	for {
		select {
		case <-clientGone:
			return
		default:
			update, err := stream.Recv()
			if err != nil {
				return
			}

			// Convert to JSON
			data, err := json.Marshal(update)
			if err != nil {
				continue
			}

			// Send SSE message
			c.SSEvent("location", string(data))
			c.Writer.Flush()
		}
	}
}

// AssignProvider assigns a provider to an order
func (h *OrderHandler) AssignProvider(c *gin.Context) {
	orderID := c.Param("id")
	if orderID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "order ID is required"})
		return
	}

	var request struct {
		ProviderID string `json:"provider_id"` // Optional for manual assignment
	}

	if err := c.ShouldBindJSON(&request); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// Convert request to protobuf
	req := &pb.AssignProviderRequest{
		OrderId:   orderID,
		ProviderId: request.ProviderID,
	}

	// Call the order service
	ctx, cancel := context.WithTimeout(c.Request.Context(), 10*time.Second)
	defer cancel()

	resp, err := h.orderClient.AssignProvider(ctx, req)
	if err != nil {
		st, ok := status.FromError(err)
		if ok {
			switch st.Code() {
			case codes.NotFound:
				c.JSON(http.StatusNotFound, gin.H{"error": st.Message()})
				return
			case codes.InvalidArgument:
				c.JSON(http.StatusBadRequest, gin.H{"error": st.Message()})
				return
			default:
				c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to assign provider"})
				return
			}
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, resp.Order)
}

// AcceptOrder handles a provider accepting an order
func (h *OrderHandler) AcceptOrder(c *gin.Context) {
	orderID := c.Param("id")
	if orderID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "order ID is required"})
		return
	}

	var request struct {
		ProviderID      string                 `json:"provider_id" binding:"required"`
		CurrentLocation map[string]interface{} `json:"current_location"`
	}

	if err := c.ShouldBindJSON(&request); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// Convert request to protobuf
	req := &pb.AcceptOrderRequest{
		OrderId:   orderID,
		ProviderId: request.ProviderID,
	}

	// Add location if provided
	if request.CurrentLocation != nil {
		req.CurrentLocation = convertLocationFromMap(request.CurrentLocation)
	}

	// Call the order service
	ctx, cancel := context.WithTimeout(c.Request.Context(), 10*time.Second)
	defer cancel()

	resp, err := h.orderClient.AcceptOrder(ctx, req)
	if err != nil {
		st, ok := status.FromError(err)
		if ok {
			switch st.Code() {
			case codes.NotFound:
				c.JSON(http.StatusNotFound, gin.H{"error": st.Message()})
				return
			case codes.PermissionDenied:
				c.JSON(http.StatusForbidden, gin.H{"error": st.Message()})
				return
			case codes.InvalidArgument:
				c.JSON(http.StatusBadRequest, gin.H{"error": st.Message()})
				return
			default:
				c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to accept order"})
				return
			}
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, resp.Order)
}

// RejectOrder handles a provider rejecting an order
func (h *OrderHandler) RejectOrder(c *gin.Context) {
	orderID := c.Param("id")
	if orderID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "order ID is required"})
		return
	}

	var request struct {
		ProviderID string `json:"provider_id" binding:"required"`
		Reason     string `json:"reason" binding:"required"`
	}

	if err := c.ShouldBindJSON(&request); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// Convert request to protobuf
	req := &pb.RejectOrderRequest{
		OrderId:   orderID,
		ProviderId: request.ProviderID,
		Reason:    request.Reason,
	}

	// Call the order service
	ctx, cancel := context.WithTimeout(c.Request.Context(), 10*time.Second)
	defer cancel()

	resp, err := h.orderClient.RejectOrder(ctx, req)
	if err != nil {
		st, ok := status.FromError(err)
		if ok {
			switch st.Code() {
			case codes.NotFound:
				c.JSON(http.StatusNotFound, gin.H{"error": st.Message()})
				return
			case codes.PermissionDenied:
				c.JSON(http.StatusForbidden, gin.H{"error": st.Message()})
				return
			case codes.InvalidArgument:
				c.JSON(http.StatusBadRequest, gin.H{"error": st.Message()})
				return
			default:
				c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to reject order"})
				return
			}
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, resp.Order)
}

// UpdateLocation updates the provider's location for an order
func (h *OrderHandler) UpdateLocation(c *gin.Context) {
	orderID := c.Param("id")
	if orderID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "order ID is required"})
		return
	}

	var request struct {
		ProviderID string                 `json:"provider_id" binding:"required"`
		Location   map[string]interface{} `json:"location" binding:"required"`
	}

	if err := c.ShouldBindJSON(&request); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// Convert request to protobuf
	req := &pb.UpdateLocationRequest{
		OrderId:   orderID,
		ProviderId: request.ProviderID,
		Location:  convertLocationFromMap(request.Location),
	}

	// Call the order service
	ctx, cancel := context.WithTimeout(c.Request.Context(), 10*time.Second)
	defer cancel()

	resp, err := h.orderClient.UpdateLocation(ctx, req)
	if err != nil {
		st, ok := status.FromError(err)
		if ok {
			switch st.Code() {
			case codes.NotFound:
				c.JSON(http.StatusNotFound, gin.H{"error": st.Message()})
				return
			case codes.PermissionDenied:
				c.JSON(http.StatusForbidden, gin.H{"error": st.Message()})
				return
			case codes.InvalidArgument:
				c.JSON(http.StatusBadRequest, gin.H{"error": st.Message()})
				return
			default:
				c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to update location"})
				return
			}
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": resp.Success,
		"message": resp.Message,
		"estimated_arrival_minutes": resp.EstimatedArrivalMinutes,
	})
}

// Helper functions

func convertOrderTypeFromString(orderType string) pb.OrderType {
	switch orderType {
	case "RIDE":
		return pb.OrderType_ORDER_TYPE_RIDE
	case "FOOD_DELIVERY":
		return pb.OrderType_ORDER_TYPE_FOOD_DELIVERY
	case "PACKAGE_DELIVERY":
		return pb.OrderType_ORDER_TYPE_PACKAGE_DELIVERY
	case "GROCERY_DELIVERY":
		return pb.OrderType_ORDER_TYPE_GROCERY_DELIVERY
	case "SERVICE_BOOKING":
		return pb.OrderType_ORDER_TYPE_SERVICE_BOOKING
	default:
		return pb.OrderType_ORDER_TYPE_UNSPECIFIED
	}
}

func convertOrderStatusFromString(status string) pb.OrderStatus {
	switch status {
	case "CREATED":
		return pb.OrderStatus_ORDER_STATUS_CREATED
	case "PAYMENT_PENDING":
		return pb.OrderStatus_ORDER_STATUS_PAYMENT_PENDING
	case "PAYMENT_COMPLETED":
		return pb.OrderStatus_ORDER_STATUS_PAYMENT_COMPLETED
	case "PROVIDER_ASSIGNED":
		return pb.OrderStatus_ORDER_STATUS_PROVIDER_ASSIGNED
	case "PROVIDER_ACCEPTED":
		return pb.OrderStatus_ORDER_STATUS_PROVIDER_ACCEPTED
	case "PROVIDER_REJECTED":
		return pb.OrderStatus_ORDER_STATUS_PROVIDER_REJECTED
	case "IN_PROGRESS":
		return pb.OrderStatus_ORDER_STATUS_IN_PROGRESS
	case "PICKED_UP":
		return pb.OrderStatus_ORDER_STATUS_PICKED_UP
	case "IN_TRANSIT":
		return pb.OrderStatus_ORDER_STATUS_IN_TRANSIT
	case "ARRIVED":
		return pb.OrderStatus_ORDER_STATUS_ARRIVED
	case "DELIVERED":
		return pb.OrderStatus_ORDER_STATUS_DELIVERED
	case "COMPLETED":
		return pb.OrderStatus_ORDER_STATUS_COMPLETED
	case "CANCELLED":
		return pb.OrderStatus_ORDER_STATUS_CANCELLED
	case "REFUNDED":
		return pb.OrderStatus_ORDER_STATUS_REFUNDED
	case "DISPUTED":
		return pb.OrderStatus_ORDER_STATUS_DISPUTED
	default:
		return pb.OrderStatus_ORDER_STATUS_UNSPECIFIED
	}
}

func convertPaymentMethodFromString(method string) pb.PaymentMethod {
	switch method {
	case "CREDIT_CARD":
		return pb.PaymentMethod_PAYMENT_METHOD_CREDIT_CARD
	case "DEBIT_CARD":
		return pb.PaymentMethod_PAYMENT_METHOD_DEBIT_CARD
	case "DIGITAL_WALLET":
		return pb.PaymentMethod_PAYMENT_METHOD_DIGITAL_WALLET
	case "CASH":
		return pb.PaymentMethod_PAYMENT_METHOD_CASH
	case "CRYPTO":
		return pb.PaymentMethod_PAYMENT_METHOD_CRYPTO
	default:
		return pb.PaymentMethod_PAYMENT_METHOD_UNSPECIFIED
	}
}

func convertLocationFromMap(location map[string]interface{}) *pb.Location {
	loc := &pb.Location{
		AdditionalInfo: make(map[string]string),
	}

	// Extract fields from map
	if lat, ok := location["latitude"].(float64); ok {
		loc.Latitude = lat
	}
	if lng, ok := location["longitude"].(float64); ok {
		loc.Longitude = lng
	}
	if addr, ok := location["address"].(string); ok {
		loc.Address = addr
	}
	if postal, ok := location["postal_code"].(string); ok {
		loc.PostalCode = postal
	}
	if city, ok := location["city"].(string); ok {
		loc.City = city
	}
	if country, ok := location["country"].(string); ok {
		loc.Country = country
	}

	// Process additional info if present
	if addInfo, ok := location["additional_info"].(map[string]interface{}); ok {
		for k, v := range addInfo {
			if strValue, ok := v.(string); ok {
				loc.AdditionalInfo[k] = strValue
			}
		}
	}

	return loc
}

func convertOrderItemsFromSlice(items []map[string]interface{}) []*pb.OrderItem {
	result := []*pb.OrderItem{}

	for _, item := range items {
		orderItem := &pb.OrderItem{
			Properties: make(map[string]string),
		}

		// Extract id and name
		if id, ok := item["item_id"].(string); ok {
			orderItem.ItemId = id
		} else {
			// Generate random ID if not provided
			orderItem.ItemId = uuid.New().String()
		}

		if name, ok := item["name"].(string); ok {
			orderItem.Name = name
		}

		// Extract quantity
		if qty, ok := item["quantity"].(float64); ok {
			orderItem.Quantity = int32(qty)
		} else {
			orderItem.Quantity = 1
		}

		// Extract price
		if price, ok := item["price"].(float64); ok {
			orderItem.Price = float32(price)
		}

		// Process properties if present
		if props, ok := item["properties"].(map[string]interface{}); ok {
			for k, v := range props {
				if strValue, ok := v.(string); ok {
					orderItem.Properties[k] = strValue
				}
			}
		}

		result = append(result, orderItem)
	}

	return result
} 