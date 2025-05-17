package blockchain

import (
	"context"
	"crypto/ecdsa"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"math/big"
	"strings"
	"time"

	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/accounts/abi/bind"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/ethereum/go-ethereum/ethclient"
)

// OrderStatus enum (matching the Solidity enum)
type OrderStatus int

const (
	OrderStatusUnspecified OrderStatus = iota
	OrderStatusCreated
	OrderStatusPaymentPending
	OrderStatusPaymentCompleted
	OrderStatusProviderAssigned
	OrderStatusProviderAccepted
	OrderStatusProviderRejected
	OrderStatusInProgress
	OrderStatusPickedUp
	OrderStatusInTransit
	OrderStatusArrived
	OrderStatusDelivered
	OrderStatusCompleted
	OrderStatusCancelled
	OrderStatusRefunded
	OrderStatusDisputed
)

// EthereumClient handles interactions with the Ethereum blockchain
type EthereumClient struct {
	client        *ethclient.Client
	contractAddr  common.Address
	contractABI   abi.ABI
	privateKey    *ecdsa.PrivateKey
	fromAddress   common.Address
	gasPrice      *big.Int
	gasLimit      uint64
	retryAttempts int
	retryDelay    time.Duration
}

// NewEthereumClient creates a new Ethereum client
func NewEthereumClient(rpcURL, contractAddress, privateKeyHex string) (*EthereumClient, error) {
	client, err := ethclient.Dial(rpcURL)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to Ethereum client: %v", err)
	}

	// Parse contract ABI
	parsedABI, err := abi.JSON(strings.NewReader(orderRegistryABI))
	if err != nil {
		return nil, fmt.Errorf("failed to parse contract ABI: %v", err)
	}

	// Parse private key
	privateKey, err := crypto.HexToECDSA(privateKeyHex)
	if err != nil {
		return nil, fmt.Errorf("failed to parse private key: %v", err)
	}

	// Derive sender address from private key
	publicKey := privateKey.Public()
	publicKeyECDSA, ok := publicKey.(*ecdsa.PublicKey)
	if !ok {
		return nil, fmt.Errorf("error casting public key to ECDSA")
	}
	fromAddress := crypto.PubkeyToAddress(*publicKeyECDSA)

	return &EthereumClient{
		client:        client,
		contractAddr:  common.HexToAddress(contractAddress),
		contractABI:   parsedABI,
		privateKey:    privateKey,
		fromAddress:   fromAddress,
		gasPrice:      big.NewInt(20000000000), // 20 Gwei
		gasLimit:      uint64(300000),
		retryAttempts: 3,
		retryDelay:    time.Second * 2,
	}, nil
}

// FromAddress returns the address derived from the private key
func (c *EthereumClient) FromAddress() common.Address {
	return c.fromAddress
}

// ComputeOrderHash computes a hash of the order data
func ComputeOrderHash(orderID, userID, providerID string, totalPrice float64, items []string, status OrderStatus) ([32]byte, error) {
	// Create a string representation of the order
	orderStr := fmt.Sprintf("%s:%s:%s:%f:%s:%d", orderID, userID, providerID, totalPrice, strings.Join(items, ","), status)
	
	// Compute SHA-256 hash
	hash := sha256.Sum256([]byte(orderStr))
	return hash, nil
}

// RecordOrder records a new order on the blockchain
func (c *EthereumClient) RecordOrder(ctx context.Context, orderID string, dataHash [32]byte, status OrderStatus) (string, error) {
	auth, err := c.getTransactOpts(ctx)
	if err != nil {
		return "", err
	}

	// Pack the transaction data
	data, err := c.contractABI.Pack("recordOrder", orderID, dataHash, uint8(status))
	if err != nil {
		return "", fmt.Errorf("failed to pack transaction data: %v", err)
	}

	// Create transaction
	tx := types.NewTransaction(
		auth.Nonce.Uint64(),
		c.contractAddr,
		big.NewInt(0),
		c.gasLimit,
		auth.GasPrice,
		data,
	)

	// Sign transaction
	signedTx, err := types.SignTx(tx, types.NewEIP155Signer(auth.ChainID), c.privateKey)
	if err != nil {
		return "", fmt.Errorf("failed to sign transaction: %v", err)
	}

	// Send transaction
	err = c.client.SendTransaction(ctx, signedTx)
	if err != nil {
		return "", fmt.Errorf("failed to send transaction: %v", err)
	}

	// Wait for transaction to be mined
	receipt, err := bind.WaitMined(ctx, c.client, signedTx)
	if err != nil {
		return "", fmt.Errorf("failed waiting for transaction to be mined: %v", err)
	}

	if receipt.Status == 0 {
		return "", fmt.Errorf("transaction failed")
	}

	return signedTx.Hash().Hex(), nil
}

