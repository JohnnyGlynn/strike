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
.PHONY: build-strike-server-container
build-strike-server-container: check-runtime
	$(CONTAINER_RUNTIME) build -t strike_server -f deployment/StrikeServer.ContainerFile .

# Run server container after checking runtime
.PHONY: run-strike-server-container
run-strike-server-container: build-strike-server-container check-runtime
	$(CONTAINER_RUNTIME) run  --name strike_server --network=strikenw -p 8080:8080 localhost/strike_server:latest

# ===== STRIKE SERVER =====

# ===== STRIKE DB =====

.PHONY: build-strike-db-container
build-strike-db-container: check-runtime
	$(CONTAINER_RUNTIME) build -t strike_db -f deployment/StrikeDatabase.ContainerFile .

.PHONY: run-strike-db-container
run-strike-db-container: build-strike-db-container check-runtime
	$(CONTAINER_RUNTIME) run --name strike_db --network=strikenw -p 5432:5432 localhost/strike_db:latest

# ===== STRIKE DB =====

# ===== STRIKE CLIENT =====

# Build strike client
.PHONY: build-client
build-strike-client:
	mkdir -p $(BUILD_DIR)
	go build -o $(BUILD_DIR)/$(APP_NAME)-client $(CLIENT_CMD_DIR)/main.go

# Run strike-client
.PHONY: run
run-strike-client: build-strike-client
	./$(BUILD_DIR)/$(APP_NAME)-client

# ===== STRIKE CLIENT =====

# Run Tests
.PHONY: test
test:
	go test ./... -v

# Clean build artifacts
.PHONY: clean
clean:
	rm -rf $(BUILD_DIR)

# Format code
.PHONY: fmt
fmt:
	go fmt ./...

# Lint code
.PHONY: lint
lint:
	golangci-lint run
