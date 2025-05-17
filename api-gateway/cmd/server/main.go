package main

import (
	"flag"
	"fmt"
	"log"
	"os"
	"os/signal"
	"syscall"

	"github.com/gin-contrib/cors"
	"github.com/gin-gonic/gin"
	"github.com/order-api-microservices/api-gateway/internal/gateway"
	orderPb "github.com/order-api-microservices/proto/order"
	"github.com/spf13/viper"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

var (
	port        = flag.Int("port", 8080, "The server port")
	configFile  = flag.String("config", "config.yaml", "Configuration file path")
	orderSvc    = flag.String("order-svc", "", "Order service address")
	userSvc     = flag.String("user-svc", "", "User service address")
	paymentSvc  = flag.String("payment-svc", "", "Payment service address")
	providerSvc = flag.String("provider-svc", "", "Provider service address")
)

func main() {
	flag.Parse()

	// Load configuration
	initConfig()

	// Create gRPC connections
	orderConn, err := createGRPCConnection("services.order")
	if err != nil {
		log.Fatalf("Failed to connect to order service: %v", err)
	}
	defer orderConn.Close()

	// Create gRPC clients
	orderClient := orderPb.NewOrderServiceClient(orderConn)

	// Create API handlers
	orderHandler := gateway.NewOrderHandler(orderClient)

	// Create Gin router
	router := gin.Default()

	// Configure CORS
	router.Use(cors.New(cors.Config{
		AllowOrigins:     []string{"*"},
		AllowMethods:     []string{"GET", "POST", "PUT", "DELETE", "OPTIONS"},
		AllowHeaders:     []string{"Origin", "Content-Type", "Accept", "Authorization"},
		ExposeHeaders:    []string{"Content-Length"},
		AllowCredentials: true,
	}))

	// Register API routes
	orderHandler.RegisterRoutes(router)

	// Add health check endpoint
	router.GET("/health", func(c *gin.Context) {
		c.JSON(200, gin.H{
			"status": "ok",
		})
	})

	// Get server port
	serverPort := viper.GetInt("server.port")
	if *port != 8080 {
		serverPort = *port
	}

	// Start the server
	go func() {
		if err := router.Run(fmt.Sprintf(":%d", serverPort)); err != nil {
			log.Fatalf("Failed to start server: %v", err)
		}
	}()

	log.Printf("API Gateway started on port %d", serverPort)

	// Wait for termination signal
	c := make(chan os.Signal, 1)
	signal.Notify(c, os.Interrupt, syscall.SIGTERM)
	<-c

	log.Println("Shutting down API Gateway...")
}

func initConfig() {
	viper.SetDefault("server.port", 8080)
	viper.SetDefault("services.order", "localhost:50051")
	viper.SetDefault("services.user", "localhost:50052")
	viper.SetDefault("services.payment", "localhost:50054")
	viper.SetDefault("services.provider", "localhost:50055")

	viper.SetConfigFile(*configFile)
	viper.AutomaticEnv()

	if err := viper.ReadInConfig(); err != nil {
		log.Printf("Warning: config file not found or invalid: %v", err)
		log.Println("Using default configuration and environment variables")
	}
}

func createGRPCConnection(configKey string) (*grpc.ClientConn, error) {
	serviceAddr := viper.GetString(configKey)

	// Override from command line if provided
	switch configKey {
	case "services.order":
		if *orderSvc != "" {
			serviceAddr = *orderSvc
		}
	case "services.user":
		if *userSvc != "" {
			serviceAddr = *userSvc
		}
	case "services.payment":
		if *paymentSvc != "" {
			serviceAddr = *paymentSvc
		}
	case "services.provider":
		if *providerSvc != "" {
			serviceAddr = *providerSvc
		}
	}

	if serviceAddr == "" {
		return nil, fmt.Errorf("service address not configured for %s", configKey)
	}

	return grpc.Dial(serviceAddr, grpc.WithTransportCredentials(insecure.NewCredentials()))
} 