// UpdateOrderStatus updates an existing order's status on the blockchain
func (c *EthereumClient) UpdateOrderStatus(ctx context.Context, orderID string, dataHash [32]byte, status OrderStatus) (string, error) {
	auth, err := c.getTransactOpts(ctx)
	if err != nil {
		return "", err
	}

	// Pack the transaction data
	data, err := c.contractABI.Pack("updateOrderStatus", orderID, dataHash, uint8(status))
	if err != nil {
		return "", fmt.Errorf("failed to pack transaction data: %v", err)
	}

	// Create transaction
	tx := types.NewTransaction(
		auth.Nonce.Uint64(),
		c.contractAddr,
		big.NewInt(0),
		c.gasLimit,
		auth.GasPrice,
		data,
	)

	// Sign transaction
	signedTx, err := types.SignTx(tx, types.NewEIP155Signer(auth.ChainID), c.privateKey)
	if err != nil {
		return "", fmt.Errorf("failed to sign transaction: %v", err)
	}

	// Send transaction
	err = c.client.SendTransaction(ctx, signedTx)
	if err != nil {
		return "", fmt.Errorf("failed to send transaction: %v", err)
	}

	// Wait for transaction to be mined
	receipt, err := bind.WaitMined(ctx, c.client, signedTx)
	if err != nil {
		return "", fmt.Errorf("failed waiting for transaction to be mined: %v", err)
	}

	if receipt.Status == 0 {
		return "", fmt.Errorf("transaction failed")
	}

	return signedTx.Hash().Hex(), nil
}

// VerifyOrderHash verifies if the given hash matches the on-chain hash for the order
func (c *EthereumClient) VerifyOrderHash(ctx context.Context, orderID string, dataHash [32]byte) (bool, error) {
	// Pack the call data
	data, err := c.contractABI.Pack("verifyOrderHash", orderID, dataHash)
	if err != nil {
		return false, fmt.Errorf("failed to pack call data: %v", err)
	}

	// Make the call
	msg := common.CallMsg{
		To:   &c.contractAddr,
		Data: data,
	}
	result, err := c.client.CallContract(ctx, msg, nil)
	if err != nil {
		return false, fmt.Errorf("contract call failed: %v", err)
	}

	// Unpack result
	var verified bool
	err = c.contractABI.UnpackIntoInterface(&verified, "verifyOrderHash", result)
	if err != nil {
		return false, fmt.Errorf("failed to unpack result: %v", err)
	}

	return verified, nil
}

// GetOrderStatus retrieves the current status of an order from the blockchain
func (c *EthereumClient) GetOrderStatus(ctx context.Context, orderID string) (bool, [32]byte, uint64, OrderStatus, error) {
	// Pack the call data
	data, err := c.contractABI.Pack("getOrderStatus", orderID)
	if err != nil {
		return false, [32]byte{}, 0, OrderStatusUnspecified, fmt.Errorf("failed to pack call data: %v", err)
	}

	// Make the call
	msg := common.CallMsg{
		To:   &c.contractAddr,
		Data: data,
	}
	result, err := c.client.CallContract(ctx, msg, nil)
	if err != nil {
		return false, [32]byte{}, 0, OrderStatusUnspecified, fmt.Errorf("contract call failed: %v", err)
	}

	// Unpack result
	var unpacked struct {
		Exists    bool
		DataHash  [32]byte
		Timestamp *big.Int
		Status    uint8
	}
	err = c.contractABI.UnpackIntoInterface(&unpacked, "getOrderStatus", result)
	if err != nil {
		return false, [32]byte{}, 0, OrderStatusUnspecified, fmt.Errorf("failed to unpack result: %v", err)
	}

	return unpacked.Exists, unpacked.DataHash, unpacked.Timestamp.Uint64(), OrderStatus(unpacked.Status), nil
}

// GetTransactionDetails retrieves details about a specific transaction
func (c *EthereumClient) GetTransactionDetails(ctx context.Context, txHash string) (*types.Transaction, *types.Receipt, error) {
	hash := common.HexToHash(txHash)

	// Get transaction
	tx, isPending, err := c.client.TransactionByHash(ctx, hash)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to get transaction: %v", err)
	}

	if isPending {
		return tx, nil, fmt.Errorf("transaction is still pending")
	}

	// Get transaction receipt
	receipt, err := c.client.TransactionReceipt(ctx, hash)
	if err != nil {
		return tx, nil, fmt.Errorf("failed to get transaction receipt: %v", err)
	}

	return tx, receipt, nil
}

// getTransactOpts prepares transaction options for sending transactions
func (c *EthereumClient) getTransactOpts(ctx context.Context) (*bind.TransactOpts, error) {
	nonce, err := c.client.PendingNonceAt(ctx, c.fromAddress)
	if err != nil {
		return nil, fmt.Errorf("failed to get nonce: %v", err)
	}

	gasPrice, err := c.client.SuggestGasPrice(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to suggest gas price: %v", err)
	}

	chainID, err := c.client.ChainID(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to get chain ID: %v", err)
	}

	auth, err := bind.NewKeyedTransactorWithChainID(c.privateKey, chainID)
	if err != nil {
		return nil, err
	}
	auth.Nonce = big.NewInt(int64(nonce))
	auth.Value = big.NewInt(0)
	auth.GasLimit = c.gasLimit
	auth.GasPrice = gasPrice

	return auth, nil
}

