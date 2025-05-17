package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"net"
	"os"
	"os/signal"
	"syscall"

	"github.com/order-api-microservices/pkg/blockchain"
	"github.com/order-api-microservices/services/blockchain/internal/service"
	pb "github.com/order-api-microservices/proto/blockchain"
	"github.com/spf13/viper"
	"google.golang.org/grpc"
	"google.golang.org/grpc/reflection"
)

var (
	port         = flag.Int("port", 50053, "The server port")
	configFile   = flag.String("config", "config.yaml", "Configuration file path")
	contractAddr = flag.String("contract", "", "Ethereum contract address")
	ethEndpoint  = flag.String("eth-endpoint", "http://localhost:8545", "Ethereum node endpoint")
	privateKey   = flag.String("key", "", "Private key for Ethereum transactions")
)

func main() {
	flag.Parse()

	// Load configuration
	initConfig()

	// Create Ethereum client
	contractAddress := viper.GetString("ethereum.contract_address")
	if *contractAddr != "" {
		contractAddress = *contractAddr
	}
	
	ethRpcUrl := viper.GetString("ethereum.rpc_url")
	if *ethEndpoint != "" {
		ethRpcUrl = *ethEndpoint
	}
	
	privKey := viper.GetString("ethereum.private_key")
	if *privateKey != "" {
		privKey = *privateKey
	}

	// For development, use a default private key if none is provided
	if privKey == "" {
		privKey = "ac0974bec39a17e36ba4a6b4d238ff944bacb478cbed5efcae784d7bf4f2ff80" // Default Ganache account
		log.Println("Warning: Using default private key for development. DO NOT use in production!")
	}

	ethClient, err := blockchain.NewEthereumClient(ethRpcUrl, contractAddress, privKey)
	if err != nil {
		log.Fatalf("Failed to create Ethereum client: %v", err)
	}

	// Create the service
	blockchainService := service.NewBlockchainService(ethClient)

	// Create gRPC server
	serverPort := viper.GetInt("server.port")
	if *port != 50053 {
		serverPort = *port
	}

	lis, err := net.Listen("tcp", fmt.Sprintf(":%d", serverPort))
	if err != nil {
		log.Fatalf("Failed to listen: %v", err)
	}

	grpcServer := grpc.NewServer()
	pb.RegisterBlockchainServiceServer(grpcServer, blockchainService)
	
	// Register reflection service for development
	reflection.Register(grpcServer)

	// Start server
	log.Printf("Starting blockchain service on port %d...", serverPort)
	go func() {
		if err := grpcServer.Serve(lis); err != nil {
			log.Fatalf("Failed to serve: %v", err)
		}
	}()

	// Handle graceful shutdown
	c := make(chan os.Signal, 1)
	signal.Notify(c, os.Interrupt, syscall.SIGTERM)

	<-c
	log.Println("Shutting down blockchain service...")
	grpcServer.GracefulStop()
}

func initConfig() {
	viper.SetDefault("server.port", 50053)
	viper.SetDefault("ethereum.rpc_url", "http://localhost:8545")
	viper.SetDefault("ethereum.contract_address", "")
	viper.SetDefault("ethereum.private_key", "")

	viper.SetConfigFile(*configFile)
	viper.AutomaticEnv()

	if err := viper.ReadInConfig(); err != nil {
		log.Printf("Warning: config file not found or invalid: %v", err)
		log.Println("Using default configuration and environment variables")
	}
} 