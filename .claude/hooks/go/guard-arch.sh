#!/usr/bin/env bash
# PreToolUse hook — enforces server architecture import boundaries.
#
# Rules (from apps/server/ARCHITECTURE.md):
#   service.go / ports.go  → must NOT import store/, migrator/, handler/,
#                             platform/temporal/, gin, go-redis, go.temporal.io
#   execution/             → must NOT import store/, migrator/, handler/
#   handler/               → must NOT import store/, migrator/, platform/temporal/
#
# Reads the content being written (Write tool) or the new_string (Edit tool)
# and scans for forbidden imports before the file is saved.

# Read stdin once into a variable so it can be passed to python3
stdin_data=$(cat)

python3 - "$stdin_data" <<'PYEOF'
import sys, json, re

try:
    d = json.loads(sys.argv[1])
except Exception:
    sys.exit(0)

file_path = d.get("tool_input", {}).get("file_path", "")
if not file_path.endswith(".go"):
    sys.exit(0)

tool_name = d.get("tool_name", "")
if tool_name == "Write":
    content = d.get("tool_input", {}).get("content", "")
elif tool_name == "Edit":
    content = d.get("tool_input", {}).get("new_string", "")
else:
    sys.exit(0)

fp = file_path.replace("\\", "/")
MODULE = "github.com/tilsley/loom/apps/server/internal/migrations"

if "/migrations/service.go" in fp or "/migrations/ports.go" in fp or "/migrations/errors.go" in fp or "/migrations/run.go" in fp:
    layer = "service"
elif "/migrations/execution/" in fp:
    layer = "execution"
elif "/migrations/handler/" in fp:
    layer = "handler"
else:
    sys.exit(0)

import_re = re.compile(r'"([^"]+)"')
imports = import_re.findall(content)

RULES = {
    "service": [
        (f"{MODULE}/store",    "store/ (use MigrationStore port instead)"),
        (f"{MODULE}/migrator", "migrator/ (use MigratorNotifier port instead)"),
        (f"{MODULE}/handler",  "handler/"),
        ("platform/temporal",  "platform/temporal/ (use ExecutionEngine port instead)"),
        ("gin-gonic/gin",      "Gin (service layer must be framework-free)"),
        ("go-redis",           "go-redis (use MigrationStore port instead)"),
        ("go.temporal.io",     "Temporal SDK (use ExecutionEngine port instead)"),
    ],
    "execution": [
        (f"{MODULE}/store",    "store/ (use MigrationStore port instead)"),
        (f"{MODULE}/migrator", "migrator/ (use MigratorNotifier port instead)"),
        (f"{MODULE}/handler",  "handler/"),
    ],
    "handler": [
        (f"{MODULE}/store",    "store/ (use service layer instead)"),
        (f"{MODULE}/migrator", "migrator/ (use service layer instead)"),
        ("platform/temporal",  "platform/temporal/ (use service layer instead)"),
    ],
}

violations = []
for imp in imports:
    for forbidden_substr, label in RULES[layer]:
        if forbidden_substr in imp:
            violations.append(f"  {imp!r}  →  forbidden: {label}")
            break

if violations:
    print(f"ERROR: Architecture violation in {file_path} (layer: {layer})", file=sys.stderr)
    print("Forbidden imports detected:", file=sys.stderr)
    for v in violations:
        print(v, file=sys.stderr)
    print("See apps/server/ARCHITECTURE.md for the import rules.", file=sys.stderr)
    sys.exit(1)

sys.exit(0)
PYEOF
