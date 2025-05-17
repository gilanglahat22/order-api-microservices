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
	"time"

	"github.com/order-api-microservices/pkg/database"
	"github.com/order-api-microservices/services/notification/internal/repository"
	"github.com/order-api-microservices/services/notification/internal/service"
	pb "github.com/order-api-microservices/proto/notification"
	"google.golang.org/grpc"
)

func main() {
	// Parse command line flags
	dbHost := flag.String("db-host", getEnv("DB_HOST", "localhost"), "Database host")
	dbPort := flag.Int("db-port", getEnvInt("DB_PORT", 5432), "Database port")
	dbUser := flag.String("db-user", getEnv("DB_USER", "postgres"), "Database user")
	dbPassword := flag.String("db-password", getEnv("DB_PASSWORD", "postgres"), "Database password")
	dbName := flag.String("db-name", getEnv("DB_NAME", "notificationdb"), "Database name")
	dbSSLMode := flag.String("db-sslmode", getEnv("DB_SSLMODE", "disable"), "Database SSL mode")
	
	port := flag.Int("port", getEnvInt("PORT", 50054), "Server port")
	
	flag.Parse()

	// Set up database connection
	dbConfig := database.NewPostgresConfig(
		*dbHost,
		*dbPort,
		*dbUser,
		*dbPassword,
		*dbName,
		*dbSSLMode,
	)
	
	db, err := database.NewPostgresDB(dbConfig)
	if err != nil {
		log.Fatalf("Failed to connect to database: %v", err)
	}
	defer db.Close()

	// Initialize repository
	notificationRepo := repository.NewNotificationRepository(db)

	// Initialize service
	notificationService := service.NewNotificationService(notificationRepo)

	// Set up gRPC server
	lis, err := net.Listen("tcp", fmt.Sprintf(":%d", *port))
	if err != nil {
		log.Fatalf("Failed to listen on port %d: %v", *port, err)
	}

	grpcServer := grpc.NewServer()
	pb.RegisterNotificationServiceServer(grpcServer, notificationService)

	// Handle graceful shutdown
	go func() {
		signals := make(chan os.Signal, 1)
		signal.Notify(signals, syscall.SIGINT, syscall.SIGTERM)
		
		<-signals
		log.Println("Received signal, stopping server...")
		
		// Give connections time to drain
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		
		done := make(chan struct{})
		go func() {
			grpcServer.GracefulStop()
			close(done)
		}()
		
		select {
		case <-ctx.Done():
			log.Println("Timeout during graceful shutdown, forcing exit")
			grpcServer.Stop()
		case <-done:
			log.Println("Server stopped gracefully")
		}
	}()

	// Start server
	log.Printf("Starting notification service on port %d...", *port)
	if err := grpcServer.Serve(lis); err != nil {
		log.Fatalf("Failed to serve: %v", err)
	}
}

// Helper function to get environment variables with defaults
func getEnv(key, defaultValue string) string {
	value := os.Getenv(key)
	if value == "" {
		return defaultValue
	}
	return value
}

// Helper function to get environment variables as integers
func getEnvInt(key string, defaultValue int) int {
	value := os.Getenv(key)
	if value == "" {
		return defaultValue
	}
	
	intValue, err := fmt.Sscanf(value, "%d")
	if err != nil || len(intValue) == 0 {
		return defaultValue
	}
	
	return intValue[0]
} 