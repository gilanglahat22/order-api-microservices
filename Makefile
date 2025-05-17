.PHONY: setup proto build run dev clean test

# Service list
SERVICES := api-gateway order user payment provider blockchain notification

# Default target
all: proto build

# Setup dependencies
setup:
	go mod download
	go install google.golang.org/protobuf/cmd/protoc-gen-go@latest
	go install google.golang.org/grpc/cmd/protoc-gen-go-grpc@latest
	@echo "Setup completed"

# Generate protobuf files
proto:
	@echo "Generating protobuf files..."
	@for dir in proto/*; do \
		if [ -d $$dir ]; then \
			for file in $$dir/*.proto; do \
				if [ -f $$file ]; then \
					protoc --go_out=. --go-grpc_out=. $$file; \
					echo "Generated from $$file"; \
				fi; \
			done; \
		fi; \
	done

# Build all services
build:
	@echo "Building all services..."
	@for service in $(SERVICES); do \
		echo "Building $$service..."; \
		go build -o bin/$$service ./$$service/cmd/server; \
	done

# Run all services in development mode
dev:
	@echo "Starting services in development mode..."
	@for service in $(SERVICES); do \
		echo "Starting $$service..."; \
		go run ./$$service/cmd/server/main.go & \
	done

# Run all services in production mode
run:
	@echo "Starting services..."
	@for service in $(SERVICES); do \
		echo "Starting $$service..."; \
		./bin/$$service & \
	done

# Clean build artifacts
clean:
	@echo "Cleaning up..."
	rm -rf bin/
	rm -rf proto/*/*.pb.go

# Run tests
test:
	go test -v ./...

# Docker compose up
docker-up:
	docker-compose up -d

# Docker compose down
docker-down:
	docker-compose down 