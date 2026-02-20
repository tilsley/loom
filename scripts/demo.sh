#!/usr/bin/env bash
set -euo pipefail

BASE_URL="${BASE_URL:-http://localhost:8080}"
DAPR_URL="${DAPR_URL:-http://localhost:3500}"

blue()  { printf "\033[1;34m%s\033[0m\n" "$*"; }
green() { printf "\033[1;32m%s\033[0m\n" "$*"; }
dim()   { printf "\033[2m%s\033[0m\n" "$*"; }

MIGRATION_ID="migrate-chart"

# --- 1. Worker announces itself via Dapr pub/sub ---

blue "==> Worker announcing migration via pub/sub: $MIGRATION_ID"
dim "    Publishing to migration-registry topic"
echo

curl -s -X POST "$DAPR_URL/v1.0/publish/pubsub/migration-registry" \
  -H 'Content-Type: application/json' \
  -d "$(cat <<'EOF'
{
  "id": "migrate-chart",
  "name": "Migrate Helm Charts",
  "description": "Migrate legacy Helm v2 charts to v3 across all services",
  "targets": ["acme/billing-api", "acme/user-service"],
  "steps": [
    {
      "name": "refactor-api",
      "description": "AI-driven API refactor from v1 to v2",
      "workerApp": "migration-worker",
      "config": {"targetVersion": "v2"}
    },
    {
      "name": "update-tests",
      "description": "Regenerate test suite for new API surface",
      "workerApp": "migration-worker",
      "config": {"minCoverage": "80"}
    }
  ]
}
EOF
)"

echo "Published."
sleep 2

# --- 2. Verify Loom discovered the migration ---

blue "==> Checking Loom discovered the migration"
curl -s "$BASE_URL/migrations/$MIGRATION_ID" | jq .
echo
sleep 1

# --- 3. Run the migration ---

blue "==> Starting run from discovered migration (target: acme/billing-api)"
RUN_RESPONSE=$(curl -s -X POST "$BASE_URL/migrations/$MIGRATION_ID/run" \
  -H 'Content-Type: application/json' \
  -d '{"target": "acme/billing-api"}')
echo "$RUN_RESPONSE" | jq .
RUN_ID=$(echo "$RUN_RESPONSE" | jq -r '.runId')
blue "    Run ID: $RUN_ID"
echo
sleep 2

# --- 4. Check status (should be RUNNING) ---

blue "==> Checking status (expecting RUNNING)"
curl -s "$BASE_URL/status/$RUN_ID" | jq .
echo
sleep 1

# --- 5. Simulate worker callbacks (sequential — matches workflow order) ---

STEPS=("refactor-api" "update-tests")
TARGET="acme/billing-api"
PR_NUM=1

for step in "${STEPS[@]}"; do
    blue "==> Worker callback: $step @ $TARGET"

    curl -s -X POST "$BASE_URL/event/$RUN_ID" \
      -H 'Content-Type: application/json' \
      -d "$(cat <<EOF
{
  "stepName": "$step",
  "target": "$TARGET",
  "success": true,
  "metadata": {
    "prUrl": "https://github.com/$TARGET/pull/$PR_NUM",
    "commitSha": "abc$(printf '%04d' $PR_NUM)"
  }
}
EOF
)" | jq .

    PR_NUM=$((PR_NUM + 1))
    echo
    sleep 2
done

# --- 6. Final status ---

sleep 2
blue "==> Final status"
curl -s "$BASE_URL/status/$RUN_ID" | jq .
echo

# --- 7. Verify registration tracks the run ---

blue "==> Checking registered migration (should show run ID)"
curl -s "$BASE_URL/migrations/$MIGRATION_ID" | jq '.runIds'
echo

green "Done. Migration $MIGRATION_ID discovered via pub/sub, run $RUN_ID complete."
