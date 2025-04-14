# Variables
APP_NAME=strike
BUILD_DIR=build

# Default target
.PHONY: all
all: build

# Only checking for Docker or Podman
CONTAINER_RUNTIME := $(shell \
    command -v docker >/dev/null 2>&1 && echo docker || \
    command -v podman >/dev/null 2>&1 && echo podman || \
    echo "")

# Runtime check
.PHONY: check-runtime
check-runtime:
	@if [ "$(CONTAINER_RUNTIME)" = "" ]; then \
		echo "Error: Neither Docker nor Podman is installed on the system."; \
		exit 1; \
	else \
		echo "Using container runtime: $(CONTAINER_RUNTIME)"; \
	fi


# ===== STRIKE SERVER =====

# Build server container after checking runtime
.PHONY: server-build
server-build: check-runtime
	$(CONTAINER_RUNTIME) build -t strike_server -f deployment/StrikeServer.ContainerFile .

# Run server container after checking runtime
.PHONY: server-run
server-run: check-runtime
	$(CONTAINER_RUNTIME) run --env-file=./config/env.server -v ~/.strike-server/:/home/strike-server/ --name strike_server --network=strikenw -p 8080:8080 localhost/strike_server:latest

# Run server container and attach stdout
.PHONY: server-start
server-start: check-runtime
	$(CONTAINER_RUNTIME) start -a strike_server

# ===== STRIKE SERVER =====


# ===== STRIKE DB =====

.PHONY: db-build
db-build: check-runtime
	$(CONTAINER_RUNTIME) build -t strike_db -f deployment/StrikeDatabase.ContainerFile .

.PHONY: db-run
db-run: check-runtime
	$(CONTAINER_RUNTIME) run --env-file=./config/env.db --name strike_db --network=strikenw -p 5432:5432 localhost/strike_db:latest

.PHONY: db-start
db-start: check-runtime
	$(CONTAINER_RUNTIME) start strike_db 

# ===== STRIKE DB =====

# ===== STRIKE CLIENT =====
.PHONY: client-build
client-build: check-runtime
	$(CONTAINER_RUNTIME) build -t strike_client -f deployment/StrikeClient.ContainerFile .

.PHONY: client-run
client-run: check-runtime
	$(CONTAINER_RUNTIME) run -it --env-file=./config/env.client -v ~/.strike-keys/:/home/strike-client/ -v ~/.strike-server/strike_server.crt:/home/strike-client/strike_server.crt --name strike_client --network=strikenw localhost/strike_client:latest

.PHONY: another-client-run
another-client-run: check-runtime
	$(CONTAINER_RUNTIME) run -it --env-file=./config/env.client -v ~/.strike-keys/:/home/strike-client/ -v ~/.strike-server/strike_server.crt:/home/strike-client/strike_server.crt --name strike_client1 --network=strikenw localhost/strike_client:latest

.PHONY: client--start
client-start: check-runtime
	$(CONTAINER_RUNTIME) start -a strike_client 

# ===== STRIKE CLIENT =====

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



# === strike cluster ===

.PHONY: strike-cluster-start
strike-cluster-start:
	ctlptl create cluster k3d --registry=ctlptl-registry && tilt up

.PHONY: strike-cluster-stop
strike-cluster-stop:
	tilt down && ctlptl delete cluster k3d-k3s-default

# === strike cluster ===

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
