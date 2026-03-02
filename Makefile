.PHONY: dev dev-otel setup build run demo temporal mock-github migrator reset test vet tidy generate generate-go generate-ts \
        lint lint-go lint-fix \
        console-install console-dev console-build \
        console-lint console-lint-fix console-typecheck console-format console-format-check

SERVER_DIR  = apps/server
CONSOLE_DIR = apps/console

# --- Dev ---

dev:
	@./dev.sh

dev-otel:
	@OTEL_ENABLED=true ./dev.sh

# --- Setup ---

setup:
	@command -v go >/dev/null 2>&1 || { echo "go not found — install from https://go.dev/dl/ and re-run make setup"; exit 1; }
	@echo "✓ go $(shell go version | awk '{print $$3}')"
	@if ! command -v temporal >/dev/null 2>&1; then \
		echo "installing temporal CLI..."; \
		brew install temporal; \
	fi
	@echo "✓ temporal CLI installed"
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
	cd $(SERVER_DIR) && go run .

demo:
	@command -v jq >/dev/null 2>&1 || { echo "jq not found — brew install jq"; exit 1; }
	./scripts/demo.sh

temporal:
	temporal server start-dev --ui-port 8088 --db-filename .temporal.db

mock-github:
	go run ./apps/mock-github

migrator:
	@lsof -ti :8082 | xargs kill -9 2>/dev/null || true
	cd apps/migrators/app-chart-migrator && go run .

reset:
	@echo "Flushing Redis (migrator pending callbacks)..."
	@docker exec loom-redis redis-cli FLUSHDB 2>/dev/null || echo "  (skipped — loom-redis not running)"
	@echo "✓ Redis cleared"
	@echo "Deleting Temporal SQLite database..."
	@rm -f .temporal.db .temporal.db-shm .temporal.db-wal
	@echo "✓ Temporal state cleared — restart 'make temporal' to get a fresh server"
	@echo "Truncating migration and event store..."
	@docker exec loom-postgres psql -U loom -d loom -c "TRUNCATE candidates, migrations, step_events CASCADE;" 2>/dev/null \
		|| echo "  (skipped — loom-postgres not running)"
	@echo "✓ Migration and event store cleared"

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
	@lsof -ti :3000 | xargs kill -9 2>/dev/null || true
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
