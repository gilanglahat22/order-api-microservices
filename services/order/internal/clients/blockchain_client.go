package clients

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	pb "github.com/order-api-microservices/proto/blockchain"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

// BlockchainGRPCClient is a client for the blockchain service
type BlockchainGRPCClient struct {
	client pb.BlockchainServiceClient
	conn   *grpc.ClientConn
}

// NewBlockchainGRPCClient creates a new blockchain service client
func NewBlockchainGRPCClient(address string) (*BlockchainGRPCClient, error) {
	conn, err := grpc.Dial(address, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		return nil, fmt.Errorf("failed to connect to blockchain service: %v", err)
	}

	client := pb.NewBlockchainServiceClient(conn)
	return &BlockchainGRPCClient{
		client: client,
		conn:   conn,
	}, nil
}

// Close closes the connection to the blockchain service
func (c *BlockchainGRPCClient) Close() error {
	if c.conn != nil {
		return c.conn.Close()
	}
	return nil
}

// RecordOrder records an order on the blockchain
func (c *BlockchainGRPCClient) RecordOrder(ctx context.Context, orderID, userID, providerID string, orderData interface{}) (string, error) {
	// Convert order data to JSON
	orderDataBytes, err := json.Marshal(orderData)
	if err != nil {
		return "", fmt.Errorf("failed to marshal order data: %v", err)
	}

	// Create a deterministic hash of the order data
	orderDataHash := []byte(fmt.Sprintf("%x", orderDataBytes))

	// Create the request
	req := &pb.RecordOrderRequest{
		OrderId:    orderID,
		UserId:     userID,
		ProviderId: providerID,
		OrderData: &pb.OrderData{
			Id:        orderID,
			UserId:    userID,
			ProviderId: providerID,
			DataHash:  orderDataHash,
		},
		Signature: "", // In a real implementation, this would be a digital signature
	}

	// Set context with timeout
	ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()

	// Call the service
	resp, err := c.client.RecordOrder(ctx, req)
	if err != nil {
		return "", fmt.Errorf("failed to record order on blockchain: %v", err)
	}

	if !resp.Success {
		return "", fmt.Errorf("blockchain service failed to record order: %s", resp.Message)
	}

	return resp.TransactionHash, nil
}

// VerifyOrder verifies an order on the blockchain
func (c *BlockchainGRPCClient) VerifyOrder(ctx context.Context, orderID, txHash string) (bool, error) {
	// Create the request
	req := &pb.VerifyOrderRequest{
		OrderId:         orderID,
		TransactionHash: txHash,
	}

	// Set context with timeout
	ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()

	// Call the service
	resp, err := c.client.VerifyOrder(ctx, req)
	if err != nil {
		return false, fmt.Errorf("failed to verify order on blockchain: %v", err)
	}

	return resp.Verified, nil
}

// GetOrderHistory gets the history of an order from the blockchain
func (c *BlockchainGRPCClient) GetOrderHistory(ctx context.Context, orderID string) ([]*pb.OrderHistoryItem, error) {
	// Create the request
	req := &pb.GetOrderHistoryRequest{
		OrderId: orderID,
	}

	// Set context with timeout
	ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()

	// Call the service
	resp, err := c.client.GetOrderHistory(ctx, req)
	if err != nil {
		return nil, fmt.Errorf("failed to get order history from blockchain: %v", err)
	}

	if !resp.Success {
		return nil, fmt.Errorf("blockchain service failed to get order history: %s", resp.Message)
	}

	return resp.History, nil
}

// GetTransactionDetails gets details about a transaction
func (c *BlockchainGRPCClient) GetTransactionDetails(ctx context.Context, txHash string) (*pb.GetTransactionDetailsResponse, error) {
	// Create the request
	req := &pb.GetTransactionDetailsRequest{
		TransactionHash: txHash,
	}

	// Set context with timeout
	ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()

	// Call the service
	resp, err := c.client.GetTransactionDetails(ctx, req)
	if err != nil {
		return nil, fmt.Errorf("failed to get transaction details from blockchain: %v", err)
	}

	if !resp.Success {
		return nil, fmt.Errorf("blockchain service failed to get transaction details: %s", resp.Message)
	}

	return resp, nil
} 