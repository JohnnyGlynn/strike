# Variables
APP_NAME=strike
BUILD_DIR=build

# Default target
.PHONY: all
all: build

# === keygen === 

.PHONY: keygen-client
keygen-client:
	mkdir -p $(BUILD_DIR)
	go build -o $(BUILD_DIR)/$(APP_NAME)-client cmd/strike-client/main.go
	./$(BUILD_DIR)/$(APP_NAME)-client --keygen 
	rm -rf $(BUILD_DIR)

.PHONY: keygen-server
keygen-server:
	mkdir -p $(BUILD_DIR)
	go build -o $(BUILD_DIR)/$(APP_NAME)-server cmd/strike-server/main.go
	./$(BUILD_DIR)/$(APP_NAME)-server --keygen 
	rm -rf $(BUILD_DIR)

# === keygen ===


.PHONY: bingen
bingen:
	mkdir -p $(BUILD_DIR)/client1
	mkdir -p $(BUILD_DIR)/client2
	go build -o $(BUILD_DIR)/client1/$(APP_NAME)-client cmd/strike-client/client.go
	go build -o $(BUILD_DIR)/client2/$(APP_NAME)-client cmd/strike-client/client.go
	cp ./config/clientConfig.json ./$(BUILD_DIR)/client1/
	$(MAKE) run-client1	
	
.PHONY: 2bin
2bin:
	cp ./config/clientConfig.json ./$(BUILD_DIR)/client2/
	$(MAKE) run-client2

.PHONY: run-client1
run-client1:
	cd $(BUILD_DIR)/client1/ && ./$(APP_NAME)-client --config=./clientConfig.json 

.PHONY: run-client2
run-client2:
	cd $(BUILD_DIR)/client2/ && ./$(APP_NAME)-client --config=./clientConfig.json


# === strike cluster ===

.PHONY: strike-cluster-start
strike-cluster-start:
	ctlptl create cluster k3d --registry=ctlptl-registry && tilt up

.PHONY: strike-cluster-stop
strike-cluster-stop:
	tilt down && ctlptl delete cluster k3d-k3s-default 

# Run Tests
.PHONY: test
test:
	go test ./... -v

# Clean build artifacts
.PHONY: clean-binary-artifacts
clean:
	rm -rf $(BUILD_DIR)

# Format code
.PHONY: fmt
fmt:
	go fmt ./...

# Protobuf generate
.PHONY: proto
proto:
	cd msgdef && protoc --go_out=. --go-grpc_out=. message.proto && cd -

# Lint code
.PHONY: lint
lint:
	golangci-lint run
