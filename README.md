# Order API Microservices

A microservice-based ordering platform with blockchain integration, similar to Uber, Gojek, and Grab.

## Architecture

This system uses a microservice architecture with the following components:

- **API Gateway**: Central entry point for all external requests using Go Gin
- **Order Service**: Core service for handling orders, with blockchain transaction support
- **Provider Service**: Manages service providers and provider matching
- **Blockchain Service**: Handles blockchain ledger integration for order verification
- **Notification Service**: Manages real-time notifications for users and providers
- **Payment Service**: Handles payment processing (stub implementation)
- **User Service**: Manages user accounts (stub implementation)

## Technologies Used

- Go (Golang) for service implementation
- gRPC for inter-service communication
- Protocol Buffers for API definitions
- PostgreSQL for persistent storage
- Ethereum (Ganache for development) for blockchain integration
- Docker & Docker Compose for containerization
- Solidity for smart contracts

## Getting Started

### Prerequisites

- Docker & Docker Compose
- Go 1.18+
- Make

### Setup & Run

1. Clone the repository:
   ```
   git clone https://github.com/your-username/order-api-microservices.git
   cd order-api-microservices
   ```

2. Build and run with Docker Compose:
   ```
   docker-compose up -d
   ```

3. To run services individually for development:
   ```
   # Start PostgreSQL and Ganache
   docker-compose up -d postgres ganache
   
   # Run any service locally
   cd services/order
   go run cmd/server/main.go
   ```

4. Run API Gateway:
   ```
   cd api-gateway
   go run cmd/server/main.go
   ```

5. The API can be accessed at:
   ```
   http://localhost:8080/api/v1
   ```

## Service Endpoints

### Order Service (gRPC: 50051)

- CreateOrder
- GetOrder
- UpdateOrderStatus
- CancelOrder
- ListUserOrders
- ListProviderOrders
- TrackOrder
- AssignProvider
- AcceptOrder
- RejectOrder
- UpdateLocation

### Provider Service (gRPC: 50053)

- FindProviders
- GetProvider
- UpdateLocation
- NotifyProvider
- UpdateAvailability
- UpdateProfile
- ListOrders

### Blockchain Service (gRPC: 50052)

- RecordTransaction
- VerifyTransaction
- GetTransactionDetails

### Notification Service (gRPC: 50054)

- SendNotification
- GetUserNotifications
- MarkNotificationAsRead
- SubscribeToNotifications

### API Gateway (HTTP: 8080)

RESTful endpoints for all services above.

## Development

### Generating Protocol Buffer Code

```
make proto
```

### Running Tests

```
make test
```

### Building Binaries

```
make build
```

## License

MIT