// ABI for the OrderRegistry contract
const orderRegistryABI = `[{"inputs":[],"stateMutability":"nonpayable","type":"constructor"},{"anonymous":false,"inputs":[{"indexed":true,"internalType":"string","name":"orderId","type":"string"},{"indexed":false,"internalType":"bytes32","name":"dataHash","type":"bytes32"},{"indexed":false,"internalType":"uint256","name":"timestamp","type":"uint256"},{"indexed":false,"internalType":"enum OrderRegistry.OrderStatus","name":"status","type":"uint8"}],"name":"OrderRecorded","type":"event"},{"anonymous":false,"inputs":[{"indexed":true,"internalType":"string","name":"orderId","type":"string"},{"indexed":false,"internalType":"bytes32","name":"dataHash","type":"bytes32"},{"indexed":false,"internalType":"uint256","name":"timestamp","type":"uint256"},{"indexed":false,"internalType":"enum OrderRegistry.OrderStatus","name":"status","type":"uint8"}],"name":"OrderUpdated","type":"event"},{"inputs":[{"internalType":"string","name":"orderId","type":"string"}],"name":"getOrderHistoryCount","outputs":[{"internalType":"uint256","name":"","type":"uint256"}],"stateMutability":"view","type":"function"},{"inputs":[{"internalType":"string","name":"orderId","type":"string"},{"internalType":"uint256","name":"index","type":"uint256"}],"name":"getOrderHistoryEntry","outputs":[{"internalType":"bytes32","name":"dataHash","type":"bytes32"},{"internalType":"uint256","name":"timestamp","type":"uint256"},{"internalType":"enum OrderRegistry.OrderStatus","name":"status","type":"uint8"},{"internalType":"address","name":"updatedBy","type":"address"}],"stateMutability":"view","type":"function"},{"inputs":[{"internalType":"string","name":"orderId","type":"string"}],"name":"getOrderStatus","outputs":[{"internalType":"bool","name":"exists","type":"bool"},{"internalType":"bytes32","name":"dataHash","type":"bytes32"},{"internalType":"uint256","name":"timestamp","type":"uint256"},{"internalType":"enum OrderRegistry.OrderStatus","name":"status","type":"uint8"}],"stateMutability":"view","type":"function"},{"inputs":[{"internalType":"string","name":"","type":"string"}],"name":"orderHistory","outputs":[{"internalType":"bytes32","name":"dataHash","type":"bytes32"},{"internalType":"uint256","name":"timestamp","type":"uint256"},{"internalType":"enum OrderRegistry.OrderStatus","name":"status","type":"uint8"},{"internalType":"address","name":"updatedBy","type":"address"},{"internalType":"bool","name":"exists","type":"bool"}],"stateMutability":"view","type":"function"},{"inputs":[{"internalType":"string","name":"","type":"string"}],"name":"orders","outputs":[{"internalType":"bytes32","name":"dataHash","type":"bytes32"},{"internalType":"uint256","name":"timestamp","type":"uint256"},{"internalType":"enum OrderRegistry.OrderStatus","name":"status","type":"uint8"},{"internalType":"address","name":"updatedBy","type":"address"},{"internalType":"bool","name":"exists","type":"bool"}],"stateMutability":"view","type":"function"},{"inputs":[],"name":"owner","outputs":[{"internalType":"address","name":"","type":"address"}],"stateMutability":"view","type":"function"},{"inputs":[{"internalType":"string","name":"orderId","type":"string"},{"internalType":"bytes32","name":"dataHash","type":"bytes32"},{"internalType":"enum OrderRegistry.OrderStatus","name":"status","type":"uint8"}],"name":"recordOrder","outputs":[],"stateMutability":"nonpayable","type":"function"},{"inputs":[{"internalType":"address","name":"newOwner","type":"address"}],"name":"transferOwnership","outputs":[],"stateMutability":"nonpayable","type":"function"},{"inputs":[{"internalType":"string","name":"orderId","type":"string"},{"internalType":"bytes32","name":"dataHash","type":"bytes32"},{"internalType":"enum OrderRegistry.OrderStatus","name":"status","type":"uint8"}],"name":"updateOrderStatus","outputs":[],"stateMutability":"nonpayable","type":"function"},{"inputs":[{"internalType":"string","name":"orderId","type":"string"},{"internalType":"bytes32","name":"dataHash","type":"bytes32"}],"name":"verifyOrderHash","outputs":[{"internalType":"bool","name":"","type":"bool"}],"stateMutability":"view","type":"function"}]` 