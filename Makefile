# Variables
APP_NAME=strike
SERVER_CMD_DIR=cmd/strike-server
CLIENT_CMD_DIR=cmd/strike-client
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
.PHONY: server-container-build
server-container-build: check-runtime
	$(CONTAINER_RUNTIME) build -t strike_server -f deployment/StrikeServer.ContainerFile .

# Run server container after checking runtime
.PHONY: server-container-run
server-container-run: check-runtime
	$(CONTAINER_RUNTIME) run --env-file=./config/env.server -v ~/.strike-server/:/tmp/strike-server/ --name strike_server --network=strikenw -p 8080:8080 localhost/strike_server:latest

# Run server container and attach stdout
.PHONY: server-container-start
server-container-start: check-runtime
	$(CONTAINER_RUNTIME) start -a strike_server

# ===== STRIKE SERVER =====


# ===== STRIKE DB =====

.PHONY: db-container-build
db-container-build: check-runtime
	$(CONTAINER_RUNTIME) build -t strike_db -f deployment/StrikeDatabase.ContainerFile .

.PHONY: db-container-run
db-container-run: check-runtime
	$(CONTAINER_RUNTIME) run --name strike_db --network=strikenw -p 5432:5432 localhost/strike_db:latest

.PHONY: db-container-start
db-container-start: check-runtime
	$(CONTAINER_RUNTIME) start strike_db 

# ===== STRIKE DB =====

# ===== STRIKE CLIENT =====
.PHONY: client-container-build
client-container-build: check-runtime
	$(CONTAINER_RUNTIME) build -t strike_client -f deployment/StrikeClient.ContainerFile .

.PHONY: client-container-run
client-container-run: check-runtime
	$(CONTAINER_RUNTIME) run -it --name strike_client --network=strikenw localhost/strike_client:latest

.PHONY: client-container-start
client-container-start: check-runtime
	$(CONTAINER_RUNTIME) start -a strike_client 

# Build strike client
.PHONY: client-binary-build
client-binary-build:
	mkdir -p $(BUILD_DIR)
	go build -o $(BUILD_DIR)/$(APP_NAME)-client $(CLIENT_CMD_DIR)/main.go

# Run strike-client
.PHONY: client-binary-run
client-binary-run:
	./$(BUILD_DIR)/$(APP_NAME)-client --config=config/clientConfig.json

# ===== STRIKE CLIENT =====

# Run Tests
.PHONY: test
test:
	go test ./... -v

# Clean build artifacts
.PHONY: clean-binary-artifacts
clean:
	rm -rf $(BUILD_DIR)
	rm -rf cfg/

# Format code
.PHONY: fmt
fmt:
	go fmt ./...

# Lint code
.PHONY: lint
lint:
	golangci-lint run
