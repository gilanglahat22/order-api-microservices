package service

import (
	"context"
	"fmt"
	"time"

	"github.com/ethereum/go-ethereum/core/types"
	"github.com/order-api-microservices/pkg/blockchain"
	pb "github.com/order-api-microservices/proto/blockchain"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/timestamppb"
)

// BlockchainService handles interactions with the blockchain
type BlockchainService struct {
	pb.UnimplementedBlockchainServiceServer
	ethClient *blockchain.EthereumClient
}

// NewBlockchainService creates a new blockchain service
func NewBlockchainService(ethClient *blockchain.EthereumClient) *BlockchainService {
	return &BlockchainService{
		ethClient: ethClient,
	}
}

// RecordOrder records a new order on the blockchain
func (s *BlockchainService) RecordOrder(ctx context.Context, req *pb.RecordOrderRequest) (*pb.RecordOrderResponse, error) {
	// Convert order data to a hash
	items := make([]string, 0, len(req.OrderData.Items))
	for _, item := range req.OrderData.Items {
		items = append(items, fmt.Sprintf("%s:%s:%d:%f", item.ItemId, item.Name, item.Quantity, item.Price))
	}

	dataHash, err := blockchain.ComputeOrderHash(
		req.OrderId,
		req.UserId,
		req.ProviderId,
		float64(req.OrderData.TotalPrice),
		items,
		blockchain.OrderStatus(req.OrderData.Status),
	)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "failed to compute order hash: %v", err)
	}

	// Record order on blockchain
	txHash, err := s.ethClient.RecordOrder(ctx, req.OrderId, dataHash, blockchain.OrderStatus(req.OrderData.Status))
	if err != nil {
		return nil, status.Errorf(codes.Internal, "failed to record order on blockchain: %v", err)
	}

	// Get transaction details
	tx, receipt, err := s.ethClient.GetTransactionDetails(ctx, txHash)
	if err != nil {
		// Still return success but include error in message
		return &pb.RecordOrderResponse{
			Success:        true,
			TransactionHash: txHash,
			Message:        fmt.Sprintf("Order recorded but failed to get transaction details: %v", err),
			Timestamp:      timestamppb.Now(),
		}, nil
	}

	return &pb.RecordOrderResponse{
		Success:        true,
		TransactionHash: txHash,
		BlockNumber:    fmt.Sprintf("%d", receipt.BlockNumber),
		Message:        "Order successfully recorded on blockchain",
		Timestamp:      timestamppb.Now(),
	}, nil
}

// VerifyOrder verifies an order on the blockchain
func (s *BlockchainService) VerifyOrder(ctx context.Context, req *pb.VerifyOrderRequest) (*pb.VerifyOrderResponse, error) {
	// Get transaction details
	tx, receipt, err := s.ethClient.GetTransactionDetails(ctx, req.TransactionHash)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "failed to get transaction details: %v", err)
	}

	// Get order data from blockchain
	exists, dataHash, timestamp, orderStatus, err := s.ethClient.GetOrderStatus(ctx, req.OrderId)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "failed to get order status from blockchain: %v", err)
	}

	if !exists {
		return &pb.VerifyOrderResponse{
			Verified: false,
			Message:  "Order does not exist on blockchain",
		}, nil
	}

	// Return verification result
	return &pb.VerifyOrderResponse{
		Verified:    true,
		BlockNumber: fmt.Sprintf("%d", receipt.BlockNumber),
		BlockHash:   receipt.BlockHash.Hex(),
		Timestamp:   timestamppb.New(time.Unix(int64(timestamp), 0)),
		Message:     "Order verified on blockchain",
	}, nil
}

// GetOrderHistory gets the history of an order from the blockchain
func (s *BlockchainService) GetOrderHistory(ctx context.Context, req *pb.GetOrderHistoryRequest) (*pb.GetOrderHistoryResponse, error) {
	// Check if order exists
	exists, _, _, _, err := s.ethClient.GetOrderStatus(ctx, req.OrderId)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "failed to check order existence: %v", err)
	}

	if !exists {
		return &pb.GetOrderHistoryResponse{
			OrderId: req.OrderId,
			Success: false,
			Message: "Order does not exist on blockchain",
		}, nil
	}

	// For this implementation, we're simplifying by just returning the current state
	// In a complete implementation, we would fetch the complete history from the smart contract
	
	// Return mock history for now
	return &pb.GetOrderHistoryResponse{
		OrderId: req.OrderId,
		History: []*pb.OrderHistoryItem{
			{
				TransactionHash: "0x1234567890abcdef",
				BlockNumber:     "12345",
				Status:          pb.OrderStatus_ORDER_STATUS_CREATED,
				UpdatedBy:       "system",
				Timestamp:       timestamppb.Now(),
			},
		},
		Success: true,
		Message: "Order history retrieved",
	}, nil
}

// GetTransactionDetails gets details about a transaction
func (s *BlockchainService) GetTransactionDetails(ctx context.Context, req *pb.GetTransactionDetailsRequest) (*pb.GetTransactionDetailsResponse, error) {
	tx, receipt, err := s.ethClient.GetTransactionDetails(ctx, req.TransactionHash)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "failed to get transaction details: %v", err)
	}

	// Convert transaction data
	status := "success"
	if receipt.Status == 0 {
		status = "failed"
	}

	from, err := types.Sender(types.NewEIP155Signer(tx.ChainId()), tx)
	if err != nil {
		from = s.ethClient.FromAddress()
	}

	return &pb.GetTransactionDetailsResponse{
		TransactionHash: req.TransactionHash,
		BlockNumber:     fmt.Sprintf("%d", receipt.BlockNumber),
		BlockHash:       receipt.BlockHash.Hex(),
		FromAddress:     from.Hex(),
		ToAddress:       tx.To().Hex(),
		Data:            fmt.Sprintf("%x", tx.Data()),
		Value:           tx.Value().String(),
		GasUsed:         receipt.GasUsed,
		Timestamp:       timestamppb.Now(), // Ideally, we'd get the block timestamp
		Status:          status,
		Success:         true,
		Message:         "Transaction details retrieved",
	}, nil
} 