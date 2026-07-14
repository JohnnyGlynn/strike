# Variables
APP_NAME=strike
BUILD_DIR=build

# Default target
.PHONY: all
all: build

# === keygen ===

KEYS_DIR=keys
CA_DIR=$(KEYS_DIR)/ca
SERVER_BIN=$(BUILD_DIR)/$(APP_NAME)-server

.PHONY: keygen-ca
keygen-ca: $(SERVER_BIN)
	./$(SERVER_BIN) --gen-ca --keydir=$(CA_DIR)

.PHONY: keygen-client
keygen-client:
	mkdir -p $(BUILD_DIR)
	go build -o $(BUILD_DIR)/$(APP_NAME)-client cmd/strike-client/main.go
	./$(BUILD_DIR)/$(APP_NAME)-client --keygen --keydir=$(or $(KEYDIR),$(HOME)/.strike-keys)
	rm -f $(BUILD_DIR)/$(APP_NAME)-client

# Two client keypairs on the host for local 2-user federation testing
.PHONY: keygen-clients
keygen-clients:
	$(MAKE) keygen-client KEYDIR=$(HOME)/.strike-keys/client1
	$(MAKE) keygen-client KEYDIR=$(HOME)/.strike-keys/client2

.PHONY: keygen-server
keygen-server: $(SERVER_BIN)
	./$(SERVER_BIN) --keygen --keydir=$(or $(KEYDIR),./$(KEYS_DIR)/server) \
		$(if $(SERVER_NAME),--name=$(SERVER_NAME)) \
		$(if $(wildcard $(CA_DIR)/strike_ca.crt),--ca-cert=$(CA_DIR)/strike_ca.crt --ca-key=$(CA_DIR)/strike_ca.pem)

$(SERVER_BIN):
	mkdir -p $(BUILD_DIR)
	go build -o $(SERVER_BIN) ./cmd/strike-server

.PHONY: gen-federation
gen-federation: $(SERVER_BIN)
	./$(SERVER_BIN) --gen-federation \
		--output=./config/server/federation.yaml \
		"endpoint0,strike-server1:9090,./$(KEYS_DIR)/server1" \
		"endpoint1,strike-server2:9090,./$(KEYS_DIR)/server2"

.PHONY: keygen-all
keygen-all: $(SERVER_BIN)
	$(MAKE) keygen-ca
	$(MAKE) keygen-server KEYDIR=./$(KEYS_DIR)/server1 SERVER_NAME=endpoint0
	$(MAKE) keygen-server KEYDIR=./$(KEYS_DIR)/server2 SERVER_NAME=endpoint1
	$(MAKE) gen-federation
	$(MAKE) keygen-clients
	rm -rf $(BUILD_DIR)

# === keygen ===

.PHONY: server-run
server-run:
	mkdir -p $(BUILD_DIR)/server
	go build -o $(BUILD_DIR)/server/$(APP_NAME)-server ./cmd/strike-server/main.go
	cp ./config/server/serverConfig.json ./$(BUILD_DIR)/server/
	./$(BUILD_DIR)/server/$(APP_NAME)-server --config=./$(BUILD_DIR)/server/serverConfig.json


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
	cp ./config/client/clientConfig1.json ./$(BUILD_DIR)/client1/clientConfig.json
	$(MAKE) run-client1

.PHONY: 2bin
2bin:
	cp ./config/client/clientConfig2.json ./$(BUILD_DIR)/client2/clientConfig.json
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

