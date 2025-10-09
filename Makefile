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

.PHONY: server-run
server-run:
	mkdir -p $(BUILD_DIR)/server
	go build -o $(BUILD_DIR)/server/$(APP_NAME)-server ./cmd/strike-server/main.go
	cp ./config/server/serverConfig.json ./$(BUILD_DIR)/server/
	cd $(BUILD_DIR)/server/
	./$(APP_NAME)-server --config=./serverConfig.json


.PHONY: db-build
db-build:
	docker build -t strike-db -f deploy/db.Dockerfile .

.PHONY: db-run
db-run:
	docker run --env-file=./config/db/env.db --name strike-db --network=strikenw -p 5432:5432 localhost/strike-db:latest

.PHONY: db-start
db-start:
	docker start strike-db


.PHONY: bingen
bingen:
	mkdir -p $(BUILD_DIR)/client1
	mkdir -p $(BUILD_DIR)/client2
	go build -o $(BUILD_DIR)/client1/$(APP_NAME)-client cmd/strike-client/main.go
	go build -o $(BUILD_DIR)/client2/$(APP_NAME)-client cmd/strike-client/main.go
	cp ./config/client/clientConfig.json ./$(BUILD_DIR)/client1/
	$(MAKE) run-client1	
	
.PHONY: 2bin
2bin:
	cp ./config/client/clientConfig.json ./$(BUILD_DIR)/client2/
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
	protoc --proto_path=msgdef --go_out=paths=source_relative:msgdef \
		--go-grpc_out=paths=source_relative:msgdef \
		message/message.proto federation/federation.proto common/common.proto 

# Lint code
.PHONY: lint
lint:
	golangci-lint run --max-issues-per-linter=0 --max-same-issues=0

