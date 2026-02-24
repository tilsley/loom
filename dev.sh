#!/usr/bin/env bash
set -uo pipefail

# ANSI colours — one per service
C_TEMPORAL='\033[0;34m'  # blue
C_SERVER='\033[0;32m'    # green
C_WORKER='\033[0;35m'    # magenta
C_MOCKGH='\033[0;33m'    # yellow
C_CONSOLE='\033[0;36m'   # cyan
C_RESET='\033[0m'
C_BOLD='\033[1m'
C_DIM='\033[2m'

C_OTEL='\033[0;31m'      # red

PIDS=()
OTEL_COMPOSE=0

# Stream lines from stdin and prefix each one with a coloured label.
prefix() {
  local label="$1" color="$2"
  while IFS= read -r line; do
    printf "${color}${C_BOLD}%-9s${C_RESET} ${C_DIM}│${C_RESET} %s\n" "$label" "$line"
  done
}

cleanup() {
  printf "\n${C_BOLD}Shutting down…${C_RESET}\n"
  for pid in "${PIDS[@]}"; do
    kill "$pid" 2>/dev/null || true
  done
  wait 2>/dev/null || true
  if [ "$OTEL_COMPOSE" -eq 1 ]; then
    printf "${C_BOLD}Stopping LGTM stack…${C_RESET}\n"
    docker compose -f docker-compose.otel.yml down 2>/dev/null || true
  fi
  printf "${C_BOLD}Done.${C_RESET}\n"
  exit 0
}

trap cleanup INT TERM

cd "$(dirname "$0")"

# Regenerate types so the console is never stale.
printf "${C_BOLD}Generating types…${C_RESET}\n"
make generate-ts --no-print-directory

printf "\n${C_BOLD}Loom dev${C_RESET}\n"
printf "  ${C_TEMPORAL}temporal${C_RESET}  → http://localhost:8088\n"
printf "  ${C_SERVER}server${C_RESET}    → http://localhost:8080\n"
printf "  ${C_WORKER}migrator${C_RESET}  → http://localhost:3001\n"
printf "  ${C_MOCKGH}mock-gh${C_RESET}   → http://localhost:8081\n"
printf "  ${C_CONSOLE}console${C_RESET}   → http://localhost:3000\n"

# Optional OTEL: start Grafana LGTM when OTEL_ENABLED=true
if [ "${OTEL_ENABLED:-}" = "true" ]; then
  printf "  ${C_OTEL}grafana${C_RESET}   → http://localhost:3002  (admin/admin)\n"
  printf "\n${C_BOLD}Starting LGTM stack…${C_RESET}\n"
  docker compose -f docker-compose.otel.yml up -d
  OTEL_COMPOSE=1
  export OTEL_ENABLED=true
  export OTEL_EXPORTER_OTLP_ENDPOINT=http://localhost:4317
  export OTEL_EXPORTER_OTLP_INSECURE=true
  export OTEL_SERVICE_NAME=loom-server
fi
printf "\n"

# 1. Temporal — start first and give it a moment to open its port.
temporal server start-dev --ui-port 8088 --db-filename .temporal.db 2>&1 \
  | prefix "temporal" "$C_TEMPORAL" &
PIDS+=($!)
sleep 1

# 2. Server
(cd apps/server && go run .) 2>&1 \
  | prefix "server" "$C_SERVER" &
PIDS+=($!)

# 3. App chart migrator
(cd apps/migrators/app-chart-migrator && go run .) 2>&1 \
  | prefix "migrator" "$C_WORKER" &
PIDS+=($!)

# 4. Mock GitHub
go run ./apps/mock-github 2>&1 \
  | prefix "mock-gh" "$C_MOCKGH" &
PIDS+=($!)

# 5. Console (Next.js dev server)
(cd apps/console && bun run dev) 2>&1 \
  | prefix "console" "$C_CONSOLE" &
PIDS+=($!)

# Block until Ctrl+C.
wait
