.PHONY: dev setup build run demo temporal mock-github migrator reset test vet tidy generate generate-go generate-ts \
        lint lint-go lint-fix \
        console-install console-dev console-build \
        console-lint console-lint-fix console-typecheck console-format console-format-check

SERVER_DIR  = apps/server
CONSOLE_DIR = apps/console

# --- Dev ---

dev:
	@./dev.sh

# --- Setup ---

setup:
	@command -v go >/dev/null 2>&1 || { echo "go not found — install from https://go.dev/dl/ and re-run make setup"; exit 1; }
	@echo "✓ go $(shell go version | awk '{print $$3}')"
	@command -v brew >/dev/null 2>&1 || { echo "homebrew not found — install from https://brew.sh and re-run make setup"; exit 1; }
	@if ! command -v dapr >/dev/null 2>&1; then \
		echo "installing dapr CLI..."; \
		brew install dapr/tap/dapr-cli; \
	fi
	@echo "✓ dapr CLI installed"
	@if ! command -v temporal >/dev/null 2>&1; then \
		echo "installing temporal CLI..."; \
		brew install temporal; \
	fi
	@echo "✓ temporal CLI installed"
	@if [ ! -f "$(HOME)/.dapr/bin/daprd" ]; then \
		echo "initializing dapr runtime (requires Docker)..."; \
		dapr init; \
	fi
	@echo "✓ dapr runtime initialized ($(shell dapr version 2>/dev/null | grep 'Runtime' | awk '{print $$3}'))"
	@go mod tidy
	@echo "✓ go dependencies resolved"
	@cd $(CONSOLE_DIR) && bun install --silent
	@echo "✓ console dependencies resolved"
	@$(MAKE) generate --no-print-directory
	@echo "\nReady. Run: make run"

# --- Code Generation (OpenAPI → Go + TypeScript) ---

generate: generate-go generate-ts

generate-go:
	@if ! command -v oapi-codegen >/dev/null 2>&1 && [ ! -f "$(HOME)/go/bin/oapi-codegen" ]; then \
		echo "installing oapi-codegen..."; \
		go install github.com/oapi-codegen/oapi-codegen/v2/cmd/oapi-codegen@latest; \
	fi
	@OAPI=$$(command -v oapi-codegen || echo "$(HOME)/go/bin/oapi-codegen"); \
	$$OAPI --config schemas/oapi-codegen.yaml schemas/openapi.yaml
	@echo "✓ pkg/api/types.gen.go"

generate-ts:
	cd $(CONSOLE_DIR) && bunx openapi-typescript ../../schemas/openapi.yaml -o src/lib/api.gen.ts
	@echo "✓ apps/console/src/lib/api.gen.ts"

# --- Server ---

build: generate-go
	cd $(SERVER_DIR) && go build -o bin/loom-server .

run:
	cd $(SERVER_DIR) && dapr run --app-id loom --app-port 8080 --dapr-http-port 3500 --resources-path ./dapr/components -- go run .

demo:
	@command -v jq >/dev/null 2>&1 || { echo "jq not found — brew install jq"; exit 1; }
	./scripts/demo.sh

temporal:
	temporal server start-dev --ui-port 8088 --db-filename .temporal.db

mock-github:
	go run ./apps/mock-github

migrator:
	cd apps/migrators/app-chart-migrator && dapr run --app-id app-chart-migrator --app-port 3001 --dapr-http-port 3501 --resources-path ./dapr/components -- go run .

reset:
	@echo "Flushing Redis (state store + pub/sub)..."
	@docker exec dapr_redis redis-cli FLUSHDB
	@echo "✓ Redis cleared — migrations, pending callbacks, and pub/sub messages removed"
	@echo "Deleting Temporal SQLite database..."
	@rm -f .temporal.db .temporal.db-shm .temporal.db-wal
	@echo "✓ Temporal state cleared — restart 'make temporal' to get a fresh server"

test:
	go test ./...

vet:
	go vet ./...

tidy:
	go mod tidy

lint-go:
	golangci-lint run ./...

lint-fix:
	golangci-lint run --fix ./...

lint: lint-go console-lint

# --- Console ---

console-install:
	cd $(CONSOLE_DIR) && bun install

console-dev: generate-ts
	cd $(CONSOLE_DIR) && bun run dev

console-build: generate-ts
	cd $(CONSOLE_DIR) && bun run build

console-typecheck:
	cd $(CONSOLE_DIR) && bun run typecheck

console-lint:
	cd $(CONSOLE_DIR) && bun run lint

console-lint-fix:
	cd $(CONSOLE_DIR) && bun run lint:fix

console-format:
	cd $(CONSOLE_DIR) && bun run format

console-format-check:
	cd $(CONSOLE_DIR) && bun run format:check
